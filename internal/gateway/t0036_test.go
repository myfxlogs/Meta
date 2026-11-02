package gateway

import (
	"context"
	"testing"
	"time"

	"meta/internal/db"
)

// TestT36_CardViewPhaseSince 票0036 锚8：cardView 装配断言——施工中票的
// PhaseSince＝最新 to_phase=施工中 事件 created_at（无关 progress 帧带不跑）、
// RenderAt 非零（耗时唯一时间源）、CreatedAt＝票行 created_at。
// 构造全经代码径＋SetClock，无 INSERT（D-56）。
func TestT36_CardViewPhaseSince(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	d := g.DB()
	ctx := context.Background()
	base := time.Unix(1762000000, 0)
	now := base
	d.SetClock(func() time.Time { return now })

	if err := d.CreateTicket(ctx, "t36", "t36 票", nil); err != nil {
		t.Fatal(err)
	}
	step := func(to db.Phase, kind db.EdgeKind, at time.Duration) {
		now = base.Add(at)
		if _, err := d.Transition(ctx, "t36", to, kind, "test", ""); err != nil {
			t.Fatalf("→%s: %v", to, err)
		}
	}
	step(db.PhaseDesigning, db.EdgeDesignStart, time.Minute)
	step(db.PhaseAssigned, db.EdgeDispatch, 2*time.Minute)
	step(db.PhaseWorking, db.EdgeClaim, 5*time.Minute)
	enter := base.Add(5 * time.Minute) // 施工中进入事件 created_at

	// 无关 progress 帧推进时钟——PhaseSince 不应被带跑（取错源者红）。
	now = base.Add(7 * time.Minute)
	if err := d.AppendEvent(ctx, "t36", "progress",
		db.Event{Actor: "r1", Note: "写码 落表"}); err != nil {
		t.Fatal(err)
	}

	v, err := g.cardView(ctx, "t36")
	if err != nil || v == nil {
		t.Fatalf("cardView: v=%v err=%v", v, err)
	}
	if !v.PhaseSince.Equal(enter) {
		t.Fatalf("PhaseSince=%v want=%v（最新 to_phase=施工中 事件）", v.PhaseSince, enter)
	}
	if v.RenderAt.IsZero() {
		t.Fatal("RenderAt 零值——耗时无法计算")
	}
	tk, err := d.GetTicket(ctx, "t36")
	if err != nil || tk == nil {
		t.Fatalf("GetTicket: %v %v", tk, err)
	}
	if !v.CreatedAt.Equal(tk.CreatedAt) || !v.CreatedAt.Equal(base) {
		t.Fatalf("CreatedAt=%v want=票行 created_at %v", v.CreatedAt, tk.CreatedAt)
	}
}
