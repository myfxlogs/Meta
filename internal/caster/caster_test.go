package caster

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pbv1 "meta/internal/pb/metav1"
)

// ---------- 伪 AdminGateway ----------

type fakeObs struct {
	ch   chan *pbv1.ObsEvent
	kill chan struct{}
	ctx  context.Context
	grpc.ClientStream
}

func (s *fakeObs) Recv() (*pbv1.ObsEvent, error) {
	select {
	case e := <-s.ch:
		return e, nil
	case <-s.kill:
		return nil, errors.New("fake: 流被杀")
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *fakeObs) Header() (metadata.MD, error) { return nil, nil }
func (s *fakeObs) Trailer() metadata.MD         { return nil }
func (s *fakeObs) CloseSend() error             { return nil }
func (s *fakeObs) Context() context.Context     { return s.ctx }
func (s *fakeObs) SendMsg(any) error            { return nil }
func (s *fakeObs) RecvMsg(any) error            { return nil }

type fakeAdmin struct {
	mu         sync.Mutex
	events     []*pbv1.EventItem
	dispatches []*pbv1.DispatchReq
	streams    []*fakeObs
	dials      int
}

func (f *fakeAdmin) EventsAfter(_ context.Context, sinceID int64, limit int32) (*pbv1.EventList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := &pbv1.EventList{}
	for _, e := range f.events {
		if e.Id > sinceID {
			out.Items = append(out.Items, e)
			if int32(len(out.Items)) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeAdmin) Dispatch(_ context.Context, req *pbv1.DispatchReq) (*pbv1.Ack, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dispatches = append(f.dispatches, req)
	return &pbv1.Ack{Ok: true}, nil
}

func (f *fakeAdmin) Observe(ctx context.Context) (pbv1.AdminGateway_ObserveClient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := &fakeObs{ch: make(chan *pbv1.ObsEvent, 16), kill: make(chan struct{}), ctx: ctx}
	f.streams = append(f.streams, s)
	return s, nil
}

func (f *fakeAdmin) add(e *pbv1.EventItem) {
	f.mu.Lock()
	f.events = append(f.events, e)
	for _, s := range f.streams {
		select {
		case s.ch <- &pbv1.ObsEvent{Kind: e.Kind, TicketId: e.TicketId}:
		default:
		}
	}
	f.mu.Unlock()
}

func (f *fakeAdmin) killStream() {
	f.mu.Lock()
	for _, s := range f.streams {
		select {
		case <-s.kill:
		default:
			close(s.kill)
		}
	}
	f.mu.Unlock()
}

func (f *fakeAdmin) nDispatch() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.dispatches)
}

// ---------- fixture ----------

func writeRoutes(t *testing.T, dir string, body string) string {
	t.Helper()
	p := filepath.Join(dir, "caster.textproto")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// fixtureTable 覆盖锚 2/14 全钦定行（精简同构副本——含审计径新行与
// note_contains 分角色行）。
const fixtureTable = `
routes { kind: "stall_suspect"
  actions { notify { template: "N {{ticket_id}}" } }
  actions { dispatch { role: "CTO" adjunct: true prompt_template: "caster/ruling.md" work_dir: "/w" } } }
routes { kind: "budget_frozen" actions { notify { template: "F {{ticket_id}}" } } }
routes { kind: "deliver" role: "CTO"
  actions { notify { template: "CD {{ticket_id}}" } } }
routes { kind: "deliver" role: "施工"
  actions { notify { template: "D {{ticket_id}}" } }
  actions { dispatch { role: "审计" prefer_shell: "claude" work_dir: "/w"
                       prompt_template: "caster/audit.md" } } }
routes { kind: "audit_start" actions { notify { template: "A {{ticket_id}}" } } }
routes { kind: "audit_pass"  actions { notify { template: "P {{ticket_id}}" } } }
routes { kind: "reject"
  actions { notify { template: "R {{ticket_id}}" } }
  actions { dispatch { role: "施工" work_dir: "/w" } } }
routes { kind: "audit_dead" actions { dispatch { role: "审计" prefer_shell: "claude" work_dir: "/w" } } }
routes { kind: "spawn_needed" note_contains: "审计" actions { dispatch { role: "审计" prefer_shell: "claude" work_dir: "/w" } } }
routes { kind: "spawn_needed" note_contains: "施工" actions { dispatch { role: "施工" work_dir: "/w" } } }
routes { kind: "ruling" actions { notify { template: "L {{ticket_id}}" } } }
`

// writeCasterTemplates 在配置根落 fixture 模板件（prompt_template 相对配置根解）。
func writeCasterTemplates(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "caster"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "caster", "ruling.md"),
		[]byte("裁决 {{ticket_id}} {{note}} run_dir={{run_dir}} sock={{gateway_sock}}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "caster", "audit.md"),
		[]byte("审计 {{ticket_id}} {{note}}"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newEngine(t *testing.T, f *fakeAdmin, notifies *[]string, replay bool) (*Engine, string) {
	t.Helper()
	dir := t.TempDir()
	rp := writeRoutes(t, dir, fixtureTable)
	writeCasterTemplates(t, dir)
	eng, err := New(Config{
		RoutesPath: rp, StateDir: filepath.Join(dir, "state"),
		RunDir: "/run", SocketPath: "/sock", ReplayHistory: replay,
		Dial: func(ctx context.Context) (AdminClient, error) { return f, nil },
		Notify: func(ctx context.Context, text string) error {
			*notifies = append(*notifies, text)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return eng, dir
}

// TestLoadTable 锚 2：fixture 全行解析；未知动作/空 kind/空 role→拒载。
func TestLoadTable(t *testing.T) {
	dir := t.TempDir()
	tb, err := LoadTable(writeRoutes(t, dir, fixtureTable))
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Routes) != 11 {
		t.Fatalf("行数=%d 预期 11", len(tb.Routes))
	}
	for _, bad := range []string{
		`routes { kind: "" actions { notify { template: "x" } } }`,
		`routes { kind: "k" actions { } }`,
		`routes { kind: "k" actions { dispatch { role: "" } } }`,
		`routes { kind: "k"`, // 坏语法
	} {
		if _, err := LoadTable(writeRoutes(t, dir, bad)); err == nil {
			t.Fatalf("坏表应收拒载: %q", bad)
		}
	}
}

// TestResync 锚 3：stall→Dispatch{adjunct,role=CTO,prompt 含票+run 线索}；
// budget_frozen→notify；表外 kind→零调用。
func TestResync(t *testing.T) {
	f := &fakeAdmin{}
	var notifies []string
	eng, _ := newEngine(t, f, &notifies, true)
	f.add(&pbv1.EventItem{Id: 1, Kind: "stall_suspect", TicketId: "0007", Note: "超时 run=abc123"})
	f.add(&pbv1.EventItem{Id: 2, Kind: "budget_frozen", TicketId: "0007"})
	f.add(&pbv1.EventItem{Id: 3, Kind: "other_kind", TicketId: "0007"})

	if err := eng.Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.nDispatch() != 1 {
		t.Fatalf("Dispatch=%d 预期 1", f.nDispatch())
	}
	req := f.dispatches[0]
	if !req.Adjunct || req.Role != "CTO" {
		t.Fatalf("stall 派发不符: %+v", req)
	}
	pf, err := os.ReadFile(req.PromptFile)
	if err != nil || !strings.Contains(string(pf), "0007") ||
		!strings.Contains(string(pf), "run=abc123") {
		t.Fatalf("prompt 缺票号/run 线索: %s %v", pf, err)
	}
	if len(notifies) != 2 { // stall notify + budget_frozen notify；表外零
		t.Fatalf("notify=%d 预期 2: %v", len(notifies), notifies)
	}
}

// TestWM_Idempotent 锚 3 续：连跑两次第二次零动作；新行只处理 id>wm；
// 水位缺失首跑＝快进只推位。
func TestWM_Idempotent(t *testing.T) {
	f := &fakeAdmin{}
	var notifies []string
	eng, dir := newEngine(t, f, &notifies, false) // 水位缺失＝快进
	f.add(&pbv1.EventItem{Id: 1, Kind: "budget_frozen", TicketId: "t"})
	f.add(&pbv1.EventItem{Id: 2, Kind: "deliver", TicketId: "t"})
	if err := eng.Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.nDispatch() != 0 || len(notifies) != 0 {
		t.Fatal("快进模式不应动作")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "state", "watermark"))
	if strings.TrimSpace(string(b)) != "2" {
		t.Fatalf("水位=%q 预期 2（推进到现存最大 id）", b)
	}
	// 第二跑：零动作（wm=2 无新行）。
	f2 := &fakeAdmin{}
	dir2 := t.TempDir()
	writeCasterTemplates(t, dir2)
	eng2, _ := newEngineAt(t, f2, &notifies, filepath.Join(dir, "state"), false, writeRoutes(t, dir2, fixtureTable))
	if err := eng2.Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f2.nDispatch() != 0 || len(notifies) != 0 {
		t.Fatal("第二跑零动作")
	}
	// 推进：插新行→只处理 id>wm。
	f2.add(&pbv1.EventItem{Id: 3, Kind: "deliver", TicketId: "t", Role: "施工"})
	if err := eng2.Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f2.nDispatch() != 1 || f2.dispatches[0].Role != "审计" {
		t.Fatalf("新行未处理或越界: %+v", f2.dispatches)
	}
	// D-39：审计派发带 prompt_template→渲染落盘，PromptFile 非空含票号。
	pf := f2.dispatches[0].PromptFile
	if f2.dispatches[0].PreferShell != "claude" || pf == "" {
		t.Fatalf("deliver→审计派发不符（prefer/prompt_file）: %+v", f2.dispatches[0])
	}
	if b, err := os.ReadFile(pf); err != nil || !strings.Contains(string(b), "审计 t") {
		t.Fatalf("审计 prompt 未渲染票号: %s %v", pf, err)
	}
}

// newEngineAt 复用既有 state-dir（水位线延续）的引擎。
func newEngineAt(t *testing.T, f *fakeAdmin, notifies *[]string, stateDir string, replay bool, routesPath string) (*Engine, string) {
	t.Helper()
	eng, err := New(Config{
		RoutesPath: routesPath, StateDir: stateDir,
		RunDir: "/run", SocketPath: "/sock", ReplayHistory: replay,
		Dial:   func(ctx context.Context) (AdminClient, error) { return f, nil },
		Notify: func(ctx context.Context, text string) error {
			*notifies = append(*notifies, text)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return eng, filepath.Dir(routesPath)
}

// TestReconnect 锚 4：流杀→退避重连→间隙行被补处理（有界等待断言）。
func TestReconnect(t *testing.T) {
	f := &fakeAdmin{}
	var notifies []string
	eng, _ := newEngine(t, f, &notifies, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- eng.Run(ctx) }()

	f.add(&pbv1.EventItem{Id: 1, Kind: "budget_frozen", TicketId: "t"})
	waitFor(t, 5*time.Second, func() bool { return len(notifies) >= 1 })

	f.killStream()
	// 间隙内落库的行：重连成功后 resync 补处理。
	f.add(&pbv1.EventItem{Id: 2, Kind: "deliver", TicketId: "t", Role: "施工"})
	waitFor(t, 8*time.Second, func() bool { return f.nDispatch() >= 1 })
	cancel()
	<-done
}

// TestSpawnNeededRoutes 锚 14：note_contains 分角色——审计/施工各一派，
// CTO 行表外零调用；deliver→Dispatch{审计,prefer=claude,prompt 空}。
func TestSpawnNeededRoutes(t *testing.T) {
	f := &fakeAdmin{}
	var notifies []string
	eng, _ := newEngine(t, f, &notifies, true)
	f.add(&pbv1.EventItem{Id: 1, Kind: "spawn_needed", TicketId: "t1", Note: "对账补发: 审计"})
	f.add(&pbv1.EventItem{Id: 2, Kind: "spawn_needed", TicketId: "t2", Note: "对账补发: 施工"})
	f.add(&pbv1.EventItem{Id: 3, Kind: "spawn_needed", TicketId: "t3", Note: "对账补发: CTO"})
	if err := eng.Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.nDispatch() != 2 {
		t.Fatalf("Dispatch=%d 预期 2（CTO 表外）: %+v", f.nDispatch(), f.dispatches)
	}
	if f.dispatches[0].Role != "审计" || f.dispatches[1].Role != "施工" {
		t.Fatalf("分角色错: %+v", f.dispatches)
	}
}

// TestRender_缺省模板（决策41）：notify 空模板回缺省形
// 【Meta】{{role}}:{{action}}[{{ticket_id}}]——动作中文化映射在渲染层。
func TestRender_缺省模板(t *testing.T) {
	f := &fakeAdmin{}
	var notifies []string
	eng, _ := newEngine(t, f, &notifies, true)
	row := &pbv1.EventItem{Id: 1, Kind: "audit_pass", TicketId: "0009", Role: "审计"}
	if got := eng.render("", row, true); got != "【Meta】审计:查验通过[0009]" {
		t.Fatalf("缺省模板不符: %q", got)
	}
	// 表外 kind 回原文（渲染兜底，路由仍只走表内行）。
	row2 := &pbv1.EventItem{Id: 2, Kind: "weird_kind", TicketId: "0010", Role: ""}
	if got := eng.render("", row2, true); got != "【Meta】:weird_kind[0010]" {
		t.Fatalf("表外 kind 兜底不符: %q", got)
	}
}

func waitFor(t *testing.T, d time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		<-time.After(10 * time.Millisecond)
	}
	t.Fatal("有界等待超时")
}

// TestRoleFilter 锚 2（D-39＋对抗性锚 a）：行 role 非空＝事件 role 等值才动作。
// deliver{role:施工}→dispatch 审计；deliver{role:CTO}→零 dispatch 只 notify；
// 空 role 行照旧全匹配（回归）；role 与 note_contains 并存＝两条件都过。
// 删 process() 的 `rt.Role != "" && rt.Role != row.Role` 本表必红。
func TestRoleFilter(t *testing.T) {
	dir := t.TempDir()
	table := `
routes { kind: "deliver" role: "CTO"
  actions { notify { template: "CTO交付 {{ticket_id}}" } } }
routes { kind: "deliver" role: "施工"
  actions { notify { template: "施工交付 {{ticket_id}}" } }
  actions { dispatch { role: "审计" prefer_shell: "claude" work_dir: "/w"
                       prompt_template: "audit.md" } } }
routes { kind: "ping"
  actions { dispatch { role: "施工" work_dir: "/w" } } }
routes { kind: "multi" role: "施工" note_contains: "go"
  actions { dispatch { role: "审计" work_dir: "/w" } } }
`
	rp := writeRoutes(t, dir, table)
	if err := os.WriteFile(filepath.Join(dir, "audit.md"),
		[]byte("审计 {{ticket_id}}"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &fakeAdmin{}
	var notifies []string
	eng, err := New(Config{
		RoutesPath: rp, StateDir: filepath.Join(dir, "state"),
		RunDir: "/run", SocketPath: "/sock", ReplayHistory: true,
		Dial: func(ctx context.Context) (AdminClient, error) { return f, nil },
		Notify: func(ctx context.Context, text string) error {
			notifies = append(notifies, text)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.add(&pbv1.EventItem{Id: 1, Kind: "deliver", TicketId: "0009", Role: "施工"})
	f.add(&pbv1.EventItem{Id: 2, Kind: "deliver", TicketId: "0007", Role: "CTO"})
	f.add(&pbv1.EventItem{Id: 3, Kind: "ping", TicketId: "p", Role: "任意"})
	f.add(&pbv1.EventItem{Id: 4, Kind: "multi", TicketId: "m1", Role: "施工", Note: "no"})
	f.add(&pbv1.EventItem{Id: 5, Kind: "multi", TicketId: "m2", Role: "CTO", Note: "go"})
	f.add(&pbv1.EventItem{Id: 6, Kind: "multi", TicketId: "m3", Role: "施工", Note: "go"})
	if err := eng.Once(context.Background()); err != nil {
		t.Fatal(err)
	}

	// dispatch 只三枚：deliver施工→审计、ping（空 role 行全匹配）→施工、
	// multi施工+go→审计。deliver CTO 与 multi 两失格行零 dispatch。
	if f.nDispatch() != 3 {
		t.Fatalf("Dispatch=%d 预期 3: %+v", f.nDispatch(), f.dispatches)
	}
	d0 := f.dispatches[0]
	if d0.Role != "审计" || d0.TicketId != "0009" {
		t.Fatalf("deliver施工应派审计: %+v", d0)
	}
	if b, err := os.ReadFile(d0.PromptFile); err != nil ||
		!strings.Contains(string(b), "0009") {
		t.Fatalf("审计 prompt 未渲染: %s %v", d0.PromptFile, err)
	}
	if f.dispatches[1].Role != "施工" || f.dispatches[2].TicketId != "m3" {
		t.Fatalf("派发序不符: %+v", f.dispatches)
	}
	// notify：deliver 两发（施工交付＋CTO交付），deliver CTO 只通知不进审。
	var hasCTO, hasW bool
	for _, n := range notifies {
		if strings.Contains(n, "CTO交付") {
			hasCTO = true
		}
		if strings.Contains(n, "施工交付") {
			hasW = true
		}
	}
	if !hasCTO || !hasW {
		t.Fatalf("deliver 两行 notify 不齐: %v", notifies)
	}
}

// TestProdConfig 锚 6：仓内 config/caster.textproto 加载全绿；deliver 按
// role 拆两行在；审计 dispatch 行全带 prompt_template；audit-review.md
// 存在且渲染含票号＋META_VERDICT 哨兵纪律＋meta events/phase 自取指引。
func TestProdConfig(t *testing.T) {
	tb, err := LoadTable(filepath.Join("..", "..", "config", "caster.textproto"))
	if err != nil {
		t.Fatalf("生产路由表拒载: %v", err)
	}
	var ctoLine, workLine bool
	auditDisp := 0
	for _, r := range tb.Routes {
		if r.Kind != "deliver" {
			continue
		}
		switch r.Role {
		case "CTO":
			ctoLine = true
			for _, a := range r.Actions {
				if a.GetDispatch() != nil {
					t.Fatal("deliver/CTO 行只许通知不许派单")
				}
			}
		case "施工":
			workLine = true
		default:
			t.Fatalf("deliver 行带意外 role: %q", r.Role)
		}
	}
	for _, r := range tb.Routes {
		for _, a := range r.Actions {
			if d := a.GetDispatch(); d != nil && d.Role == "审计" {
				auditDisp++
				if d.PromptTemplate != "caster/audit-review.md" {
					t.Fatalf("审计 dispatch 行缺 prompt_template: %+v", d)
				}
			}
		}
	}
	if !ctoLine || !workLine {
		t.Fatalf("deliver 未按 role 拆两行: CTO=%v 施工=%v", ctoLine, workLine)
	}
	if auditDisp != 3 {
		t.Fatalf("审计 dispatch 行数=%d 预期 3（deliver/audit_dead/spawn_needed）", auditDisp)
	}

	b, err := os.ReadFile(filepath.Join("..", "..", "config", "caster", "audit-review.md"))
	if err != nil {
		t.Fatalf("audit-review.md 缺失: %v", err)
	}
	for _, s := range []string{"{{ticket_id}}", "META_VERDICT", "meta events", "meta phase"} {
		if !strings.Contains(string(b), s) {
			t.Fatalf("审计模板缺 %q", s)
		}
	}
	// 渲染含真实票号、占位符被消化。
	out := (&Engine{}).render(string(b),
		&pbv1.EventItem{TicketId: "0009"}, false)
	if !strings.Contains(out, "0009") || strings.Contains(out, "{{ticket_id}}") {
		t.Fatal("模板渲染未消化 {{ticket_id}}")
	}
}
// TestDeployRoutes 票0010 锚9：真路由表 config/caster.textproto——
// audit_pass→Dispatch{role:"CTO"}（部署支派发径）；deploy_failed→notify。
func TestDeployRoutes(t *testing.T) {
	tb, err := LoadTable("../../config/caster.textproto")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeAdmin{}
	var notifies []string
	e := &Engine{table: tb, configRoot: "../../config",
		cfg: Config{StateDir: t.TempDir(),
			Notify: func(_ context.Context, s string) error {
				notifies = append(notifies, s)
				return nil
			},
			Logf: func(string, ...any) {}}}
	e.process(context.Background(), f, &pbv1.EventItem{Kind: "audit_pass", TicketId: "t1"})
	if f.nDispatch() != 1 || f.dispatches[0].Role != "CTO" {
		t.Fatalf("audit_pass 应派 CTO-B: %+v", f.dispatches)
	}
	e.process(context.Background(), f, &pbv1.EventItem{Kind: "deploy_failed", TicketId: "t2"})
	if len(notifies) == 0 {
		t.Fatal("deploy_failed 应触发 notify")
