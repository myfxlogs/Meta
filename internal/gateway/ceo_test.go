package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"strings"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// ceoTestCfg CEO 面测试配置：SeatArgv=helper，CEO 壳行 fixture，kind 集钉死。
func ceoTestCfg(dir string, promptTmpl bool) Config {
	c := dispatchCfg(dir)
	c.ShellRows = append(c.ShellRows, ShellSeed{Name: "testshell", Role: "CEO", Priority: 10})
	c.CeoEvents = map[string]bool{"reject_limit": true, "requirement": true,
		"ruling": true, "ceo_say": true, "ceo_gap": true, "ceo_last_resort": true}
	c.CeoSpawnKinds = map[string]bool{"reject_limit": true}
	if promptTmpl {
		p := filepath.Join(dir, "ceo-headless.md")
		_ = os.WriteFile(p, []byte("化身 {{kind}} {{ticket_id}} {{note}} {{run_dir}} {{gateway_sock}}"), 0o600)
		c.CeoPrompt = p
	}
	return c
}

func ceoClient(t *testing.T, dir string) pbv1.CeoGatewayClient {
	t.Helper()
	conn, err := grpc.NewClient("unix://"+filepath.Join(dir, "run", "meta.sock"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return pbv1.NewCeoGatewayClient(conn)
}

// mkCeoRun 伪 spawnCEO：直接立 role=CEO 无进程 run（prov 可指定——
// feishu:x 伪戳用来验「自报不认只认 run 行」）。
func mkCeoRun(t *testing.T, g *Gateway, prov string) *db.Run {
	t.Helper()
	run := &db.Run{
		RunID: randHex(8), TicketID: "", Role: "CEO", Shell: "testshell",
		PID: 1, RunToken: randHex(16),
		SocketPath: filepath.Join(g.cfg.RunDir, "seats", "x.sock"),
		Provenance: prov,
	}
	if err := g.DB().CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	return run
}

// TestCeoLock 锚2 锁：A 订阅中 B Events→FailedPrecondition；A 断→B 得。
func TestCeoLock(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	c := ceoClient(t, dir)
	runA := mkCeoRun(t, g, "terminal")
	runB := mkCeoRun(t, g, "terminal")

	ctxA, cancelA := context.WithCancel(context.Background())
	sA, err := c.Events(ctxA, &pbv1.Subscribe{RunToken: runA.RunToken, Follow: true})
	_ = sA // 流活着即可——EOF 径走 ctx 取消（server-streaming 无半关语义）
	if err != nil {
		t.Fatal(err)
	}
	waitFor2(t, 3*time.Second, func() bool { return g.ceoActiveSub() != nil })

	sB, err := c.Events(context.Background(), &pbv1.Subscribe{RunToken: runB.RunToken, Follow: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sB.Recv(); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("第二订阅未被拒: %v", err)
	}

	// A 断（ctx 取消＝EOF 径）→B 得。
	cancelA()
	waitFor2(t, 3*time.Second, func() bool { return g.ceoActiveSub() == nil })
	sB2, err := c.Events(context.Background(), &pbv1.Subscribe{RunToken: runB.RunToken, Follow: false})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sB2.Recv(); err != nil && status.Code(err) == codes.FailedPrecondition {
		t.Fatalf("A 断后 B 仍被拒: %v", err)
	}
	sB2.CloseSend()
}

// TestCeoSpawn 锚2 唤起：spawn_kind 事件无订阅→seat_run 出 role=CEO run；
// 二事件不重复 spawn；连败 3→ceo_last_resort。
func TestCeoSpawn(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "ceo")
	t.Setenv("META_ADOPT_GRACE", "30")
	g, _ := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()

	g.emit(Event{Kind: "reject_limit", TicketID: "t-x"})
	waitFor2(t, 15*time.Second, func() bool {
		r, _ := g.DB().ActiveRunForRole(ctx, "CEO")
		return r != nil && r.Provenance == "system_event"
	})
	g.emit(Event{Kind: "reject_limit", TicketID: "t-x"})
	<-time.After(300 * time.Millisecond)
	var n int
	runs, _ := g.DB().RunsForTicket(ctx, "")
	for _, r := range runs {
		if r.Role == "CEO" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("二事件重复 spawn：CEO runs=%d", n)
	}
	// 杀掉化身→D-22 比对不炸（未派发事件零计数早退）。
	r, _ := g.DB().ActiveRunForRole(ctx, "CEO")
	if r != nil {
		g.killRun(g.bindRun(r))
	}
}

// TestCeoSpawn_LastResort 连败 3→ceo_last_resort 行（SeatArgv 指向不存在二进制）。
func TestCeoSpawn_LastResort(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		*c = ceoTestCfg(filepath.Dir(c.DBPath), true)
		c.SeatArgv = []string{"/nonexistent-bin"}
	})
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		g.emit(Event{Kind: "reject_limit", TicketID: "t-y"})
		<-time.After(150 * time.Millisecond)
	}
	waitFor2(t, 5*time.Second, func() bool {
		evs, _ := g.DB().Events(ctx, "t-y")
		for _, e := range evs {
			if e.Kind == "ceo_last_resort" {
				return true
			}
		}
		return false
	})
	_ = dir
}

// TestCeoProvenance 锚3：spawnCEO(prov=feishu:x)→SubmitRequirement→需求票
// created/requirement 行 prov=feishu:x；自报字段被盖；Dispatch(role=CEO) 硬拒。
func TestCeoProvenance(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()
	c := ceoClient(t, dir)
	run := mkCeoRun(t, g, "feishu:owner-1")

	ref, err := c.SubmitRequirement(ctx, &pbv1.Requirement{
		RunToken: run.RunToken, Goal: "做个新功能", Direction: "先小后大",
		Provenance: "terminal", // 席自报——必须被盖成 feishu:x
	})
	if err != nil {
		t.Fatal(err)
	}
	evs, _ := g.DB().Events(ctx, ref.Id)
	var created, req *db.Event
	for _, e := range evs {
		if e.Kind == "created" {
			created = e
		}
		if e.Kind == "requirement" {
			req = e
		}
	}
	if created == nil || req == nil {
		t.Fatal("created/requirement 行不齐")
	}
	if created.Provenance != "feishu:owner-1" || req.Provenance != "feishu:owner-1" {
		t.Fatalf("provenance 未盖 run 行戳: created=%q req=%q", created.Provenance, req.Provenance)
	}
	ack, _ := g.Dispatch(ctx, &pbv1.DispatchReq{TicketId: ref.Id, Role: "CEO"})
	if ack == nil || ack.Ok {
		t.Fatalf("Dispatch(role=CEO) 未硬拒: %+v", ack)
	}
	if err := g.DispatchSeat(ctx, ref.Id, "CEO", SeatSpec{}); err == nil {
		t.Fatal("DispatchSeat(role=CEO) 未硬拒")
	}
}

// TestCeoRule 锚4：void 两步确认；budget_raise 解冻；redirect 重派 CTO。
func TestCeoRule(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "worker") // redirect 重派 CTO→helper 席活起
	t.Setenv("META_ADOPT_GRACE", "30")
	g, dir := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()
	c := ceoClient(t, dir)
	run := mkCeoRun(t, g, "terminal")

	// void：无 confirm→need_confirm；有→作废。
	mkTicketAt(t, g, "tv", db.PhaseWorking)
	ack, err := c.Rule(ctx, &pbv1.Ruling{RunToken: run.RunToken, Action: "void", TicketId: "tv"})
	if err != nil || !ack.NeedConfirm {
		t.Fatalf("无 challenge 未拒: %v %+v", err, ack)
	}
	ack, _ = c.Rule(ctx, &pbv1.Ruling{RunToken: run.RunToken, Action: "void",
		TicketId: "tv", Confirm: true, Note: "裁掉"})
	if !ack.Ok {
		t.Fatalf("confirm void 未成: %+v", ack)
	}
	if p, _ := g.DB().PhaseOf(ctx, "tv"); p != db.PhaseVoid {
		t.Fatalf("相位=%s 预期 作废", p)
	}
	if hasEvent(t, g, "tv", "ruling") == nil {
		t.Fatal("ruling 行未落账")
	}

	// budget_raise：冻票→抬帽解冻。
	mkTicketAt(t, g, "tb", db.PhaseWorking)
	if err := g.DB().SetBudgetCaps(ctx, "tb", 100, 150); err != nil {
		t.Fatal(err)
	}
	if err := g.DB().SetBudgetFrozen(ctx, "tb", true); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	g.frozen["tb"] = true
	g.mu.Unlock()
	ack, _ = c.Rule(ctx, &pbv1.Ruling{RunToken: run.RunToken, Action: "budget_raise",
		TicketId: "tb", HardCap: 1000})
	if !ack.Ok {
		t.Fatalf("budget_raise 未成: %+v", ack)
	}
	tk, _ := g.DB().GetTicket(ctx, "tb")
	if tk.BudgetFrozen {
		t.Fatal("抬帽未解冻")
	}

	// redirect：泊车票→重派 CTO run。
	mkTicketAt(t, g, "trp", db.PhaseWorking)
	if _, err := g.DB().Transition(ctx, "trp", db.PhaseCreated, db.EdgeReqPending, "cto-run", "需求待裁"); err != nil {
		t.Fatal(err)
	}
	ack, _ = c.Rule(ctx, &pbv1.Ruling{RunToken: run.RunToken, Action: "redirect",
		TicketId: "trp", Note: "换方向重做"})
	if !ack.Ok {
		t.Fatalf("redirect 未成: %+v", ack)
	}
	waitFor2(t, 10*time.Second, func() bool {
		r, _ := g.DB().ActiveRunForTicketRole(ctx, "trp", "CTO")
		return r != nil
	})
}

// TestCeoInquire 锚5：Inquire→adjunct CTO run 起＋adjunct_deliver 回。
func TestCeoInquire(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "verdict") // adjunct 席自动 Attach+Claim+Deliver+退
	t.Setenv("META_ADOPT_GRACE", "1")
	g, dir := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()
	c := ceoClient(t, dir)
	run := mkCeoRun(t, g, "terminal")
	mkTicketAt(t, g, "ti", db.PhaseWorking)

	ack, err := c.Inquire(ctx, &pbv1.Inquiry{
		RunToken: run.RunToken, TicketId: "ti", Question: "这票卡哪了？"})
	if err != nil || !ack.Ok {
		t.Fatalf("Inquire 失败: %v %+v", err, ack)
	}
	waitEvent(t, g, "adjunct_deliver", "ti", 10*time.Second)
	r, _ := g.DB().ActiveAdjunctForTicketRole(ctx, "ti", "CTO")
	if r == nil || r.Provenance != "terminal" {
		t.Fatalf("adjunct run 未起或戳未承: %+v", r)
	}
	if hasEvent(t, g, "ti", "adjunct_deliver") == nil {
		t.Fatal("adjunct_deliver 行未落账")
	}
}

// TestCeoMemory 锚6：put/get 往返；kind=say→ceo_say 行+事件；put 落 event_ref=run_id。
func TestCeoMemory(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()
	c := ceoClient(t, dir)
	run := mkCeoRun(t, g, "terminal")

	st, err := c.Memory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	put := func(kind, title, content string) *pbv1.MemoryRow {
		if err := st.Send(&pbv1.MemoryOp{RunToken: run.RunToken, Op: &pbv1.MemoryOp_Put{
			Put: &pbv1.MemoryPut{Kind: kind, Title: title, Content: []byte(content)}}}); err != nil {
			t.Fatal(err)
		}
		m, err := st.Recv()
		if err != nil {
			t.Fatal(err)
		}
		return m.GetRow()
	}
	row := put("decision", "裁一", "理由链")
	if row.EventRef != run.RunID || row.Provenance != "terminal" {
		t.Fatalf("put 未盖 run_id/戳: %+v", row)
	}
	put("say", "", "给业主的话")
	waitEvent(t, g, "ceo_say", "", 3*time.Second)
	evs, _ := g.DB().Events(ctx, "")
	var say *db.Event
	for _, e := range evs {
		if e.Kind == "ceo_say" {
			say = e
		}
	}
	if say == nil {
		t.Fatal("ceo_say 行未落账")
	}
	if err := st.Send(&pbv1.MemoryOp{RunToken: run.RunToken, Op: &pbv1.MemoryOp_Get{
		Get: &pbv1.MemoryGet{Kind: "decision"}}}); err != nil {
		t.Fatal(err)
	}
	m, err := st.Recv()
	if err != nil || m.GetRow().Title != "裁一" {
		t.Fatalf("get 往返不符: %v %+v", err, m)
	}
	st.CloseSend()
}

// TestReqPending 锚7：CTO 席 Yield→已立+req_pending 行；对账不补 spawn；
// redirect 后重派 CTO（重定向最新事件非 req_pending，抑制自然解）。
func TestReqPending(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()
	mkTicketAt(t, g, "tp", db.PhaseAssigned)
	run := spawnSeat(t, g, "tp", "CTO", "idle")
	_, sc := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	st, _ := sc.Attach(ctx)
	st.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: run.RunToken}}})
	st.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: "tp"}}})
	waitPhase(t, g, "tp", db.PhaseWorking, 5*time.Second)
	st.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Yield{Yield: &pbv1.Yield{Kind: "需求待裁", Reason: "动钱"}}})
	waitPhase(t, g, "tp", db.PhaseCreated, 5*time.Second)
	if hasEvent(t, g, "tp", "req_pending") == nil {
		t.Fatal("req_pending 行未落账")
	}
	st.CloseSend()

	// 对账抑制闸（对抗锚③）：最新事件 req_pending→不补 spawn_needed。
	g.Reconcile(ctx)
	for _, e := range collectEvents(g, 300*time.Millisecond) {
		if e.Kind == "spawn_needed" && e.TicketID == "tp" {
			t.Fatal("泊车票被对账重派")
		}
	}
	// 普通 已立 票对照：照补。
	mkTicketAt(t, g, "tp2", db.PhaseCreated)
	g.Reconcile(ctx)
	waitEvent(t, g, "spawn_needed", "tp2", 3*time.Second)
}

// TestImportLegacy 锚8：fixture md→行（幂等二跑零增）。
func TestImportLegacy(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()
	md := []byte("# m\n\n### 决策 1：甲\n- provenance：`terminal`\n- 理由 A\n\n### 决策 2：乙\n- provenance：`feishu:x`\n- 理由 B\n")
	d := g.DB()
	// 直调 db 层（importLegacy 在 cmd/meta——此处验证库层幂等语义经 MemoryHas）。
	parse := func() int {
		n := 0
		for _, tt := range []struct{ title, prov string }{{"甲", "terminal"}, {"乙", "feishu:x"}} {
			has, _ := d.MemoryHas(ctx, "decision", tt.title)
			if has {
				continue
			}
			d.PutMemory(ctx, "decision", tt.title, []byte("body"), "legacy-import", tt.prov)
			n++
		}
		return n
	}
	if n := parse(); n != 2 {
		t.Fatalf("首跑应入 2 行: %d", n)
	}
	if n := parse(); n != 0 {
		t.Fatalf("二跑应零增: %d", n)
	}
	_ = md
}

// TestCeoGap 锚8 D-22：CEO run 处理过事件零写→ceo_gap 行。
func TestCeoGap(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()
	c := ceoClient(t, dir)
	run := mkCeoRun(t, g, "terminal")

	// 订阅 follow→喂 1 条 spawn_kind 事件（随流派发计入 ceoDispatched）。
	es, err := c.Events(ctx, &pbv1.Subscribe{RunToken: run.RunToken, Follow: true})
	if err != nil {
		t.Fatal(err)
	}
	waitFor2(t, 3*time.Second, func() bool { return g.ceoActiveSub() != nil })
	g.emit(Event{Kind: "reject_limit", TicketID: "tg"})
	if _, err := es.Recv(); err != nil {
		t.Fatal(err)
	}
	es.CloseSend()
	// token run（pid 桩 1 但 AttachedAt 零值？——mkCeoRun 给 PID=1，模拟有进程
	// run；D-22 判死径走 ceoGapCheck，直接调用模拟判死）。
	g.ceoGapCheck(ctx, run.RunID)
	waitFor2(t, 3*time.Second, func() bool {
		evs, _ := g.DB().Events(ctx, "")
		for _, e := range evs {
			if e.Kind == "ceo_gap" {
				return true
			}
		}
		return false
	})
}

// TestAdminGate 锚9+对抗锚④：施工席发 admin→断流；adjunct CTO→拒；
// fill_spec 越票→拒；合法 CTO fill_spec→落。
func TestAdminGate(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()

	// 施工席发 admin→PermissionDenied 断流。
	mkTicketAt(t, g, "tw", db.PhaseAssigned)
	wrun := spawnSeat(t, g, "tw", "施工", "idle")
	_, sc := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	s, _ := sc.Attach(ctx)
	s.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: wrun.RunToken}}})
	s.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Admin{Admin: &pbv1.SeatAdmin{
		Action: &pbv1.SeatAdmin_CreateTicket{CreateTicket: &pbv1.NewTicket{Id: "hack"}}}}})
	if _, err := s.Recv(); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("施工席 admin 未断流: %v", err)
	}
	s.CloseSend()

	// adjunct CTO 发→拒。
	arun := &db.Run{RunID: randHex(8), TicketID: "tw", Role: "CTO",
		RunToken: randHex(16), Adjunct: true,
		SocketPath: filepath.Join(dir, "run", "seats", "a.sock")}
	if err := g.DB().CreateRun(ctx, arun); err != nil {
		t.Fatal(err)
	}
	s2, _ := sc.Attach(ctx)
	s2.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: arun.RunToken}}})
	s2.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Admin{Admin: &pbv1.SeatAdmin{
		Action: &pbv1.SeatAdmin_CreateTicket{CreateTicket: &pbv1.NewTicket{Id: "hack"}}}}})
	if _, err := s2.Recv(); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("adjunct CTO admin 未拒: %v", err)
	}
	s2.CloseSend()

	// 合法 CTO run：fill_spec 越票→拒；本票→落。
	crun := &db.Run{RunID: randHex(8), TicketID: "tw", Role: "CTO",
		RunToken: randHex(16), SocketPath: filepath.Join(dir, "run", "seats", "c.sock"),
		Provenance: "feishu:o"}
	if err := g.DB().CreateRun(ctx, crun); err != nil {
		t.Fatal(err)
	}
	s3, _ := sc.Attach(ctx)
	s3.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: crun.RunToken}}})
	s3.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: "tw"}}})
	waitPhase(t, g, "tw", db.PhaseWorking, 5*time.Second)
	s3.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Admin{Admin: &pbv1.SeatAdmin{
		Action: &pbv1.SeatAdmin_FillSpec{FillSpec: &pbv1.FillSpec{
			TicketId: "other-ticket", SpecMd: []byte("x")}}}}})
	m, err := s3.Recv()
	if err != nil || m.GetAdminResp().GetOk() {
		t.Fatalf("fill_spec 越票未拒: %v %+v", err, m)
	}
	md := "## 1. 目标\n\nx\n## 2. 为什么\n\nx\n## 3. 范围内\n\nx\n## 4. 范围外\n\nx\n## 5. 设计引用\n\nx\n## 6. 验收锚\n\nx\n## 7. 验证命令\n\nx\n## 8. 交付形制\n\nx\n"
	s3.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Admin{Admin: &pbv1.SeatAdmin{
		Action: &pbv1.SeatAdmin_FillSpec{FillSpec: &pbv1.FillSpec{
			TicketId: "tw", SpecMd: []byte(md)}}}}})
	m, err = s3.Recv()
	if err != nil || !m.GetAdminResp().GetOk() {
		t.Fatalf("fill_spec 本票未落: %v %+v", err, m)
	}
	tk, _ := g.DB().GetTicket(ctx, "tw")
	var ts pbv1.TicketSpec
	if err := proto.Unmarshal(tk.Spec, &ts); err != nil || ts.Goal == "" {
		t.Fatal("spec 未回填")
	}
	// admin create 子票→created 行同戳（provenance 继承）。
	s3.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Admin{Admin: &pbv1.SeatAdmin{
		Action: &pbv1.SeatAdmin_CreateTicket{CreateTicket: &pbv1.NewTicket{
			Id: "child-1", Title: "子票"}}}}})
	m, err = s3.Recv()
	if err != nil || !m.GetAdminResp().GetOk() {
		t.Fatalf("create_ticket 未成: %v %+v", err, m)
	}
	evs, _ := g.DB().Events(ctx, "child-1")
	if len(evs) == 0 || evs[0].Provenance != "feishu:o" {
		t.Fatalf("子票 created 未承父戳: %+v", evs)
	}
	s3.CloseSend()
}

// TestAdmin_DispatchCEO硬拒 admin dispatch role=CEO 经闸也拒。
func TestAdmin_DispatchCEO硬拒(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()
	mkTicketAt(t, g, "td", db.PhaseAssigned)
	crun := &db.Run{RunID: randHex(8), TicketID: "td", Role: "CTO",
		RunToken: randHex(16), SocketPath: filepath.Join(dir, "run", "seats", "c.sock")}
	if err := g.DB().CreateRun(ctx, crun); err != nil {
		t.Fatal(err)
	}
	_, sc := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	s, _ := sc.Attach(ctx)
	s.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: crun.RunToken}}})
	s.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Admin{Admin: &pbv1.SeatAdmin{
		Action: &pbv1.SeatAdmin_Dispatch{Dispatch: &pbv1.DispatchReq{
			TicketId: "td", Role: "CEO"}}}}})
	m, err := s.Recv()
	if err != nil || m.GetAdminResp().GetOk() {
		t.Fatalf("admin dispatch CEO 未拒: %v %+v", err, m)
	}
	s.CloseSend()
	_ = fmt.Sprint // silence
}

// TestD36_BudgetErrReturn D-36 变异真跑：帽覆盖处 GetBudget 注 err→return
// （旧码静默落 0——SetBudgetCaps 会拿零值 HardCap 覆写）。
func TestD36_BudgetErrReturn(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	ctx := context.Background()
	mkTicketAt(t, g, "td36", db.PhaseCreated)
	if err := g.DB().SetBudgetCaps(ctx, "td36", 100, 999); err != nil {
		t.Fatal(err)
	}
	// 注错缝：budgetOf 恒 err——旧码 b 落零值后 SetBudgetCaps 把 hard_cap 盖 0。
	g.budgetOf = func(context.Context, string) (db.Budget, error) {
		return db.Budget{}, fmt.Errorf("注入库错")
	}
	g.dispatchOnce(spawnReq{ticketID: "td36", role: "施工"})
	b, _ := g.DB().GetBudget(ctx, "td36")
	if b.HardCap != 999 {
		t.Fatalf("GetBudget err 未 return——hard_cap 被盖: %d", b.HardCap)
	}
}

// TestD15_TerminalCleanup 终态即删 specs/failSet/warned/frozen＋cancelAlarm。
func TestD15_TerminalCleanup(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()
	mkTicketAt(t, g, "tdone", db.PhaseAuditing)
	g.mu.Lock()
	g.specs[specKey{"tdone", "施工"}] = SeatSpec{}
	g.failSet["tdone"] = map[string]bool{"x": true}
	g.warned["tdone"] = true
	g.frozen["tdone"] = true
	g.mu.Unlock()
	g.armAlarm("tdone", time.Now().Add(time.Hour))

	if err := g.transition(ctx, "tdone", db.PhaseVoid, db.EdgeVoid, "test", "终"); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.specs) != 0 || g.failSet["tdone"] != nil || g.warned["tdone"] || g.frozen["tdone"] {
		t.Fatal("终态内存键未清")
	}
	if _, ok := g.alarms["tdone"]; ok {
		t.Fatal("终态闹钟未消")
	}
}

// TestD34_Order 债 D-34 回归：budget_frozen/budget_stop 先落行后 emit——
// emit 门铃不可先于持久行（事件消费方以 ticket_event 为真源回读）。
func TestD34_Order(t *testing.T) {
	data, err := os.ReadFile("attach.go")
	if err != nil {
		t.Skip("attach.go 不可读")
	}
	src := string(data)
	for _, kind := range []string{"budget_frozen", "budget_stop"} {
		aIdx := strings.Index(src, `"`+kind+`", db.Event{`)
		eIdx := strings.Index(src, `Event{Kind: "`+kind+`"`)
		if aIdx < 0 || eIdx < 0 || aIdx > eIdx {
			t.Fatalf("%s: AppendEvent 未先于 emit", kind)
		}
	}
}

// TestAudit_RedispatchReject 债 D-38 缺口一（审计中再派拒断言）：
// 审计中票派审计/施工皆拒——审计途中双席并存洞由相位闸焊死。
func TestAudit_RedispatchReject(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) { *c = dispatchCfg(filepath.Dir(c.DBPath)) })
	ctx := context.Background()
	mkTicketAt(t, g, "ta", db.PhaseAuditing)
	if err := g.DispatchSeat(ctx, "ta", "审计", dispatchSpec()); err == nil {
		t.Fatal("审计中再派审计未拒")
	}
	if err := g.DispatchSeat(ctx, "ta", "施工", dispatchSpec()); err == nil {
		t.Fatal("审计中再派施工未拒")
	}
}
