package gateway

// 票 T53③ 锚测：环境即死判 spawn_env_fail（壳不烧/有界重试）。

import (
	"path/filepath"

	"context"
	"meta/internal/db"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestT53_SpawnEnvFailNotSeatDead(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.ReconcileDebounce = 30 * time.Second
		c.SeatArgv = []string{os.Args[0], "-test.run=TestHelperProcess", "-test.v=false"}
	})
	ctx := context.Background()
	mkTicketAt(t, g, "te", db.PhaseAssigned)
	run := spawnSeat(t, g, "te", "施工", "idle")
	// 环境即死签名＝attach 前 exit=-1（SIGKILL 直杀——scope 起不来同面）。
	_ = syscall.Kill(-run.PID, syscall.SIGKILL)
	waitFor2(t, 10*time.Second, func() bool { return !processAlive(run.PID) }, g)
	g.mu.Lock()
	st := g.runs[run.RunID]
	g.mu.Unlock()
	if st == nil {
		t.Fatal("runState 未注册")
	}
	g.onRunDead(st)
	evs, _ := g.DB().Events(ctx, "te")
	var envFail, seatDead bool
	for _, e := range evs {
		if e.Kind == "spawn_env_fail" {
			envFail = true
		}
		if e.Kind == "seat_dead" {
			seatDead = true
		}
	}
	if st.attached {
		// attach 过的席不属环境即死——改判 seat_dead 语义仍应成立
		if !seatDead {
			t.Fatal("attached 席应记 seat_dead")
		}
		return
	}
	if !envFail || seatDead {
		t.Fatalf("环境即死应 spawn_env_fail 且非 seat_dead: env=%v dead=%v", envFail, seatDead)
	}
}
