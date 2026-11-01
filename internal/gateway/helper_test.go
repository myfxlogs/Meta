package gateway

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
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

// ---------- 测试用席位（helper process） ----------
// 模式经 SEAT_MODE 注入：idle（连都不连）/ hold（只监听收养口）/ worker
// （Attach+RunAuth+Claim，流断进收养等待）。席位契约=spawn env：
// META_GATEWAY_SOCK / META_SEAT_SOCK / META_RUN_TOKEN / META_TICKET_ID /
// META_TRACE / META_ADOPT_GRACE。

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_HELPER_SEAT") != "1" {
		return
	}
	runHelperSeat()
	os.Exit(0)
}

func runHelperSeat() {
	switch os.Getenv("SEAT_MODE") {
	case "idle":
		select {} // spawn 后不连接——验收锚 a 的反面角色
	case "hold":
		serveAdopt(nil)
		select {} // 只监听收养口，守到被杀
	case "worker":
		workerSeat()
	case "rule":
		ruleSeat() // 裁决席：Attach+Claim→发 Rule 帧→守
	case "verdict":
		verdictSeat() // 审计席：Attach+Claim→Deliver{verdict,ref}→退
	case "exitcode":
		// 强制退码席：SEAT_EXIT_CODE 钦定值秒退——席位级/壳级死因分级测试用。
		code := 1
		if v, err := strconv.Atoi(os.Getenv("SEAT_EXIT_CODE")); err == nil {
			code = v
		}
		os.Exit(code)
	case "ceo":
		ceoSeat() // CEO run：Attach+RunAuth 不 Claim（无票），守流散
	default:
		fmt.Fprintln(os.Stderr, "helper: 未知 SEAT_MODE")
		os.Exit(1)
	}
}

// adoptSvc 席位侧收养面：验 run_token——它发的它知道，验不过拒续+留痕+退出。
type adoptSvc struct {
	pbv1.UnimplementedSeatAdoptServer
	token    string
	trace    string
	ticketID string
	accepted chan struct{}
}

func (s *adoptSvc) Adopt(_ context.Context, a *pbv1.AdoptAuth) (*pbv1.AdoptAck, error) {
	if a.RunToken != s.token {
		appendTrace(s.trace, "adopt refused token="+a.RunToken)
		// 席位验不过＝拒续并退出（§5.1）。先让响应回去再退。
		time.AfterFunc(50*time.Millisecond, func() { os.Exit(2) })
		return nil, status.Error(codes.PermissionDenied, "run_token 不符")
	}
	select {
	case s.accepted <- struct{}{}:
	default:
	}
	return &pbv1.AdoptAck{Accepted: true, TicketId: s.ticketID}, nil
}

func appendTrace(path, line string) {
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}

// serveAdopt 监听 per-seat UDS（0600）供网关重启收养；返回已收养的信号道。
func serveAdopt(accepted chan struct{}) *grpc.Server {
	sock := os.Getenv("META_SEAT_SOCK")
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		fmt.Fprintln(os.Stderr, "helper listen:", err)
		os.Exit(1)
	}
	_ = os.Chmod(sock, 0o600)
	srv := grpc.NewServer()
	pbv1.RegisterSeatAdoptServer(srv, &adoptSvc{
		token:    os.Getenv("META_RUN_TOKEN"),
		trace:    os.Getenv("META_TRACE"),
		ticketID: os.Getenv("META_TICKET_ID"),
		accepted: accepted,
	})
	go srv.Serve(ln)
	return srv
}

// workerSeat 完整生命周期：Attach+RunAuth+Claim 守流 → 流断（网关死）→ 装
// 一次性收养闹钟（§5.1：缺省10min，测试经 env 缩短）→ 闹钟内被收养＝续跑
// （有界重试链重连重领），闹钟到＝自行退出。
func workerSeat() {
	accepted := make(chan struct{}, 1)
	serveAdopt(accepted)
	if !attachAndHold(nil) { // 初次必须连上（spawn 同步语义）
		os.Exit(1)
	}

	grace := 600 * time.Second
	if s := os.Getenv("META_ADOPT_GRACE"); s != "" {
		if sec, err := strconv.Atoi(s); err == nil {
			grace = time.Duration(sec) * time.Second
		}
	}
	quit := make(chan struct{})
	time.AfterFunc(grace, func() { close(quit) }) // 一次性收养闹钟

	select {
	case <-accepted: // 被收养：续跑——重连重领
	case <-quit:
		os.Exit(3) // 闹钟到未被收养：自行退出
	}
	// 续跑：有界重试链（每次尝试 2s 立住判据），闹钟到即退。
	for {
		if attachAndHold(quit) {
			select {} // 立住了：守到被杀
		}
		select {
		case <-time.After(50 * time.Millisecond):
		case <-quit:
			os.Exit(3)
		}
	}
}

// attachAndHold 连主 socket → 首帧 RunAuth → Claim → 守流。
// quit==nil：守到流断（返回 true=曾立住）；quit!=nil：连接/流 2s 内立住即返
// true 让调用者长守，快速失败返 false 供重试，quit 触发即退。
func attachAndHold(quit <-chan struct{}) bool {
	conn, err := grpc.NewClient("unix://"+os.Getenv("META_GATEWAY_SOCK"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return false
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if quit != nil {
		done := make(chan struct{})
		defer close(done)
		go func() {
			select {
			case <-quit:
				cancel()
			case <-done:
			}
		}()
	}

	stream, err := pbv1.NewSeatGatewayClient(conn).Attach(ctx)
	if err != nil {
		return false
	}
	if err := stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{
		Auth: &pbv1.RunAuth{RunToken: os.Getenv("META_RUN_TOKEN")},
	}}); err != nil {
		return false
	}
	if err := stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{
		Claim: &pbv1.Claim{TicketId: os.Getenv("META_TICKET_ID")},
	}}); err != nil {
		return false
	}
	res := make(chan error, 1)
	go func() { _, e := stream.Recv(); res <- e }()
	if quit == nil {
		<-res // 长守到 EOF
		return true
	}
	select {
	case <-res:
		return false // 秒死（验签拒/连接断）：让外层重试
	case <-time.After(2 * time.Second):
		return true // 2s 无错＝立住
	case <-quit:
		os.Exit(3)
		return false
	}
}

// ruleSeat 裁决席（adjunct）生命周期：Attach+Claim（adjunct_claim 不转相位）
// →发 SEAT_RULE="action:note" 帧 SEAT_RULE_COUNT 次（缺省 1）→守流。
func ruleSeat() {
	serveAdopt(make(chan struct{}, 1))
	conn, err := grpc.NewClient("unix://"+os.Getenv("META_GATEWAY_SOCK"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		os.Exit(1)
	}
	defer conn.Close()
	stream, err := pbv1.NewSeatGatewayClient(conn).Attach(context.Background())
	if err != nil {
		os.Exit(1)
	}
	send := func(m *pbv1.SeatMsg) {
		if err := stream.Send(m); err != nil {
			os.Exit(1)
		}
	}
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: os.Getenv("META_RUN_TOKEN")}}})
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: os.Getenv("META_TICKET_ID")}}})
	n := 1
	if c, err := strconv.Atoi(os.Getenv("SEAT_RULE_COUNT")); err == nil && c > 0 {
		n = c
	}
	action, note, _ := strings.Cut(os.Getenv("SEAT_RULE"), ":")
	for i := 0; i < n; i++ {
		send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Rule{Rule: &pbv1.Rule{Action: action, Note: note}}})
		<-time.After(50 * time.Millisecond) // 帧间间隔（有界，非周期）
	}
	select {} // 守到被杀
}

// verdictSeat 审计席生命周期：Attach+Claim（audit_start→审计中）→
// Deliver{SEAT_VERDICT, SEAT_REF}→等 Ack/EOF→退。
func verdictSeat() {
	serveAdopt(make(chan struct{}, 1))
	conn, err := grpc.NewClient("unix://"+os.Getenv("META_GATEWAY_SOCK"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		os.Exit(1)
	}
	defer conn.Close()
	stream, err := pbv1.NewSeatGatewayClient(conn).Attach(context.Background())
	if err != nil {
		os.Exit(1)
	}
	send := func(m *pbv1.SeatMsg) {
		if err := stream.Send(m); err != nil {
			os.Exit(1)
		}
	}
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: os.Getenv("META_RUN_TOKEN")}}})
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: os.Getenv("META_TICKET_ID")}}})
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Deliver{Deliver: &pbv1.Deliver{
		Verdict: os.Getenv("SEAT_VERDICT"), Ref: os.Getenv("SEAT_REF")}}})
	_, _ = stream.Recv() // 等 Ack 或断流
}

// ceoSeat CEO run 席位：Attach+RunAuth 守流——META_TICKET_ID 空串免 Claim
// （真实 meta seat 同径，票0011 无头化身最小形态）。
func ceoSeat() {
	serveAdopt(nil)
	conn, err := grpc.NewClient("unix://"+os.Getenv("META_GATEWAY_SOCK"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		os.Exit(1)
	}
	defer conn.Close()
	stream, err := pbv1.NewSeatGatewayClient(conn).Attach(context.Background())
	if err != nil {
		os.Exit(1)
	}
	if err := stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{
		Auth: &pbv1.RunAuth{RunToken: os.Getenv("META_RUN_TOKEN")},
	}}); err != nil {
		os.Exit(1)
	}
	for {
		if _, err := stream.Recv(); err != nil {
			os.Exit(0)
		}
	}
}

// ---------- 测试工具 ----------

// testShellRows 壳登记种子 fixture（配置化后码内 defaultRegistry 已废——
// 测试显式喂配置行，与 config/gateway.textproto 同形）。
func testShellRows() []ShellSeed {
	return []ShellSeed{
		{Name: "devin", Model: "swe-2-high", Role: "施工", Priority: 10},
		{Name: "claude", Role: "施工", Priority: 20},
		{Name: "opencode", Role: "施工", Priority: 30, AttemptCap: 8_000_000},
		{Name: "claude", Role: "审计", Priority: 10},
		{Name: "devin", Model: "swe-2-max", Role: "审计", Priority: 20},
		{Name: "devin", Model: "swe-2-max", Role: "CTO", Priority: 10},
		{Name: "claude", Role: "CTO", Priority: 20},
	}
}

func newTestGateway(t *testing.T, tweak func(*Config)) (*Gateway, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		DBPath:           filepath.Join(dir, "meta.db"),
		RunDir:           filepath.Join(dir, "run"),
		SocketPath:       filepath.Join(dir, "run", "meta.sock"),
		ProgressTimeout:  30 * time.Minute,
		AuthTimeout:      2 * time.Second,
		AdoptDialTimeout: 2 * time.Second,
		ShellRows:        testShellRows(),
	}
	if tweak != nil {
		tweak(&cfg)
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	go g.Serve()
	t.Cleanup(g.Close)
	return g, dir
}

// spawnSeat 经网关 spawn 一个 helper 席位；测试结束自动补杀。
func spawnSeat(t *testing.T, g *Gateway, ticketID, role, mode string, extraEnv ...string) *db.Run {
	t.Helper()
	env := append(os.Environ(),
		"GO_HELPER_SEAT=1",
		"SEAT_MODE="+mode,
		"PATH="+os.Getenv("PATH"),
	)
	env = append(env, extraEnv...)
	run, err := g.Spawn(context.Background(), ticketID, role, "testshell",
		[]string{os.Args[0], "-test.run=TestHelperProcess", "-test.v=false"}, env)
	if err != nil {
		t.Fatalf("spawn %s: %v", mode, err)
	}
	pid := run.PID
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	return run
}

// mkTicketAt 建票并推到目标相位。
func mkTicketAt(t *testing.T, g *Gateway, id string, target db.Phase) {
	t.Helper()
	d := g.DB()
	ctx := context.Background()
	if err := d.CreateTicket(ctx, id, id+" 票", nil); err != nil {
		t.Fatal(err)
	}
	chain := []struct {
		to   db.Phase
		kind db.EdgeKind
	}{
		{db.PhaseDesigning, db.EdgeDesignStart},
		{db.PhaseAssigned, db.EdgeDispatch},
		{db.PhaseWorking, db.EdgeClaim},
		{db.PhaseReview, db.EdgeDeliver},
		{db.PhaseAuditing, db.EdgeAuditStart},
		{db.PhaseToDeploy, db.EdgeAuditPass},
		{db.PhaseDeploying, db.EdgeDeployStart},
		{db.PhaseDone, db.EdgeDeployDone},
	}
	for _, s := range chain {
		p, _ := d.PhaseOf(ctx, id)
		if p == target {
			return
		}
		if _, err := d.Transition(ctx, id, s.to, s.kind, "test", ""); err != nil {
			t.Fatalf("mkTicketAt→%s: %v", s.to, err)
		}
	}
}

// waitEvent 在观察口等一条匹配事件（有界等待，非轮询）。
func waitEvent(t *testing.T, g *Gateway, kind, ticketID string, timeout time.Duration) Event {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		left := time.Until(deadline)
		if left <= 0 {
			t.Fatalf("等事件 %s[%s] 超时 %v", kind, ticketID, timeout)
		}
		select {
		case e := <-g.EventCh():
			if e.Kind == kind && (ticketID == "" || e.TicketID == ticketID) {
				return e
			}
		case <-time.After(left):
			t.Fatalf("等事件 %s[%s] 超时 %v", kind, ticketID, timeout)
		}
	}
}

// collectEvents 在窗口内收集全部事件（负向断言用）。
func collectEvents(g *Gateway, dur time.Duration) []Event {
	var out []Event
	end := time.After(dur)
	for {
		select {
		case e := <-g.EventCh():
			out = append(out, e)
		case <-end:
			return out
		}
	}
}

// waitPhase 有界重试链（≤deadline）等相位到达——机械判，非周期巡检。
func waitPhase(t *testing.T, g *Gateway, id string, want db.Phase, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		p, err := g.DB().PhaseOf(context.Background(), id)
		if err == nil && p == want {
			return
		}
		<-time.After(10 * time.Millisecond)
	}
	p, _ := g.DB().PhaseOf(context.Background(), id)
	t.Fatalf("票 %s 相位=%s 预期 %s（%v 未达）", id, p, want, timeout)
}

// adoptOnce 对席位收养口发一次 Adopt（单次调用，供有界重试链用）。
func adoptOnce(ctx context.Context, sockPath, token string) (*pbv1.AdoptAck, error) {
	conn, err := grpc.NewClient("unix://"+sockPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return pbv1.NewSeatAdoptClient(conn).Adopt(cctx, &pbv1.AdoptAuth{RunToken: token})
}

// waitSeatUp 等 helper 席位的收养口可用——有界重试链（R2 边界条款）：
// socket 文件出现前 unix 拨号是 fast-fail，所以逐次尝试到 deadline。
func waitSeatUp(t *testing.T, run *db.Run) *pbv1.AdoptAck {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		ack, err := adoptOnce(context.Background(), run.SocketPath, run.RunToken)
		if err == nil {
			return ack
		}
		<-time.After(30 * time.Millisecond)
	}
	log, _ := os.ReadFile(run.LogPath)
	t.Fatalf("席位收养口未就绪\npid=%d alive=%v\nseat log:\n%s", run.PID, processAlive(run.PID), log)
	return nil
}

// waitFile 有界重试链等文件出现。
func waitFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		<-time.After(10 * time.Millisecond)
	}
	t.Fatalf("等文件 %s 超时", path)
}

func dialSeatGateway(t *testing.T, sockPath string) (*grpc.ClientConn, pbv1.SeatGatewayClient) {
	t.Helper()
	conn, err := grpc.NewClient("unix://"+sockPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn, pbv1.NewSeatGatewayClient(conn)
}
