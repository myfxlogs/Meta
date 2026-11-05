package gateway

// 票 T46 锚测：resume 瞬回角——复位前死亡账（死行凭/竞态落账事件）
// 不重启观察窗；seat_dead 须期窗内新死才计数。

import (
	"context"
	"testing"
	"time"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// 锚：cap=1 seat_dead 超限→blocked→resume→复位前死账清零→零瞬回。
func TestT46_ResumeNoInstantReblock(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		c.ReconcileDebounce = 30 * time.Second
		c.FailureRules = []FailureRule{
			{Kind: "seat_dead", Action: "next_shell", Cap: 1},
			{Kind: "*", Action: "blocked", Cap: 0},
		}
	})
	ctx := context.Background()
	mkTicketAt(t, g, "tr46", db.PhaseWorking)
	// 复位前死亡：clock 回拨标死——ExitedAt 早于 resume 锚点（实证形态：
	// seat_dead 事件竞态落账在放行后，死亡事实在复位前）。
	past := time.Now().Add(-time.Hour)
	g.DB().SetClock(func() time.Time { return past })
	run := spawnSeat(t, g, "tr46", "施工", "exitcode")
	waitFor2(t, 5*time.Second, func() bool { return !processAlive(run.PID) }, g)
	_ = g.DB().MarkDead(ctx, run.RunID)
	_ = g.DB().AppendEvent(ctx, "tr46", "seat_dead", db.Event{Actor: "gateway"})
	_ = g.DB().AppendEvent(ctx, "tr46", "seat_dead", db.Event{Actor: "gateway"})
	_, _ = g.DB().Transition(ctx, "tr46", db.PhaseBlocked, db.EdgeBlocked, "gateway", "cap 超限")
	g.DB().SetClock(time.Now)
	// 放行→调和——复位前死账不得瞬回（T46 铁证：三秒回 blocked）。
	if ack, _ := g.Resume(ctx, &pbv1.TicketRef{Id: "tr46"}); ack == nil || !ack.Ok {
		t.Fatalf("Resume 未放行: %+v", ack)
	}
	g.reconcile(ctx)
	if p, _ := g.DB().PhaseOf(ctx, "tr46"); p == db.PhaseBlocked {
		t.Fatal("resume 后瞬回 blocked——复位前死账未清零")
	}
}
