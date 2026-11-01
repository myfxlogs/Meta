package seat

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"

	"meta/internal/adapter"
	pbv1 "meta/internal/pb/metav1"
)

// ---------- D-14：specFromEnv 基础件 ----------

func baseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("META_SHELL", "devin")
	t.Setenv("META_MODEL", "swe-2")
	t.Setenv("META_ROLE", "施工")
	t.Setenv("META_GATEWAY_SOCK", "/tmp/g.sock")
	t.Setenv("META_SEAT_SOCK", "/tmp/s.sock")
	t.Setenv("META_RUN_TOKEN", "tok")
	t.Setenv("META_TICKET_ID", "t1")
	t.Setenv("META_RUN_ID", "r1")
	t.Setenv("META_ADOPT_GRACE", "30")
}

func TestSpecFromEnv(t *testing.T) {
	baseEnv(t)
	s, err := specFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if s.Shell != "devin" || s.TicketID != "t1" || s.AdoptGrace != 30*time.Second {
		t.Fatalf("spec 不符: %+v", s)
	}
	if s.Interactive {
		t.Fatal("Interactive 默认应 false")
	}
	t.Setenv("META_INTERACTIVE", "1")
	s, _ = specFromEnv()
	if !s.Interactive {
		t.Fatal("META_INTERACTIVE=1 未生效")
	}
}

func TestSpecFromEnv_Missing(t *testing.T) {
	baseEnv(t)
	t.Setenv("META_TICKET_ID", "")
	if _, err := specFromEnv(); err == nil {
		t.Fatal("空 ticket 非 CEO 应拒")
	}
	t.Setenv("META_ROLE", "CEO")
	if _, err := specFromEnv(); err != nil {
		t.Fatalf("CEO 空 ticket 应放行（免 claim）: %v", err)
	}
	t.Setenv("META_SHELL", "")
	if _, err := specFromEnv(); err == nil {
		t.Fatal("空 shell 应拒")
	}
}

// ---------- D-14：哨兵扫描基础件 ----------

func newTestSession(ticketID string) *session {
	return &session{
		spec:    spec{Spec: adapter.Spec{Shell: "fake", Role: "施工"}, TicketID: ticketID},
		sendq:   make(chan *pbv1.SeatMsg, 16),
		adopted: make(chan struct{}, 1),
		ackCh:   make(chan struct{}, 1),
	}
}

func TestScanSentinel(t *testing.T) {
	s := newTestSession("t1")
	scan := func(l string) bool { return s.scanSentinel([]byte(l)) }
	if scan("normal output line") {
		t.Fatal("普通行被吞")
	}
	if !scan("META_DELIVER:ref9") {
		t.Fatal("META_DELIVER 未吞")
	}
	if !s.deliverSeen || s.deliverRef != "ref9" {
		t.Fatalf("deliver 哨兵未落: seen=%v ref=%q", s.deliverSeen, s.deliverRef)
	}
	if !scan("META_VERDICT:pass:vr1") {
		t.Fatal("META_VERDICT 未吞")
	}
	if s.verdict != "pass" || s.verdictRef != "vr1" {
		t.Fatalf("verdict 哨兵未落: %q %q", s.verdict, s.verdictRef)
	}
	if !scan("META_RULE:defer:在推进") {
		t.Fatal("META_RULE 未吞")
	}
	if m := <-s.sendq; m.GetRule() == nil || m.GetRule().Action != "defer" {
		t.Fatalf("Rule 帧未进队: %+v", m)
	}
}

func TestScanSentinel_Admin(t *testing.T) {
	s := newTestSession("t1")
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.md")
	_ = os.WriteFile(specFile, []byte("# spec"), 0o600)
	scan := func(l string) bool { return s.scanSentinel([]byte(l)) }

	if !scan("META_ADMIN:fill_spec|" + specFile) {
		t.Fatal("fill_spec 哨兵未吞")
	}
	m := <-s.sendq
	fs := m.GetAdmin().GetFillSpec()
	if fs == nil || fs.TicketId != "t1" || len(fs.SpecMd) == 0 {
		t.Fatalf("fill_spec 帧不符: %+v", m)
	}
	if !scan("META_ADMIN:create_ticket|c1|子票|" + specFile) {
		t.Fatal("create_ticket 哨兵未吞")
	}
	m = <-s.sendq
	if m.GetAdmin().GetCreateTicket().GetId() != "c1" {
		t.Fatalf("create_ticket 帧不符: %+v", m)
	}
	if !scan("META_ADMIN:dispatch|c1|施工") {
		t.Fatal("dispatch 哨兵未吞")
	}
	m = <-s.sendq
	if m.GetAdmin().GetDispatch().GetRole() != "施工" {
		t.Fatalf("dispatch 帧不符: %+v", m)
	}
}

// TestScanSentinel_AdminProject 票0015：create_ticket 可选第 5 段 project
// →NewTicket.project；缺段→空（网关继承）；set_project 哨兵→SetProject 帧
// TicketId 取本票。
func TestScanSentinel_AdminProject(t *testing.T) {
	s := newTestSession("t1")
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.md")
	_ = os.WriteFile(specFile, []byte("# spec"), 0o600)
	scan := func(l string) bool { return s.scanSentinel([]byte(l)) }

	if !scan("META_ADMIN:create_ticket|c1|子票|" + specFile + "|meta") {
		t.Fatal("create_ticket 五段哨兵未吞")
	}
	m := <-s.sendq
	nt := m.GetAdmin().GetCreateTicket()
	if nt.GetId() != "c1" || nt.GetProject() != "meta" {
		t.Fatalf("create_ticket project 段未落: %+v", nt)
	}
	if !scan("META_ADMIN:create_ticket|c2|子票|" + specFile) {
		t.Fatal("create_ticket 四段哨兵未吞")
	}
	m = <-s.sendq
	if nt = m.GetAdmin().GetCreateTicket(); nt.GetProject() != "" {
		t.Fatalf("缺第 5 段 project 应空（网关继承本票）: %+v", nt)
	}
	if !scan("META_ADMIN:set_project|meta") {
		t.Fatal("set_project 哨兵未吞")
	}
	m = <-s.sendq
	sp := m.GetAdmin().GetSetProject()
	if sp == nil || sp.TicketId != "t1" || sp.Project != "meta" {
		t.Fatalf("set_project 帧不符（TicketId 须取本票）: %+v", m)
	}
}

// ---------- D-32：净退无哨兵→ExitSeatErr 真跑（伪适配器+桩网关） ----------

type fakeAdapter struct{ argv []string }

func (f *fakeAdapter) Name() string { return "fake" }
func (f *fakeAdapter) Build(adapter.Spec) (adapter.Command, error) {
	return adapter.Command{Argv: f.argv}, nil
}
func (f *fakeAdapter) Parse([]byte) []adapter.Frame           { return nil }
func (f *fakeAdapter) Tokens() int64                          { return 0 }
func (f *fakeAdapter) FinalTokens(adapter.Spec) (int64, bool) { return 0, false }
func (f *fakeAdapter) Classify(int, string) adapter.ExitClass { return adapter.ExitOK }

type stubSeatGW struct {
	pbv1.UnimplementedSeatGatewayServer
	got chan *pbv1.SeatMsg
}

func (s *stubSeatGW) Attach(stream pbv1.SeatGateway_AttachServer) error {
	for {
		m, err := stream.Recv()
		if err != nil {
			return err
		}
		s.got <- m
		if m.GetDeliver() != nil {
			_ = stream.Send(&pbv1.GatewayMsg{Body: &pbv1.GatewayMsg_Ack{
				Ack: &pbv1.Ack{Ok: true}}})
		}
	}
}

func runSeatWithArgv(t *testing.T, role, script string) (int, []*pbv1.SeatMsg) {
	t.Helper()
	dir := t.TempDir()
	gsock := filepath.Join(dir, "g.sock")
	stub := &stubSeatGW{got: make(chan *pbv1.SeatMsg, 32)}
	ln, err := net.Listen("unix", gsock)
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pbv1.RegisterSeatGatewayServer(srv, stub)
	go srv.Serve(ln)
	t.Cleanup(srv.Stop)

	old := adapterNew
	adapterNew = func(string) (adapter.Adapter, error) {
		return &fakeAdapter{argv: []string{"/bin/sh", "-c", script}}, nil
	}
	t.Cleanup(func() { adapterNew = old })

	s := &session{
		spec: spec{
			Spec:        adapter.Spec{Shell: "fake", Role: role, WorkDir: dir},
			GatewaySock: gsock,
			SeatSock:    filepath.Join(dir, "seat.sock"),
			RunToken:    "tok", TicketID: "t1", RunID: "r1",
			AdoptGrace: time.Second,
		},
		sendq:   make(chan *pbv1.SeatMsg, 16),
		adopted: make(chan struct{}, 1),
		ackCh:   make(chan struct{}, 1),
	}
	code := s.run()
	var msgs []*pbv1.SeatMsg
	grace := time.After(300 * time.Millisecond)
drain:
	for { // 不关 got——Attach 处理流可能仍在收（关→panic）
		select {
		case m := <-stub.got:
			msgs = append(msgs, m)
		case <-grace:
			break drain
		}
	}
	return code, msgs
}

func hasDeliver(msgs []*pbv1.SeatMsg) *pbv1.Deliver {
	for _, m := range msgs {
		if m.GetDeliver() != nil {
			return m.GetDeliver()
		}
	}
	return nil
}

// 对抗锚/D-32 主锚：净退无 META_DELIVER 哨兵→ExitSeatErr＋Deliver 不出
// （死席径——完工没喊交货就当没交）。
func TestRun_NoSentinelNoDeliver(t *testing.T) {
	code, msgs := runSeatWithArgv(t, "施工", "echo quiet work done; exit 0")
	if code != ExitSeatErr {
		t.Fatalf("无哨兵净退应 ExitSeatErr: %d", code)
	}
	if d := hasDeliver(msgs); d != nil {
		t.Fatalf("无哨兵却送了 Deliver: %+v", d)
	}
}

func TestRun_DeliverSentinel(t *testing.T) {
	code, msgs := runSeatWithArgv(t, "施工", "echo META_DELIVER:ref42; exit 0")
	if code != ExitClean {
		t.Fatalf("有哨兵应 ExitClean: %d", code)
	}
	d := hasDeliver(msgs)
	if d == nil || d.Ref != "ref42" {
		t.Fatalf("Deliver.ref 不符: %+v", d)
	}
}

func TestRun_AuditVerdictSentinel(t *testing.T) {
	code, msgs := runSeatWithArgv(t, "审计", "echo META_VERDICT:pass:abc; exit 0")
	if code != ExitClean {
		t.Fatalf("审计席 verdict 哨兵应放行: %d", code)
	}
	d := hasDeliver(msgs)
	if d == nil || d.Verdict != "pass" || d.Ref != "abc" {
		t.Fatalf("审计 Deliver 不符: %+v", d)
	}
	// 审计席无 verdict 净退同样死席径。
	code2, msgs2 := runSeatWithArgv(t, "审计", "echo no verdict; exit 0")
	if code2 != ExitSeatErr || hasDeliver(msgs2) != nil {
		t.Fatalf("审计无哨兵净退应死席径: %d", code2)
	}
}

// TestClassOfExit_席位级（票0009 锚 4）：席位级死因（2/3/4/5）→ExitWrapper；
// ①/②a/②b/0 与未识码照旧。互逆侧 ExitCodeOf(ExitWrapper)=ExitInputErr。
func TestClassOfExit_席位级(t *testing.T) {
	for _, c := range []int{ExitSeatErr, ExitOrphan, ExitAbort, ExitInputErr} {
		if got := ClassOfExit(c); got != adapter.ExitWrapper {
			t.Fatalf("ClassOfExit(%d)=%v 预期 ExitWrapper", c, got)
		}
	}
	for code, want := range map[int]adapter.ExitClass{
		ExitClean:     adapter.ExitOK,
		ExitSpawnFail: adapter.ExitSpawn,
		ExitTransient: adapter.ExitTransient,
		ExitFatal:     adapter.ExitFatal,
		1:             adapter.ExitSpawn,
		-1:            adapter.ExitSpawn,
	} {
		if got := ClassOfExit(code); got != want {
			t.Fatalf("ClassOfExit(%d)=%v 预期 %v", code, got, want)
		}
	}
	if got := ExitCodeOf(adapter.ExitWrapper); got != ExitInputErr {
		t.Fatalf("ExitCodeOf(ExitWrapper)=%d 预期 %d", got, ExitInputErr)
	}
}

// TestRun_缺env（锚 4 钦定径）：META_* 不全→ExitSeatErr（席位级——
// 网关侧归 ExitWrapper，只落 seat_err 不标壳）。
func TestRun_缺env(t *testing.T) {
	for _, k := range []string{"META_SHELL", "META_GATEWAY_SOCK", "META_SEAT_SOCK",
		"META_RUN_TOKEN", "META_TICKET_ID"} {
		t.Setenv(k, "")
	}
	if got := Run(); got != ExitSeatErr {
		t.Fatalf("Run()=%d 预期 ExitSeatErr(%d)", got, ExitSeatErr)
	}
}

// TestRunChild_输入层（锚 4）：prompt 文件不可读与适配器 Build 失败→
// codeInputErr 哨兵（run() 落成 ExitInputErr→网关 ExitWrapper）。
func TestRunChild_输入层(t *testing.T) {
	ad, err := adapter.New("claude") // claude Build 恒成，stdin 文件径必失败
	if err != nil {
		t.Fatal(err)
	}
	s := &session{spec: spec{Spec: adapter.Spec{
		Shell: "claude", PromptFile: filepath.Join(t.TempDir(), "no-such.md")}}}
	code, tail := s.runChild(ad)
	if code != codeInputErr || tail == "" {
		t.Fatalf("prompt 不可读 runChild=(%d,%q) 预期 codeInputErr 哨兵", code, tail)
	}
	// Build 失败同码径（devin 缺 PromptFile/ArtifactDir 即拒）。
	ad2, err := adapter.New("devin")
	if err != nil {
		t.Fatal(err)
	}
	s2 := &session{spec: spec{Spec: adapter.Spec{Shell: "devin"}}}
	if code2, _ := s2.runChild(ad2); code2 != codeInputErr {
		t.Fatalf("Build 失败 runChild=%d 预期 codeInputErr", code2)
	}
}
