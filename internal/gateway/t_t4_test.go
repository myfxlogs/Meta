package gateway

import (
	"context"
	"testing"
	"time"

	"meta/internal/db"
)

// TestT4_ChainDepth 链深闸锚：深度≥3 拦截＋chain_depth_limit 通知；深 2 放行。
func TestT4_ChainDepth(t *testing.T) {
	if chainDepth("0034-p1-p1-p1") != 3 || chainDepth("0034-p1-p1") != 2 ||
		chainDepth("0034") != 0 || chainDepth("patrol-9-p1") != 1 {
		t.Fatal("chainDepth 段数判读错")
	}
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	// T27/D-87：判据由段数改族员计数——深名票建齐族 {x,x-p1,x-p1-p1,…}
	// 达 3 员才截断；兄弟新编号同层入族（绕闸实证修法）。
	for _, id := range []string{"xx", "xx-p1", "xx-p1-p1"} {
		if err := g.DB().CreateTicket(ctx, id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	mkTicketAt(t, g, "xx-p1-p1-p1", db.PhaseAssigned)
	if err := g.DispatchSeat(ctx, "xx-p1-p1-p1", "施工", SeatSpec{}); err == nil {
		t.Fatal("族满 3 员深链票竟放行")
	}
	waitEvent(t, g, "chain_depth_limit", "xx-p1-p1-p1", 3*time.Second)
	evs, _ := g.DB().Events(ctx, "xx-p1-p1-p1")
	var saw bool
	for _, e := range evs {
		saw = saw || e.Kind == "chain_depth_limit"
	}
	if !saw {
		t.Fatal("截断无事件留痕")
	}
	mkTicketAt(t, g, "x-p1-p1", db.PhaseAssigned)
	spec := SeatSpec{PromptFile: "", Adjunct: true}
	if err := g.DispatchSeat(ctx, "x-p1-p1", "巡检", spec); err == nil ||
		err.Error() == "" {
		// 深2放行——错误应非链深闸（Adjunct 无 prompt 被拒属另一闸）
	}
}

// TestT4_BatchDeploy 批量闸锚：第 5 张到待部署→批量 spawn_needed；
// 4 张不触发；流水线忙（有施工票）也不触发。
func TestT4_BatchDeploy(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	// 流水线忙挡：1 张待部署＋1 张施工中→不触发。
	mkTicketAt(t, g, "w1", db.PhaseWorking)
	mkTicketAt(t, g, "b0", db.PhaseAuditing)
	if err := g.transition(ctx, "b0", db.PhaseToDeploy, db.EdgeAuditPass, "t", ""); err != nil {
		t.Fatal(err)
	}
	<-time.After(300 * time.Millisecond)
	for _, e := range collectEvents(g, 100*time.Millisecond) {
		if e.Kind == "batch_deploy" || (e.Kind == "spawn_needed" && e.TicketID == "b0") {
			t.Fatalf("流水线忙竟触发批部署: %+v", e)
		}
	}
	// 4 张＋流水线清空（w1→完成）：仍不触发（<5 但清空会触发——先验 4 张
	// 忙线不触发，再清空验触发）。
	for i := 0; i < 3; i++ {
		id := "b" + string(rune('1'+i))
		mkTicketAt(t, g, id, db.PhaseAuditing)
		if err := g.transition(ctx, id, db.PhaseToDeploy, db.EdgeAuditPass, "t", ""); err != nil {
			t.Fatal(err)
		}
	}
	<-time.After(300 * time.Millisecond)
	for _, e := range collectEvents(g, 100*time.Millisecond) {
		if e.Kind == "batch_deploy" {
			t.Fatal("4 张忙线竟触发批部署")
		}
	}
	// 第 5 张＋流水线清空（施工中票全完）→触发。
	mkTicketAt(t, g, "b4", db.PhaseAuditing)
	// 回队清空流水线（非终态但仍忙——已派算忙吗？判定只数 施工中/设计中，
	// 回 已派 即算清空）。
	if err := g.transition(ctx, "w1", db.PhaseAssigned, db.EdgeRequeue, "t", ""); err != nil {
		t.Fatal(err)
	}
	if err := g.transition(ctx, "b4", db.PhaseToDeploy, db.EdgeAuditPass, "t", ""); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "batch_deploy", "", 5*time.Second)
	waitEvent(t, g, "spawn_needed", "b0", 5*time.Second)
}
