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

// TestAnchorC_MidDeath 中途死（验收锚 c）：杀席位进程→SIGCHLD＋EOF 双源→施工中→已派。
func TestAnchorC_MidDeath(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	mkTicketAt(t, g, "tc", db.PhaseAssigned)

	run := spawnSeat(t, g, "tc", "施工", "worker")
	waitEvent(t, g, "phase:claim", "tc", 5*time.Second)
	waitPhase(t, g, "tc", db.PhaseWorking, 2*time.Second)

	if err := syscall.Kill(run.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "requeue", "tc", 5*time.Second)
	waitPhase(t, g, "tc", db.PhaseAssigned, 2*time.Second)

	// 双源互证必须都有记录：SIGCHLD(exited_at)＋EOF(stream_closed)。
	st, _ := g.DB().RunByToken(context.Background(), run.RunToken)
	if !st.Dead || st.ExitedAt.IsZero() || !st.StreamClosed {
		t.Fatalf("双源不全: dead=%v exited_at=%v stream_closed=%v", st.Dead, st.ExitedAt, st.StreamClosed)
	}
	// 判决史/事件留痕
	evs, _ := g.DB().Events(context.Background(), "tc")
	found := false
	for _, e := range evs {
		if e.Kind == string(db.EdgeRequeue) {
			found = true
		}
	}
	if !found {
		t.Fatal("回队未记 ticket_event")
	}
}

// TestAnchorD_AllPhaseReconcile 全相位对账（验收锚 d）：
// 于 设计中/施工中/审计中/部署中/待审（另补 已派/待部署）杀网关一次→
// 重启按 §5.3 表逐相收养/回队/补 spawn。
func TestAnchorD_AllPhaseReconcile(t *testing.T) {
	cases := []struct {
		name       string
		phase      db.Phase
		actor      string // live|dead|none
		seatMode   string // hold|worker|""（无席位）
		wantPhase  db.Phase
		wantEvent  string
		wantRole   string // spawn_needed 时的角色
	}{
		{"设计中·活行动者·收养", db.PhaseDesigning, "live", "hold", db.PhaseDesigning, "adopted", ""},
		{"设计中·死行动者·回已立", db.PhaseDesigning, "dead", "", db.PhaseCreated, "requeue", ""},
		{"施工中·活行动者·收养", db.PhaseWorking, "live", "worker", db.PhaseWorking, "adopted", ""},
		{"施工中·死行动者·回已派", db.PhaseWorking, "dead", "", db.PhaseAssigned, "requeue", ""},
		{"审计中·活行动者·收养", db.PhaseAuditing, "live", "hold", db.PhaseAuditing, "adopted", ""},
		{"审计中·死行动者·回待审", db.PhaseAuditing, "dead", "", db.PhaseReview, "requeue", ""},
		{"部署中·活行动者·收养", db.PhaseDeploying, "live", "hold", db.PhaseDeploying, "adopted", ""},
		{"部署中·死行动者·回待部署", db.PhaseDeploying, "dead", "", db.PhaseToDeploy, "requeue", ""},
		{"待审·无行动者·补spawn", db.PhaseReview, "none", "", db.PhaseReview, "spawn_needed", "审计"},
		{"已派·无行动者·补spawn", db.PhaseAssigned, "none", "", db.PhaseAssigned, "spawn_needed", "施工"},
		{"待部署·无行动者·补spawn", db.PhaseToDeploy, "none", "", db.PhaseToDeploy, "spawn_needed", "CTO"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := Config{
				DBPath:           filepath.Join(dir, "meta.db"),
				RunDir:           filepath.Join(dir, "run"),
				SocketPath:       filepath.Join(dir, "run", "meta.sock"),
				ProgressTimeout:  30 * time.Minute,
				AuthTimeout:      2 * time.Second,
				AdoptDialTimeout: 2 * time.Second,
			}

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

			ev := waitEvent(t, g2, tc.wantEvent, tid, 8*time.Second)
			if tc.wantRole != "" && ev.Role != tc.wantRole {
				t.Fatalf("spawn_needed 角色=%s 预期 %s", ev.Role, tc.wantRole)
			}
			waitPhase(t, g2, tid, tc.wantPhase, 2*time.Second)

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
				st, _ := g2.DB().RunByToken(context.Background(), run.RunToken)
				if st == nil || st.AdoptedAt.IsZero() {
					t.Fatal("收养未记 adopted_at")
				}
			}
		})
	}
}

// TestAnchorE_AlarmRecompute 闹钟重算（验收锚 e）：
// 重启后 deadline 取自 ticket_event；已过期→立即发事件；未过期→设到原 deadline。
func TestAnchorE_AlarmRecompute(t *testing.T) {
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
		waitEvent(t, g1, "phase:claim", "te1", 5*time.Second)
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
		waitEvent(t, g1, "phase:claim", "te2", 5*time.Second)
		last, _ := g1.DB().LastActivityAt(context.Background(), "te2")
		g1.Close()

		g2, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		go g2.Serve()
		defer g2.Close()
		waitEvent(t, g2, "adopted", "te2", 8*time.Second)

		deadline, ok := g2.AlarmDeadline("te2")
		if !ok {
			t.Fatal("收养后闹钟未重挂")
		}
		want := last.Add(30 * time.Minute)
		if deadline.Sub(want).Abs() > 2*time.Second {
			t.Fatalf("deadline=%v 预期≈%v（应取自 ticket_event）", deadline, want)
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
	g, dir := newTestGateway(t, nil)
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
