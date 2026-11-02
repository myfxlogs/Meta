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
	g.specs[specKey{"t19", "施工"}] = SeatSpec{}
	g.failSet["t19"] = map[string]bool{"devin": true}
	g.warned["t19"] = true
	g.frozen["t19"] = true
	g.mu.Unlock()

	if err := g.transition(ctx, "t19", db.PhaseVoid, db.EdgeVoid,
		"t", "作废终态清理锚"); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.specs) != 0 || len(g.failSet) != 0 || len(g.warned) != 0 || len(g.frozen) != 0 {
		t.Fatalf("终态票内存键未清: specs=%d failSet=%d warned=%d frozen=%d",
			len(g.specs), len(g.failSet), len(g.warned), len(g.frozen))
	}
}
