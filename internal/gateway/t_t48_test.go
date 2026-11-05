package gateway

// 票 T48 锚测：deploy_failed「死席无据」账边沿化——同期窗不重复追加。
// 0050 实证：resume 后第一轮记 1、毫秒级第二轮记 2>cap→blocked 全灭。

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestT48_DeployFailedDedupe(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.ReconcileDebounce = 30 * time.Second
		c.SeatArgv = []string{os.Args[0], "-test.run=TestHelperProcess", "-test.v=false"}
	})
	ctx := context.Background()
	mkDeploying(t, g, "tr48", "P")
	// 造死席（无哨兵证据形态）→第一轮调和记 1 条 deploy_failed。
	run := spawnSeat(t, g, "tr48", "CTO", "exitcode")
	waitFor2(t, 5*time.Second, func() bool { return !processAlive(run.PID) }, g)
	_ = g.DB().MarkDead(ctx, run.RunID)
	g.reconcile(ctx)
	cnt := func() int {
		evs, _ := g.DB().Events(ctx, "tr48")
		n := 0
		for _, e := range evs {
			if e.Kind == "deploy_failed" {
				n++
			}
		}
		return n
	}
	n0 := cnt()
	if n0 < 1 {
		t.Fatalf("首轮无 deploy_failed 账: %d", n0)
	}
	// 第二轮（模拟 emit 自 kick 的毫秒级续跑）——零追加（边沿化；
	// attach 席死径同窗双记属 T49 票面未修，与本径去重无涉）。
	g.reconcile(ctx)
	if got := cnt() - n0; got != 0 {
		t.Fatalf("二轮应零追加（边沿化）: +%d", got)
	}
}
