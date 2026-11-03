package gateway

import (
	"context"
	"testing"
	"time"

	"meta/internal/db"
)

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
