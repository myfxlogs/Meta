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
routes { kind: "deliver"
  actions { notify { template: "D {{ticket_id}}" } }
  actions { dispatch { role: "审计" prefer_shell: "claude" work_dir: "/w" } } }
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

func newEngine(t *testing.T, f *fakeAdmin, notifies *[]string, replay bool) (*Engine, string) {
	t.Helper()
	dir := t.TempDir()
	rp := writeRoutes(t, dir, fixtureTable)
	// 模板落点：相对配置根（routes 父目录）的 caster/ruling.md
	if err := os.MkdirAll(filepath.Join(dir, "caster"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "caster", "ruling.md"),
		[]byte("裁决 {{ticket_id}} {{note}} run_dir={{run_dir}} sock={{gateway_sock}}"), 0o600); err != nil {
		t.Fatal(err)
	}
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
	if len(tb.Routes) != 10 {
		t.Fatalf("行数=%d 预期 10", len(tb.Routes))
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
	eng2, _ := newEngineAt(t, f2, &notifies, filepath.Join(dir, "state"), false, writeRoutes(t, t.TempDir(), fixtureTable))
	if err := eng2.Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f2.nDispatch() != 0 || len(notifies) != 0 {
		t.Fatal("第二跑零动作")
	}
	// 推进：插新行→只处理 id>wm。
	f2.add(&pbv1.EventItem{Id: 3, Kind: "deliver", TicketId: "t"})
	if err := eng2.Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f2.nDispatch() != 1 || f2.dispatches[0].Role != "审计" {
		t.Fatalf("新行未处理或越界: %+v", f2.dispatches)
	}
	if f2.dispatches[0].PreferShell != "claude" || f2.dispatches[0].PromptFile != "" {
		t.Fatalf("deliver→审计派发不符（prefer/prompt_file）: %+v", f2.dispatches[0])
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
	f.add(&pbv1.EventItem{Id: 2, Kind: "deliver", TicketId: "t"})
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
