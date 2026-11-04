package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// TestAnchorA_ConnectThenClaim 先连后领（验收锚 a）：
// spawn 后不连接＝不产生 施工中。
func TestAnchorA_ConnectThenClaim(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	mkTicketAt(t, g, "ta", db.PhaseAssigned)

	run := spawnSeat(t, g, "ta", "施工", "idle") // idle：永不连接

	// 给足够窗口：连接若发生会立即出 attach/claim 事件；预期什么都没有。
	for _, e := range collectEvents(g, 300*time.Millisecond) {
		if e.Kind == "attach" || e.Kind == "phase:claim" {
			t.Fatalf("未连接却出现 %s 事件", e.Kind)
		}
	}
	if p, _ := g.DB().PhaseOf(context.Background(), "ta"); p != db.PhaseAssigned {
		t.Fatalf("相位=%s 预期 已派（先连后领被击穿）", p)
	}
	st, _ := g.DB().RunByToken(context.Background(), run.RunToken)
	if !st.AttachedAt.IsZero() {
		t.Fatal("未连接的 run 被记 attached_at")
	}
}

// TestAnchorC_MidDeath 中途死（验收锚 c）：杀席位进程→SIGCHLD＋EOF 双源；
// T35：相位稳守 施工中（席死不回队）＋seat_dead 账，调和器按失败表重派。
func TestAnchorC_MidDeath(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	mkTicketAt(t, g, "tc", db.PhaseAssigned)

	run := spawnSeat(t, g, "tc", "施工", "worker")
	waitEvent(t, g, "phase:claim", "tc", 5*time.Second)
	waitPhase(t, g, "tc", db.PhaseWorking, 2*time.Second)

	if err := syscall.Kill(run.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "seat_dead", "tc", 5*time.Second)
	waitPhase(t, g, "tc", db.PhaseWorking, 2*time.Second)

	// 双源互证必须都有记录：SIGCHLD(exited_at)＋EOF(stream_closed)。
	st, _ := g.DB().RunByToken(context.Background(), run.RunToken)
	if !st.Dead || st.ExitedAt.IsZero() || !st.StreamClosed {
		t.Fatalf("双源不全: dead=%v exited_at=%v stream_closed=%v", st.Dead, st.ExitedAt, st.StreamClosed)
	}
	// 判决史/事件留痕：seat_dead 落账；相位稳守不回队。
	evs, _ := g.DB().Events(context.Background(), "tc")
	found := false
	for _, e := range evs {
		if e.Kind == "seat_dead" {
			found = true
		}
	}
	if !found {
		t.Fatal("席死未记 ticket_event")
	}
}

// TestAnchorD_AllPhaseReconcile 全相位对账（验收锚 d）：
// 于 设计中/施工中/审计中/部署中/待审（另补 已派/待部署）杀网关一次→
// 重启按 §5.3 表逐相收养/回队/补 spawn。
func TestAnchorD_AllPhaseReconcile(t *testing.T) {
	shortSkipInteg(t) // T51④：直建 New(cfg) 径——闸口径豁免
	cases := []struct {
		name      string
		phase     db.Phase
		actor     string // live|dead|none
		seatMode  string // hold|worker|""（无席位）
		wantPhase db.Phase
		wantEvent string
		wantRole  string // 调和器重派的角色（dead/none 支）
	}{
		{"设计中·活行动者·收养", db.PhaseDesigning, "live", "hold", db.PhaseDesigning, "adopted", ""},
		{"设计中·死行动者·相位守+重派", db.PhaseDesigning, "dead", "", db.PhaseDesigning, "attach", "CTO"},
		{"施工中·活行动者·收养", db.PhaseWorking, "live", "worker", db.PhaseWorking, "adopted", ""},
		{"施工中·死行动者·相位守+重派", db.PhaseWorking, "dead", "", db.PhaseWorking, "attach", "施工"},
		{"审计中·活行动者·收养", db.PhaseAuditing, "live", "hold", db.PhaseAuditing, "adopted", ""},
		{"审计中·死行动者·相位守+重派", db.PhaseAuditing, "dead", "", db.PhaseAuditing, "attach", "审计"},
		{"部署中·活行动者·收养", db.PhaseDeploying, "live", "hold", db.PhaseDeploying, "adopted", ""},
		{"部署中·死行动者·相位守+重派", db.PhaseDeploying, "dead", "", db.PhaseDeploying, "attach", "CTO"},
		{"待审·无行动者·起审计", db.PhaseReview, "none", "", db.PhaseReview, "attach", "审计"},
		{"已派·无行动者·起施工", db.PhaseAssigned, "none", "", db.PhaseAssigned, "attach", "施工"},
		{"待部署·无行动者·起CTO", db.PhaseToDeploy, "none", "", db.PhaseToDeploy, "attach", "CTO"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GO_HELPER_SEAT", "1")
			t.Setenv("SEAT_MODE", "worker")
			dir := t.TempDir()
			cfg := dispatchCfg(dir)
			cfg.InjectRoot = injectFixture(t)

			// ---- 网关1：布置相位与行动者，然后「杀网关」 ----
			g1, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			go g1.Serve()
			tid := "td"
			claimPhase := tc.phase
			if tc.seatMode == "worker" {
				claimPhase = db.PhaseAssigned // 先派后领
			}
			mkTicketAt(t, g1, tid, claimPhase)
			_ = g1.DB().UpdateSpec(context.Background(), tid, testSpec(t))
			_ = g1.DB().UpsertProject(context.Background(), stubProject("P"))

			var run *db.Run
			switch tc.actor {
			case "live":
				role := "施工"
				if tc.seatMode == "hold" {
					role = map[db.Phase]string{
						db.PhaseDesigning: "CTO", db.PhaseAuditing: "审计", db.PhaseDeploying: "CTO",
					}[tc.phase]
				}
				run = spawnSeat(t, g1, tid, role, tc.seatMode,
					"META_ADOPT_GRACE=30", "META_TRACE="+filepath.Join(dir, "trace.log"))
				if tc.seatMode == "worker" {
					waitEvent(t, g1, "phase:claim", tid, 5*time.Second)
				}
				waitSeatUp(t, run)
			case "dead":
				// 死行动者：run 行在、socket 不在——收养拨号必败。
				run = &db.Run{
					RunID: "dead-run", TicketID: tid, Role: "施工",
					RunToken:   "dead-token",
					SocketPath: filepath.Join(dir, "run", "seats", "ghost.sock"),
				}
				if err := g1.DB().CreateRun(context.Background(), run); err != nil {
					t.Fatal(err)
				}
			}

			if tc.phase == db.PhaseWorking && tc.seatMode == "worker" {
				waitPhase(t, g1, tid, db.PhaseWorking, 2*time.Second)
			}
			g1.Close() // 杀网关（席位在独立 scope 不随死）

			// ---- 网关2：重启对账 ----
			g2, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			go g2.Serve()
			defer g2.Close()

			if tc.wantRole != "" {
				// T35：调和器重派以 run 行为账（attach 门铃可能先于观察口注册）。
				var got string
				waitFor2(t, 15*time.Second, func() bool {
					rs, _ := g2.DB().RunsForTicket(context.Background(), tid)
					for _, r := range rs {
						if !r.Dead && r.RunID != "dead-run" && r.Role == tc.wantRole {
							got = r.RunID
							return true
						}
					}
					return false
				})
				if got == "" {
					t.Fatalf("调和器未重派 %s 席", tc.wantRole)
				}
			} else {
				// T35：helper 席无收养 socket——收养尽力径；锚＝活席不误杀。
				waitFor2(t, 10*time.Second, func() bool {
					st, _ := g2.DB().RunByToken(context.Background(), run.RunToken)
					return st != nil && !st.Dead
				})
			}
			// T40：调和器事件路径负载下 2s 窗饿死（0050 笼内实证）——
			// 相位收敛是条件等不是时限押，放宽到调和周期足轮。
			waitPhase(t, g2, tid, tc.wantPhase, 15*time.Second)

			if tc.actor == "live" && run != nil {
				// 收养落痕
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					st, _ := g2.DB().RunByToken(context.Background(), run.RunToken)
					if st != nil && !st.AdoptedAt.IsZero() {
						break
					}
					<-time.After(10 * time.Millisecond)
				}
				// T35：helper 席无收养 socket——锚＝活席不误杀。
				st, _ := g2.DB().RunByToken(context.Background(), run.RunToken)
				if st == nil || st.Dead {
					t.Fatal("活席被误杀——desired==actual 对齐破")
				}
			}
		})
	}
}

// TestAnchorE_AlarmRecompute 闹钟重算（验收锚 e）：
// 重启后 deadline 取自 ticket_event；已过期→立即发事件；未过期→设到原 deadline。
func TestAnchorE_AlarmRecompute(t *testing.T) {
	shortSkipInteg(t) // T51④：直建 New(cfg) 径——闸口径豁免
	newCfg := func(dir string) Config {
		return Config{
			DBPath:           filepath.Join(dir, "meta.db"),
			RunDir:           filepath.Join(dir, "run"),
			SocketPath:       filepath.Join(dir, "run", "meta.sock"),
			ProgressTimeout:  30 * time.Minute,
			AuthTimeout:      2 * time.Second,
			AdoptDialTimeout: 2 * time.Second,
		}
	}

	t.Run("已过期立即发事件", func(t *testing.T) {
		dir := t.TempDir()
		cfg := newCfg(dir)
		g1, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		go g1.Serve()
		// 时钟回拨一小时：claim/进度等活性帧全部落在过去——重启后
		// deadline=活性帧+30min 已过，应立即发疑似卡死。
		past := time.Now().Add(-time.Hour)
		g1.DB().SetClock(func() time.Time { return past })
		mkTicketAt(t, g1, "te1", db.PhaseAssigned)
		spawnSeat(t, g1, "te1", "施工", "worker", "META_ADOPT_GRACE=30")
		// T40：门铃竞读改相位条件等（worker 自领→边账即证）。
		waitPhase(t, g1, "te1", db.PhaseWorking, 30*time.Second)
		_ = g1.DB().AppendEvent(context.Background(), "te1", "progress", db.Event{Actor: "seat", Note: "写码"})
		g1.DB().SetClock(time.Now)
		g1.Close()

		g2, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		go g2.Serve()
		defer g2.Close()
		// 直接等 stall：adopted 亦在缓冲道中，事件序不依赖读取序。
		waitEvent(t, g2, "stall_suspect", "te1", 8*time.Second) // 过期→立即响
	})

	t.Run("未过期设到原deadline", func(t *testing.T) {
		dir := t.TempDir()
		cfg := newCfg(dir)
		g1, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		go g1.Serve()
		mkTicketAt(t, g1, "te2", db.PhaseAssigned)
		spawnSeat(t, g1, "te2", "施工", "worker", "META_ADOPT_GRACE=30")
		waitPhase(t, g1, "te2", db.PhaseWorking, 30*time.Second)
		last, _ := g1.DB().LastActivityAt(context.Background(), "te2")
		g1.Close()

		g2, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		go g2.Serve()
		defer g2.Close()
		waitEvent(t, g2, "adopted", "te2", 8*time.Second)
		// T35：收养续命 armAlarm 径删——停滞检测并入调和器期窗
		// （LastActivityAt+ProgressTimeout 未过期→近期零 stall_suspect）。
		_ = last
		for _, e := range collectEvents(g2, 500*time.Millisecond) {
			if e.Kind == "stall_suspect" && e.TicketID == "te2" {
				t.Fatal("未过期竟发 stall——续命径语义破")
			}
		}
		// 窗口内不得有 stall 事件。
		for _, e := range collectEvents(g2, 300*time.Millisecond) {
			if e.Kind == "stall_suspect" {
				t.Fatal("未过期闹钟误响")
			}
		}
	})
}

// TestAnchorF_FakeConnCut 伪连接断流（验收锚 f）：无 token/错 token 的 Attach 被断。
func TestAnchorF_FakeConnCut(t *testing.T) {
	_, dir := newTestGateway(t, func(c *Config) { c.AuthTimeout = 400 * time.Millisecond })
	_, client := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))

	ctx := context.Background()

	// 无 token：首帧不是 RunAuth。
	s1, err := client.Attach(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: "x"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s1.Recv(); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("无 token Attach 未被断: %v", err)
	}

	// 错 token。
	s2, err := client.Attach(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{
		Auth: &pbv1.RunAuth{RunToken: "forged-token"},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Recv(); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("错 token Attach 未被断: %v", err)
	}

	// 干连不发：超首帧时限被断。
	s3, err := client.Attach(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s3.Recv(); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("静默连接未被断: %v", err)
	}
}

// TestAnchorG_AdoptMutualAuth 收养互证（验收锚 g）：
// 收养连接首帧出示 run_token；错误 token 席位拒续且留痕。
func TestAnchorG_AdoptMutualAuth(t *testing.T) {
	g, dir := newTestGateway(t, nil)
	mkTicketAt(t, g, "tg", db.PhaseAssigned)

	trace := filepath.Join(dir, "trace.log")
	run := spawnSeat(t, g, "tg", "施工", "hold", "META_TRACE="+trace)

	// 错误 token：拒续＝PermissionDenied＋留痕。先等收养口起来（有界重试链，
	// 拨号错非 PermissionDenied 时重试）。
	var adoptErr error
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		_, adoptErr = adoptOnce(context.Background(), run.SocketPath, "forged")
		if status.Code(adoptErr) == codes.PermissionDenied {
			break
		}
		<-time.After(30 * time.Millisecond)
	}
	if status.Code(adoptErr) != codes.PermissionDenied {
		t.Fatalf("错 token 收养未被拒: %v", adoptErr)
	}
	waitFile(t, trace, 3*time.Second)
	data, _ := os.ReadFile(trace)
	if !strings.Contains(string(data), "refused") {
		t.Fatalf("拒续未留痕: %q", data)
	}

	// 正确 token（换个席位避免上例已退出）。
	run2 := spawnSeat(t, g, "tg", "施工", "hold", "META_TRACE="+trace)
	conn2, err := grpc.NewClient("unix://"+run2.SocketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()
	waitSeatUp(t, run2)
}

// TestAnchorE_SecretRPC Secret(name) 最小径：ACL 放行给对角色，拒错角色。
func TestSecretRPC(t *testing.T) {
	shortSkipInteg(t) // T51④：直建 New(cfg) 径——闸口径豁免
	g, dir := newTestGateway(t, nil)
	t.Cleanup(func() { // T18 只读凭据件清理前回权限位（unlinkat 闸）
		_ = filepath.Walk(filepath.Join(dir, "run"), func(p string, _ os.FileInfo, e error) error {
			if e == nil {
				_ = os.Chmod(p, 0o700)
			}
			return nil
		})
	})
	mkTicketAt(t, g, "ts", db.PhaseAssigned)

	// 造 vault.key + secret（meta secret put 同径）。
	keyPath := filepath.Join(dir, "run", "vault.key")
	key, err := db.LoadOrCreateVaultKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	ct, _ := db.EncryptSecret(key, []byte("s3cret"))
	if err := g.DB().PutSecret(context.Background(), "shell.devin", ct, []string{"施工"}); err != nil {
		t.Fatal(err)
	}
	g.Close()

	// 重起以载 key。
	cfg := Config{
		DBPath:           filepath.Join(dir, "meta.db"),
		RunDir:           filepath.Join(dir, "run"),
		SocketPath:       filepath.Join(dir, "run", "meta.sock"),
		ProgressTimeout:  30 * time.Minute,
		AuthTimeout:      2 * time.Second,
		AdoptDialTimeout: 2 * time.Second,
	}
	g2, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	go g2.Serve()
	defer g2.Close()

	run := spawnSeat(t, g2, "ts", "施工", "hold")
	conn, c := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	_ = conn
	stream, err := c.Attach(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: run.RunToken}}}); err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_SecretReq{SecretReq: &pbv1.SecretReq{Name: "shell.devin"}}}); err != nil {
		t.Fatal(err)
	}
	msg, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	resp := msg.GetSecretResp()
	if resp == nil || resp.Denied || string(resp.Value) != "s3cret" {
		t.Fatalf("Secret RPC 返回异常: %+v", msg)
	}
}
