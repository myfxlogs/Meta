package gateway

// 票 T45 锚测：Working×CTO 血统角——最近 run 是 CTO 血统时，
// 血统角色不可派（DispatchSeat 硬拒）→退化「施工」起席。

import (
	"context"
	"os"
	"testing"
	"time"

	"meta/internal/db"
)

func TestT45_WorkingCtoLineageFallsBack(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		c.ReconcileDebounce = 30 * time.Second
		c.SeatArgv = []string{os.Args[0], "-test.run=TestHelperProcess", "-test.v=false"}
		c.InjectRoot = injectFixture(t)
		deepenShellPool(c) // T53⑨：名单片普查补齐
	})
	ctx := context.Background()
	mkTicketAt(t, g, "tz45", db.PhaseWorking)
	// 造一具 CTO 血统死席（施工中唯一席=CTO 血统=0052 实证形态）。
	run := spawnSeat(t, g, "tz45", "CTO", "exitcode")
	waitFor2(t, 5*time.Second, func() bool { return !processAlive(run.PID) }, g)
	_ = g.DB().MarkDead(ctx, run.RunID)
	_ = g.DB().UpdateSpec(ctx, "tz45", testSpec(t))
	// 调和器接手：血统=CTO 不可派→须退化成「施工」起新席（非静默吞）。
	g.reconcile(ctx)
	waitFor2(t, 10*time.Second, func() bool {
		rs, _ := g.DB().RunsForTicket(ctx, "tz45")
		for _, r := range rs {
			if !r.Dead && r.Role == "施工" {
				return true
			}
		}
		return false
	}, g)
}
