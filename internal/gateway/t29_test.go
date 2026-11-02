package gateway

// T29 巡逻泵两病——两病各一锚。

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// 病①+② 巡逻票恒巡检＋零设计锁：patrol-*（含 -pN 子票）无论派发方
// 报什么角色一律归位巡检；CTO 角色派发不再走设计支取锁（0048 实证）。
// 对抗锚提名行：patrol- 归位行——删则杂票又能抓走设计锁。
func TestT29_PatrolRoleNormalize(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.ShellRows = append(c.ShellRows,
			ShellSeed{Name: "devin", Role: "巡检", Priority: 1})
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	for _, tid := range []string{"patrol-900", "patrol-900-p1"} {
		if err := g.DB().CreateTicket(ctx, tid, "值班票", nil); err != nil {
			t.Fatal(err)
		}
		if err := g.DB().SetTicketProject(ctx, tid, "P"); err != nil {
			t.Fatal(err)
		}
		// 派发方报 CTO——巡逻票应被机械归位巡检（含 -pN 子票）。
		if err := g.DispatchSeat(ctx, tid, "CTO", dispatchSpec(t)); err != nil {
			t.Fatalf("%s 派发归位应放行: %v", tid, err)
		}
		waitEvent(t, g, "seat_up", tid, 8*time.Second)
		runs, _ := g.DB().RunsForTicket(ctx, tid)
		last := runs[len(runs)-1]
		if last.Role != "巡检" {
			t.Fatalf("%s 派单角色应恒巡检: %s", tid, last.Role)
		}
	}
	g.mu.Lock()
	held := g.designLocks["P"]
	g.mu.Unlock()
	if held != "" {
		t.Fatalf("巡逻票不该持项目设计锁: %s", held)
	}
}
