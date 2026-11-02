package gateway

import (
	"context"
	"testing"
	"time"

	"meta/internal/db"
)

// TestT24_四闸 巡检空勤风暴四闸锚：A 待部署不派监工；B 短窗 exit=0
// 不记死；C 三死停派+CEO 通知；D 机器事件喂泵零触发。
func TestT24_四闸(t *testing.T) {
	g, dir := newTestGateway(t, nil)
	ctx := context.Background()

	// A 范围闸：待部署票信号→patrol_skip 不派监工。
	mkTicketAt(t, g, "t24a", db.PhaseToDeploy)
	g.routeSignal(ctx, Event{Kind: "sig", TicketID: "t24a", Note: "探"}, "巡检")
	// 派发实证＝run 行（相位链自带 dispatch 事件不算本泵发）。
	if rs, _ := g.DB().RunsForTicket(ctx, "t24a"); len(rs) != 0 {
		t.Fatalf("待部署票竟起监工席: %v", rs)
	}
	evs, _ := g.DB().Events(ctx, "t24a")
	var sawSkip bool
	for _, e := range evs {
		sawSkip = sawSkip || e.Kind == "patrol_skip"
	}
	if !sawSkip {
		t.Fatal("待部署票无 patrol_skip 留痕")
	}

	// B 语义闸：adjunct 短窗 exit=0→patrol_skip 不 adjunct_dead。
	runB := &db.Run{RunID: "rb24", TicketID: "t24a", Role: "巡检",
		Shell: "devin", RunToken: "t", Adjunct: true,
		StartedAt: time.Now()}
	_ = g.DB().CreateRun(ctx, runB)
	st := &runState{run: runB, exitCode: 0}
	g.onRunDead(st)
	evs, _ = g.DB().Events(ctx, "t24a")
	var sawDead, sawSkipB bool
	for _, e := range evs {
		sawDead = sawDead || e.Kind == "adjunct_dead"
		sawSkipB = sawSkipB || (e.Kind == "patrol_skip" && e.Note != "" &&
			len(e.Note) > 5)
	}
	if sawDead {
		t.Fatal("短窗 exit=0 误记 adjunct_dead")
	}
	if !sawSkipB {
		t.Fatal("短窗 exit=0 无 patrol_skip 留痕")
	}

	// C 退避闸：同票三死→停派＋一条 patrol_dead_cap。
	mkTicketAt(t, g, "t24c", db.PhaseWorking)
	for i := 0; i < 3; i++ {
		g.patrolDeathBump("t24c")
	}
	g.routeSignal(ctx, Event{Kind: "sig", TicketID: "t24c", Note: "探"}, "巡检")
	evs, _ = g.DB().Events(ctx, "t24c")
	var caps int
	for _, e := range evs {
		if e.Kind == "patrol_dead_cap" {
			caps++
		}
	}
	if rs, _ := g.DB().RunsForTicket(ctx, "t24c"); len(rs) != 0 {
		t.Fatal("过帽仍起监工席")
	}
	if caps != 1 {
		t.Fatalf("patrol_dead_cap 应 1 条: %d", caps)
	}

	// D 泵去自续：adjunct_dead 喂泵零触发（patrolCh 无入队）。
	g.patrolFeed(Event{Kind: "adjunct_dead", TicketID: "t24a"})
	select {
	case <-g.patrolCh:
		t.Fatal("机器事件误入泵")
	default:
	}
	_ = dir
}
