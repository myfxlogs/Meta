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
	scan := func(l string) bool { return s.scanSentinel(l, false) }
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
	scan := func(l string) bool { return s.scanSentinel(l, false) }

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

// ---------- D-32：净退无哨兵→ExitSeatErr 真跑（伪适配器+桩网关） ----------

type fakeAdapter struct{ argv []string }

func (f *fakeAdapter) Name() string { return "fake" }
func (f *fakeAdapter) Build(adapter.Spec) (adapter.Command, error) {
	return adapter.Command{Argv: f.argv}, nil
}

// Parse 直通帧：D-42 后哨兵走帧 Text 逐段扫描，伪适配器须原行透传
// （与 devin 适配器的 Text=原行 同形）。
func (f *fakeAdapter) Parse(l []byte) []adapter.Frame {
	return []adapter.Frame{{Kind: adapter.FrameText, Note: string(l), Text: string(l)}}
}
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

// parseAdapter 真 Parse 假起壳（D-42 测试件）：Build 换脚本 argv——
// JSON 壳哨兵/不可解析行测试须走真适配器文本流，又不能真起壳 CLI。
type parseAdapter struct {
	adapter.Adapter
	argv []string
}

func (p *parseAdapter) Build(adapter.Spec) (adapter.Command, error) {
	return adapter.Command{Argv: p.argv}, nil
}

func runSeatWithArgv(t *testing.T, role, script string) (int, []*pbv1.SeatMsg) {
	t.Helper()
	return runSeatSession(t, role, "",
		&fakeAdapter{argv: []string{"/bin/sh", "-c", script}})
}

// runSeatSession 起一次真席位（桩网关＋预造适配器）；artDir 非空挂
// META_ART_DIR（sentinel.log 文件兜底径测试用）。
func runSeatSession(t *testing.T, role, artDir string, ad adapter.Adapter) (int, []*pbv1.SeatMsg) {
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
	adapterNew = func(string) (adapter.Adapter, error) { return ad, nil }
	t.Cleanup(func() { adapterNew = old })

	s := &session{
		spec: spec{
			Spec:        adapter.Spec{Shell: "fake", Role: role, WorkDir: dir, ArtifactDir: artDir},
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

// ---------- D-42：哨兵扫描移层＋sentinel.log 文件兜底 ----------

// TestScanSentinel_TextLines（对抗锚）：帧 Text 多行块内嵌哨兵行命中；
// 行中提及 META_DELIVER:（非行首）不命中——旁注免疫。
func TestScanSentinel_TextLines(t *testing.T) {
	s := newTestSession("t1")
	live := s.scanFrameText("干活正文\nMETA_DELIVER:ref99\n收尾正文")
	if !s.deliverSeen || s.deliverRef != "ref99" {
		t.Fatalf("内嵌哨兵行未命中: seen=%v ref=%q", s.deliverSeen, s.deliverRef)
	}
	if !live {
		t.Fatal("含正文段的帧应算活性")
	}
	// 全哨兵段帧不算活性。
	s2 := newTestSession("t1")
	if s2.scanFrameText("META_DELIVER:r1\nMETA_DEPLOY:done") {
		t.Fatal("全哨兵段帧误报活性")
	}
	// 行中提及（非行首）不命中——保「单独一行」钦定。
	s3 := newTestSession("t1")
	s3.scanFrameText("旁注：末行须 META_DELIVER:x 钦定")
	if s3.deliverSeen {
		t.Fatal("行中提及哨兵被误命中")
	}
}

// TestRun_JSONShellSentinel（D-42 主锚）：真 claude 适配器＋脚本吐
// assistant JSONL（文本块含 \nMETA_VERDICT:pass:vr1\n）——JSON 包裹的
// 哨兵经帧 Text 流命中，原始行扫描永不可能捕到。
func TestRun_JSONShellSentinel(t *testing.T) {
	real, err := adapter.New("claude")
	if err != nil {
		t.Fatal(err)
	}
	script := `printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"干活中\nMETA_VERDICT:pass:vr1\n收尾"}]}}'; exit 0`
	code, msgs := runSeatSession(t, "审计", "",
		&parseAdapter{Adapter: real, argv: []string{"/bin/sh", "-c", script}})
	if code != ExitClean {
		t.Fatalf("JSON 壳哨兵应 ExitClean: %d", code)
	}
	d := hasDeliver(msgs)
	if d == nil || d.Verdict != "pass" || d.Ref != "vr1" {
		t.Fatalf("Deliver 不符: %+v", d)
	}
}

// TestRun_MergedLineFileFallback（对抗锚）：stdout 粘并行（哨兵非行首，
// 流源不中）＋sentinel.log 预置净行→文件兜底捕获，Deliver.Ref 正确。
func TestRun_MergedLineFileFallback(t *testing.T) {
	art := t.TempDir()
	if err := os.WriteFile(filepath.Join(art, "sentinel.log"),
		[]byte("META_DELIVER:ref7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, msgs := runSeatSession(t, "施工", art,
		&fakeAdapter{argv: []string{"/bin/sh", "-c",
			"echo '前缀粘文META_DELIVER:polluted'; exit 0"}})
	if code != ExitClean {
		t.Fatalf("文件兜底哨兵应 ExitClean: %d", code)
	}
	d := hasDeliver(msgs)
	if d == nil || d.Ref != "ref7" {
		t.Fatalf("Deliver.ref 不符（文件净行应胜）: %+v", d)
	}
}

// TestSentinelFileDedup（对抗锚）：stdout＋文件双发同一 META_ADMIN 行→
// 原文配对消化，CreateTicket 帧仅一条；污染行与净行不配对各发各的。
func TestSentinelFileDedup(t *testing.T) {
	s := newTestSession("t1")
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.md")
	_ = os.WriteFile(specFile, []byte("# spec"), 0o600)
	line := "META_ADMIN:create_ticket|c1|子票|" + specFile

	s.scanSentinel(line, false) // 流源
	s.scanSentinel(line, true)  // 文件源同行→配对消化
	count := func() int {
		n := 0
		for {
			select {
			case m := <-s.sendq:
				if m.GetAdmin().GetCreateTicket() != nil {
					n++
				}
			default:
				return n
			}
		}
	}
	if n := count(); n != 1 {
		t.Fatalf("双发同文哨兵应只出一条 admin 帧: %d", n)
	}
	// 对称序：文件先见，流中再见同样配对消化。
	if s.pending[line] != 0 {
		t.Fatalf("配对后 pending 应归 0: %d", s.pending[line])
	}
	s.scanSentinel(line, true)
	s.scanSentinel(line, false)
	if n := count(); n != 1 {
		t.Fatalf("文件先序配对应仍只一条: %d", n)
	}
	// 污染流行与文件净行不配对（配对键＝原文整体）——各发各的。
	polluted := line + "尾巴"
	s.scanSentinel(line, true)
	s.scanSentinel(polluted, false)
	if n := count(); n != 2 {
		t.Fatalf("污染行与净行应各发各的: %d", n)
	}
}

// TestRun_RawLineFallback（对抗锚）：fd-1 直写形不可解析行上哨兵
// 经适配器透传帧（Text=原行）仍捕获。
func TestRun_RawLineFallback(t *testing.T) {
	real, err := adapter.New("claude")
	if err != nil {
		t.Fatal(err)
	}
	script := "echo 'META_DELIVER:rawref'; exit 0" // claude 解析失败→Text=原行
	code, msgs := runSeatSession(t, "施工", "",
		&parseAdapter{Adapter: real, argv: []string{"/bin/sh", "-c", script}})
	if code != ExitClean {
		t.Fatalf("原行哨兵应 ExitClean: %d", code)
	}
	d := hasDeliver(msgs)
	if d == nil || d.Ref != "rawref" {
		t.Fatalf("Deliver.ref 不符: %+v", d)
	}
}
