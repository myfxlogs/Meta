package gateway

// 票 T49 锚测：attach 席死径与 reconcile 径同窗双记根治——期窗恰 1 条
// deploy_failed（0051 实证：seat_dead 与 deploy_failed 同秒×2 凑 cap）。

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestT49_AttachReconcileSingleDeployFailed(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.ReconcileDebounce = 30 * time.Second
		c.SeatArgv = []string{os.Args[0], "-test.run=TestHelperProcess", "-test.v=false"}
	})
	ctx := context.Background()
	mkDeploying(t, g, "tr49", "P")
	run := spawnSeat(t, g, "tr49", "CTO", "exitcode")
	waitFor2(t, 5*time.Second, func() bool { return !processAlive(run.PID) }, g)
	// 双径同窗：attach 席死径（MarkDead+哨兵判）＋调和径——两径都过
	// 「席死无据」支，期窗恰 1 条 deploy_failed（T49 钦定锚）。
	_ = g.DB().MarkDead(ctx, run.RunID)
	g.mu.Lock()
	st := g.runs[run.RunID]
	g.mu.Unlock()
	if st != nil {
		g.onRunDead(st)
	}
	g.reconcile(ctx)
	n := 0
	evs, _ := g.DB().Events(ctx, "tr49")
	for _, e := range evs {
		if e.Kind == "deploy_failed" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("双径同窗应恰 1 条 deploy_failed: %d", n)
	}
}
