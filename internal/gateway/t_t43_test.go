package gateway

// 票 T43 锚测：resume/ruling 人工干预＝活性戳刷新——新席不背过期死线。
// 0050 实证：resume→新席上岗→9 秒 stall_kill（死线仍算历史活性戳）。

import (
	"context"
	"testing"
	"time"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// 锚：活性戳过期→resume→LastActivityAt 刷新到放行点→调和器不判停滞。
func TestT43_ResumeRefreshesActivity(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		c.ReconcileDebounce = 30 * time.Second
		c.ProgressTimeout = 500 * time.Millisecond // 短窗验死线
	})
	ctx := context.Background()
	// 活性戳整体回拨（claim 边本就在活性白名单——须连建票链一起回拨）。
	past := time.Now().Add(-time.Hour)
	g.DB().SetClock(func() time.Time { return past })
	mkTicketAt(t, g, "tr44", db.PhaseWorking)
	_ = g.DB().AppendEvent(ctx, "tr44", "progress", db.Event{Actor: "seat"})
	g.DB().SetClock(time.Now)
	la, _ := g.DB().LastActivityAt(ctx, "tr44")
	if time.Since(la) < time.Hour/2 {
		t.Fatal("夹具未过期")
	}
	// blocked→resume：放行行＝活性起点——新席死线须从放行起算。
	if _, err := g.DB().Transition(ctx, "tr44", db.PhaseBlocked, db.EdgeBlocked, "gateway", "x"); err != nil {
		t.Fatal(err)
	}
	if ack, _ := g.Resume(ctx, &pbv1.TicketRef{Id: "tr44"}); ack == nil || !ack.Ok {
		t.Fatalf("Resume 未放行: %+v", ack)
	}
	la2, _ := g.DB().LastActivityAt(ctx, "tr44")
	if time.Since(la2) > 10*time.Second {
		t.Fatalf("resume 未刷新活性戳: since=%v", time.Since(la2))
	}
	// 新席活过停滞窗：调和器不得把放行后新席判 stall_suspect。
	spawnSeat(t, g, "tr44", "施工", "idle")
	g.reconcile(ctx)
	evs, _ := g.DB().Events(ctx, "tr44")
	for _, e := range evs {
		if e.Kind == "stall_suspect" && e.CreatedAt.After(time.Now().Add(-time.Minute)) {
			t.Fatalf("放行后席被误杀: %v", e.Note)
		}
	}
}
