package gateway

// 票 T37 锚测：交付后退场非暴毙（0050 实证）——同票最近已 deliver 的
// 席死＝功成身退不入失败账；未交付而死照旧计。

import (
	"context"
	"syscall"
	"testing"
	"time"
	"path/filepath"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// 锚1：deliver→席退场→票进 待审 零 blocked（不判失败表）。
func TestT37_DeliverThenExitNotFailure(t *testing.T) {
	// 调和重派席须能立住——helper env 沿 os.Environ 入新席。
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "idle")
	g, dir := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.InjectRoot = injectFixture(t)
		c.ReconcileDebounce = 30 * time.Second // 本测手工 kick
	})
	ctx := context.Background()
	mkTicketAt(t, g, "tx1", db.PhaseAssigned)
	_ = g.DB().UpdateSpec(ctx, "tx1", testSpec(t))
	run := spawnSeat(t, g, "tx1", "施工", "idle")
	d := driveSeat(t, g, dir, run)
	d.claim(t, "tx1")
	waitPhase(t, g, "tx1", db.PhaseWorking, 10*time.Second)
	d.send(t, &pbv1.SeatMsg{Body: &pbv1.SeatMsg_Deliver{Deliver: &pbv1.Deliver{
		Note: "交活", Ref: "r1"}}})
	waitPhase(t, g, "tx1", db.PhaseReview, 10*time.Second)
	// 功成身退：关流＋退场（双源互证 EOF+exit）。
	d.stream.CloseSend()
	_ = syscall.Kill(-run.PID, syscall.SIGKILL)
	// 调和若干轮——seat_dead 不入失败账：零 blocked、零施工重派。
	time.Sleep(200 * time.Millisecond)
	g.reconcile(ctx)
	time.Sleep(500 * time.Millisecond)
	g.reconcile(ctx)
	if p, _ := g.DB().PhaseOf(ctx, "tx1"); p == db.PhaseBlocked {
		evs, _ := g.DB().Events(ctx, "tx1")
		for _, e := range evs {
			t.Logf("EV %s %s", e.Kind, e.Note)
		}
		t.Fatal("交付后退场被 blocked——T37 豁免未生效")
	}
	rs, _ := g.DB().RunsForTicket(ctx, "tx1")
	for _, r := range rs {
		if r.Role == "施工" && !r.Dead && r.RunID != run.RunID {
			t.Fatal("交付后旧席死亡触发了施工重派（应豁免）")
		}
	}
}

// 锚2：未交付席死照旧计失败——seat_dead→next_shell 重派（豁免不滥伤）。
func TestT37_UndeliveredDeathStillCounts(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "idle")
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.InjectRoot = injectFixture(t)
		c.ReconcileDebounce = 30 * time.Second
	})
	ctx := context.Background()
	mkTicketAt(t, g, "tx2", db.PhaseWorking)
	_ = g.DB().UpdateSpec(ctx, "tx2", testSpec(t))
	run := spawnSeat(t, g, "tx2", "施工", "idle")
	_ = syscall.Kill(-run.PID, syscall.SIGKILL) // 未交付即死
	waitFor2(t, 15*time.Second, func() bool {
		g.reconcile(ctx)
		rs, _ := g.DB().RunsForTicket(ctx, "tx2")
		for _, r := range rs {
			if r.Role == "施工" && r.RunID != run.RunID {
				return true
			}
		}
		return false
	}, g)
}
