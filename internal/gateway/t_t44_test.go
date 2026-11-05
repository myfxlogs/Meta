package gateway

// 票 T44 锚测：僵尸席判死——processAlive 对 zombie 恒活是「施工中席死
// 24min 调和零动作」（0052 实证）的根因。

import (
	"context"
	"os"
	"testing"
	"time"

	"meta/internal/db"
)

// 锚1：zombie PID 不判活——kill(pid,0) 谎报不采（/proc state Z 即死）。
func TestT44_ZombieNotAlive(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	run := spawnSeat(t, g, "", "施工", "exitcode")
	// g.Spawn 直建径无 Wait 协程——子退后成僵尸挂账（实证面即此形态）。
	waitFor2(t, 5*time.Second, func() bool { return !processAlive(run.PID) }, g)
}

// 锚2：僵尸席→调和器标死＋seat_dead 账＋换壳重派（全链通）。
func TestT44_ZombieReconcileRespawn(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		c.ReconcileDebounce = 30 * time.Second
		c.SeatArgv = []string{os.Args[0], "-test.run=TestHelperProcess", "-test.v=false"}
	})
	ctx := context.Background()
	mkTicketAt(t, g, "tz44", db.PhaseWorking)
	run := spawnSeat(t, g, "tz44", "施工", "exitcode")
	waitFor2(t, 5*time.Second, func() bool { return !processAlive(run.PID) }, g)
	g.reconcile(ctx)
	waitFor2(t, 10*time.Second, func() bool {
		st, _ := g.DB().RunByToken(ctx, run.RunToken)
		return st != nil && st.Dead
	}, g)
	evs, _ := g.DB().Events(ctx, "tz44")
	for _, e := range evs {
		if e.Kind == "seat_dead" {
			return
		}
	}
	t.Fatal("seat_dead 账未落——僵尸席调和器零动作")
}
