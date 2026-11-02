package gateway

import (
	"context"
	"path/filepath"
	"testing"

	"meta/internal/db"
)

// TestT15_LockLeak 锁泄锚：待部署 CTO 取双锁→席 claim 前死→锁必释
// （spawn_blocked 死循环实证根修）。
func TestT15_LockLeak(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkTicketAt(t, g, "tl15", db.PhaseToDeploy)
	_ = g.DB().SetTicketProject(ctx, "tl15", "P")
	if err := g.DispatchSeat(ctx, "tl15", "CTO", deploySpec(t)); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	held := len(g.projectLocks["P"])
	g.mu.Unlock()
	if held != 2 {
		t.Fatalf("双锁未取: %d", held)
	}
	// 席 claim 前死（相位未离 待部署）→死席径释双锁。
	run := waitRun(t, g, "tl15", "CTO")
	st := &runState{run: run}
	st.exited, st.streamClosed = true, true
	g.onRunDead(st)
	g.mu.Lock()
	remain := g.projectLocks["P"]
	g.mu.Unlock()
	if len(remain) != 0 {
		t.Fatalf("席死锁泄: %v", remain)
	}
}
