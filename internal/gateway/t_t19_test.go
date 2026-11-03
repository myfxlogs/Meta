package gateway

import (
	"context"
	"testing"

	"meta/internal/db"
)

// TestT19_终态清理 锚（D-15 实证钉）：specs/failSet/warned/frozen 四表
// 在票入终态即清——进程级内存泄涨闸。
func TestT19_终态清理(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "t19", db.PhaseWorking)

	g.mu.Lock()
	g.runWorkDir["rt19"] = "/tmp/x"
	g.mu.Unlock()
	run := &db.Run{RunID: "rt19", TicketID: "t19", Role: "施工"}
	if err := g.DB().CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}

	if err := g.transition(ctx, "t19", db.PhaseVoid, db.EdgeVoid,
		"t", "作废终态清理锚"); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.runWorkDir["rt19"]; ok {
		t.Fatal("终态票 runWorkDir 索引未清")
	}
}
