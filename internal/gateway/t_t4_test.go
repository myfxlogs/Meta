package gateway

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"meta/internal/db"
)

// TestT4_BatchDeploy 批量闸锚（T35 并入调和器）：待部署≥5 或流水线清空
// （无 施工中/设计中）→调和器起 CTO-B；不足批且流水线忙→挂起不派。
func TestT4_BatchDeploy(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "idle")
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.ReconcileDebounce = 60 * time.Second // 关周期噪声只留事件径
			c.SelfPath = "/bin/false" // cli-drive 即死放锁（测径手工非 CliDrive 席证语义）
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	noRun := func(tid string) {
		if r, _ := g.DB().ActiveRunForTicket(ctx, tid); r != nil {
			t.Fatalf("流水线忙竟起席 %s: %s", tid, r.RunID)
		}
	}

	// 流水线忙（w1 施工中）＋4 张待部署→不触发。
	mkTicketAt(t, g, "w1", db.PhaseWorking)
	for _, id := range []string{"b1", "b2", "b3"} {
		mkToDeploy(t, g, id, "P")
	}
	g.reconcile(ctx)
	<-time.After(300 * time.Millisecond)
	for _, id := range []string{"b0", "b1", "b2", "b3"} {
		noRun(id)
	}

	// 第 5 张到待部署（流水线仍忙）→≥5 开闸：首票拿 B1 双锁起 CTO-B
	// （同项目串行——他票 spawn_blocked 属闸内语义，非调和失格）。
	mkToDeploy(t, g, "b4", "P")
	g.reconcile(ctx)
	// 开闸证：任一待部署票派发尝试/run 落账（B1 串行——谁先谁后属竞态）。
	waitFor2(t, 30*time.Second, func() bool {
		for _, id := range []string{"b0", "b1", "b2", "b3", "b4"} {
			evs, _ := g.DB().Events(ctx, id)
			for _, e := range evs {
				if e.Kind == "spawn_failed" || e.Kind == "seat_err" ||
					e.Kind == "shells_exhausted" || e.Kind == "spawn_blocked" {
					return true
				}
			}
			rs, _ := g.DB().RunsForTicket(ctx, id)
			if len(rs) > 0 {
				return true
			}
		}
		return false
	}, g)
}

// TestT4_PipelineClear 流水线清空（无 施工中/设计中）→<5 张待部署也开闸。
func TestT4_PipelineClear(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "idle")
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.ReconcileDebounce = 60 * time.Second
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "pb0", "P")
	g.reconcile(ctx)
	waitFor2(t, 15*time.Second, func() bool {
		rs, _ := g.DB().RunsForTicket(ctx, "pb0")
		return len(rs) > 0
	}, g)
}
