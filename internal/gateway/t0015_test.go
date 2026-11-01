package gateway

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// ---------- 票0015：票→项目挂接线验收锚 ----------

// projectCount 票上 project_set 成功行（denied 前缀不算）计数——
// 幂等断言「无重复事件」用。
func projectSetCount(t *testing.T, g *Gateway, tid string) (okRows, deniedRows int) {
	t.Helper()
	evs, err := g.DB().Events(context.Background(), tid)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range evs {
		if e.Kind != "project_set" {
			continue
		}
		if strings.HasPrefix(e.Note, "denied:") {
			deniedRows++
		} else {
			okRows++
		}
	}
	return okRows, deniedRows
}

// ctoSeat 在本票上立合法 CTO run 并 attach+claim（票相位→施工中）。
// 返回 SeatAdmin 帧的发送面（test 代驾，同 TestAdminGate 形制）。
func ctoSeat(t *testing.T, g *Gateway, dir, tid string) pbv1.SeatGateway_AttachClient {
	t.Helper()
	ctx := context.Background()
	run := &db.Run{RunID: randHex(8), TicketID: tid, Role: "CTO",
		RunToken: randHex(16), SocketPath: filepath.Join(dir, "run", "seats", "c.sock"),
		Provenance: "test:cto"}
	if err := g.DB().CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	_, sc := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	s, err := sc.Attach(ctx)
	if err != nil {
		t.Fatal(err)
	}
	send := func(m *pbv1.SeatMsg) {
		if err := s.Send(m); err != nil {
			t.Fatal(err)
		}
	}
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: run.RunToken}}})
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: tid}}})
	waitPhase(t, g, tid, db.PhaseWorking, 5*time.Second)
	return s
}

func adminResp(t *testing.T, s pbv1.SeatGateway_AttachClient) *pbv1.Ack {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		m, err := s.Recv()
		if err != nil {
			t.Fatalf("等 admin_resp 断流: %v", err)
		}
		if a := m.GetAdminResp(); a != nil {
			return a
		}
	}
	t.Fatal("等 admin_resp 超时")
	return nil
}

// TestAttach_CreateTicket 锚3＋对抗锚 c：NewTicket{project:未登记}→CreateTicket
// 拒且票不立；{project:已登记}→ticket.project 原子落入＋project_set 留痕。
func TestAttach_CreateTicket(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	ack, err := g.CreateTicket(ctx, &pbv1.NewTicket{Id: "tu", Title: "x", Project: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if ack.GetOk() {
		t.Fatal("未登记项目应拒（对抗锚 c：删登记校验则此立票成功）")
	}
	if tk, _ := g.DB().GetTicket(ctx, "tu"); tk != nil {
		t.Fatal("未登记拒后票不应立")
	}
	ack, err = g.CreateTicket(ctx, &pbv1.NewTicket{Id: "tok", Title: "x", Project: "P"})
	if err != nil || !ack.GetOk() {
		t.Fatalf("已登记立票应放: %v %+v", err, ack)
	}
	tk, _ := g.DB().GetTicket(ctx, "tok")
	if tk == nil || tk.Project != "P" {
		t.Fatalf("ticket.project 应=P: %+v", tk)
	}
	if ok, _ := projectSetCount(t, g, "tok"); ok != 1 {
		t.Fatalf("立票 project_set 留痕应 1 行: %d", ok)
	}
}

// TestAttach_Dispatch 锚4＋对抗锚 b：DispatchReq{project:P}→派发后
// ticket.project==P＋project_set 事件（actor/prov 断言）；挂接在取锁闸之前——
// 派 待部署 CTO 票后 projectLocks[P] 占用而非 "" 键。
func TestAttach_Dispatch(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.SudoBin = sudoStub(t, d, 0)
	})
	_ = dir
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	// RPC 径：已立票 Dispatch{project:P}→attachProject→相位走边→已派。
	mkTicketAt(t, g, "tdx", db.PhaseCreated)
	ack, err := g.Dispatch(ctx, &pbv1.DispatchReq{
		TicketId: "tdx", Role: "施工", Project: "P", Provenance: "test:disp"})
	if err != nil || !ack.GetOk() {
		t.Fatalf("dispatch 应放: %v %+v", err, ack)
	}
	tk, _ := g.DB().GetTicket(ctx, "tdx")
	if tk.Project != "P" {
		t.Fatalf("派发后 ticket.project 应=P: %q", tk.Project)
	}
	var ev *db.Event
	evs, _ := g.DB().Events(ctx, "tdx")
	for _, e := range evs {
		if e.Kind == "project_set" {
			ev = e
		}
	}
	if ev == nil || ev.Actor != "gateway" || ev.Provenance != "test:disp" {
		t.Fatalf("project_set 事件 actor/prov 不符: %+v", ev)
	}
	// 待部署 CTO 票：挂接先于取锁——projectLocks[P] 占 integrate+deploy，"" 键空。
	mkToDeploy(t, g, "tdep", "")
	spec := deploySpec(t)
	spec.Project = "P"
	if err := g.DispatchSeat(ctx, "tdep", "CTO", spec); err != nil {
		t.Fatalf("待部署派 CTO 应放: %v", err)
	}
	g.mu.Lock()
	held := g.projectLocks["P"]
	emptyBucket := g.projectLocks[""]
	g.mu.Unlock()
	if !held["integrate"] || !held["deploy"] {
		t.Fatalf("projectLocks[P] 应占 integrate+deploy（对抗锚 b 提名行）: %+v", held)
	}
	if len(emptyBucket) != 0 {
		t.Fatalf("锁应按新键 P 取——\"\" 键桶应空: %+v", emptyBucket)
	}
	// 撞闸：同票换挂 Y→dispatch 拒（attachProject 第 5 判）票停发原位。
	mkTicketAt(t, g, "tdy", db.PhaseCreated)
	if err := g.DB().SetTicketProject(ctx, "tdy", "P"); err != nil {
		t.Fatal(err)
	}
	ack, err = g.Dispatch(ctx, &pbv1.DispatchReq{TicketId: "tdy", Role: "施工", Project: "P"})
	if err != nil || !ack.GetOk() {
		t.Fatalf("同值幂等派发应放: %v %+v", err, ack)
	}
	if ok, denied := projectSetCount(t, g, "tdy"); ok != 0 || denied != 0 {
		t.Fatalf("幂等挂接应零事件行: ok=%d denied=%d", ok, denied)
	}
}

// TestAttach_SeatAdmin 锚5/6＋对抗锚 d：席内 create_ticket 空→继承本票
// project、显式→落显式值、未登记→admin_resp false 票不立；set_project
// 越票→admin_resp false（事件无写）、本票→落＋project_set 事件。
func TestAttach_SeatAdmin(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()
	for _, p := range []string{"P", "Q"} {
		if err := g.DB().UpsertProject(ctx, stubProject(p)); err != nil {
			t.Fatal(err)
		}
	}
	mkTicketAt(t, g, "tw", db.PhaseAssigned)
	s := ctoSeat(t, g, dir, "tw")
	send := func(a *pbv1.SeatAdmin) {
		if err := s.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Admin{Admin: a}}); err != nil {
			t.Fatal(err)
		}
	}
	// 锚6：set_project 越票→admin_resp false（事件无写）。
	send(&pbv1.SeatAdmin{Action: &pbv1.SeatAdmin_SetProject{SetProject: &pbv1.SetProject{
		TicketId: "other-ticket", Project: "P"}}})
	if a := adminResp(t, s); a.GetOk() {
		t.Fatal("set_project 越票未拒（对抗锚 d：删越票判则他票被改写）")
	}
	if tk, _ := g.DB().GetTicket(ctx, "other-ticket"); tk != nil {
		t.Fatal("越票票据以存在")
	}
	// 锚6：本票→落＋project_set 事件。
	send(&pbv1.SeatAdmin{Action: &pbv1.SeatAdmin_SetProject{SetProject: &pbv1.SetProject{
		TicketId: "tw", Project: "P"}}})
	if a := adminResp(t, s); !a.GetOk() {
		t.Fatalf("set_project 本票应落: %+v", a)
	}
	tk, _ := g.DB().GetTicket(ctx, "tw")
	if tk.Project != "P" {
		t.Fatalf("本票 project 应=P: %q", tk.Project)
	}
	var psEv *db.Event
	evs, _ := g.DB().Events(ctx, "tw")
	for _, e := range evs {
		if e.Kind == "project_set" {
			psEv = e
		}
	}
	if psEv == nil || psEv.Actor == "" {
		t.Fatalf("set_project 缺 project_set 事件: %+v", evs)
	}
	// 锚5：create_ticket 空→子票继承本票 project=P。
	send(&pbv1.SeatAdmin{Action: &pbv1.SeatAdmin_CreateTicket{CreateTicket: &pbv1.NewTicket{
		Id: "child-inherit", Title: "子"}}})
	if a := adminResp(t, s); !a.GetOk() {
		t.Fatalf("create_ticket 继承应立: %+v", a)
	}
	if tk, _ := g.DB().GetTicket(ctx, "child-inherit"); tk == nil || tk.Project != "P" {
		t.Fatalf("子票应继承本票 P: %+v", tk)
	}
	// 锚5：显式 Q→落 Q。
	send(&pbv1.SeatAdmin{Action: &pbv1.SeatAdmin_CreateTicket{CreateTicket: &pbv1.NewTicket{
		Id: "child-exp", Title: "子", Project: "Q"}}})
	if a := adminResp(t, s); !a.GetOk() {
		t.Fatalf("create_ticket 显式应立: %+v", a)
	}
	if tk, _ := g.DB().GetTicket(ctx, "child-exp"); tk == nil || tk.Project != "Q" {
		t.Fatalf("子票应落显式 Q: %+v", tk)
	}
	// 锚5：未登记→admin_resp false 票不立。
	send(&pbv1.SeatAdmin{Action: &pbv1.SeatAdmin_CreateTicket{CreateTicket: &pbv1.NewTicket{
		Id: "child-bad", Title: "子", Project: "nope"}}})
	if a := adminResp(t, s); a.GetOk() {
		t.Fatal("create_ticket 未登记未拒")
	}
	if tk, _ := g.DB().GetTicket(ctx, "child-bad"); tk != nil {
		t.Fatal("未登记子票不应立")
	}
	s.CloseSend()
}

// TestAttach_SetTicketProjectRPC 锚7＋对抗锚 a：五判全验——未登记拒／
// 部署中相位拒／终态拒／X→Y 换挂拒／同值幂等 ok 且无重复事件。
func TestAttach_SetTicketProjectRPC(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.SudoBin = sudoStub(t, d, 0)
	})
	ctx := context.Background()
	for _, p := range []string{"P", "Q"} {
		if err := g.DB().UpsertProject(ctx, stubProject(p)); err != nil {
			t.Fatal(err)
		}
	}
	call := func(tid, proj string) *pbv1.Ack {
		a, err := g.SetTicketProject(ctx, &pbv1.TicketProjectReq{TicketId: tid, Project: proj})
		if err != nil {
			t.Fatalf("RPC 错: %v", err)
		}
		return a
	}
	mkTicketAt(t, g, "tp", db.PhaseCreated)
	if a := call("tp", "nope"); a.GetOk() {
		t.Fatal("未登记应拒")
	}
	if tk, _ := g.DB().GetTicket(ctx, "tp"); tk.Project != "" {
		t.Fatal("未登记拒后 project 应保持空")
	}
	if a := call("tp", "P"); !a.GetOk() {
		t.Fatalf("空→P 应放: %+v", a)
	}
	if tk, _ := g.DB().GetTicket(ctx, "tp"); tk.Project != "P" {
		t.Fatal("project 未落")
	}
	// 同值幂等：ok 且无重复事件。
	if a := call("tp", "P"); !a.GetOk() {
		t.Fatal("同值幂等应 ok")
	}
	if ok, _ := projectSetCount(t, g, "tp"); ok != 1 {
		t.Fatalf("幂等后 project_set 成功行应仍 1: %d", ok)
	}
	// X→Y 换挂拒（对抗锚 a：判等翻则挂上）。
	if a := call("tp", "Q"); a.GetOk() {
		t.Fatal("X→Y 换挂应拒（对抗锚 a）")
	}
	if tk, _ := g.DB().GetTicket(ctx, "tp"); tk.Project != "P" {
		t.Fatal("换挂拒后 project 应仍=P")
	}
	// 部署中相位拒。
	mkTicketAt(t, g, "tdep", db.PhaseDeploying)
	if a := call("tdep", "P"); a.GetOk() {
		t.Fatal("部署中相位应拒")
	}
	// 终态拒。
	mkTicketAt(t, g, "tdone", db.PhaseDone)
	if a := call("tdone", "P"); a.GetOk() {
		t.Fatal("终态票应拒")
	}
	// 票不存在拒。
	if a := call("ghost", "P"); a.GetOk() {
		t.Fatal("不存在票应拒")
	}
}

// TestAttach_Requirement 锚8：Requirement{project:P}→需求票落地
// ticket.project==P；未登记→需求不立（RPC 层拒）。
func TestAttach_Requirement(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) { *c = ceoTestCfg(filepath.Dir(c.DBPath), true) })
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	run := mkCeoRun(t, g, "terminal")
	if _, err := g.CeoSubmitRequirement(ctx, &pbv1.Requirement{
		RunToken: run.RunToken, Goal: "坏需求", Project: "nope"}); err == nil {
		t.Fatal("未登记项目需求应拒")
	}
	ref, err := g.CeoSubmitRequirement(ctx, &pbv1.Requirement{
		RunToken: run.RunToken, Goal: "挂项目需求", Project: "P"})
	if err != nil {
		t.Fatal(err)
	}
	tk, _ := g.DB().GetTicket(ctx, ref.Id)
	if tk == nil || tk.Project != "P" {
		t.Fatalf("需求票 project 应=P: %+v", tk)
	}
	if ok, _ := projectSetCount(t, g, ref.Id); ok != 1 {
		t.Fatalf("需求票 project_set 留痕应 1 行: %d", ok)
	}
}

// TestAttach_ObserveEmit 锚10：project_set 事件行进 ticket_event 且 emit 出
// （观察者断言）。
func TestAttach_ObserveEmit(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkTicketAt(t, g, "tobs", db.PhaseCreated)
	if _, err := g.SetTicketProject(ctx, &pbv1.TicketProjectReq{TicketId: "tobs", Project: "P"}); err != nil {
		t.Fatal(err)
	}
	e := waitEvent(t, g, "project_set", "tobs", 3*time.Second)
	if e.Note == "" || !strings.Contains(e.Note, "project=P") {
		t.Fatalf("emit 帧 note 不符: %+v", e)
	}
	if ok, _ := projectSetCount(t, g, "tobs"); ok != 1 {
		t.Fatal("ticket_event 无 project_set 行")
	}
}
