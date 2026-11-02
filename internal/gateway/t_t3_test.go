package gateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"meta/internal/db"
	"meta/internal/route"
)

// TestT3_逐壳判荒 D-75 锚：单壳尽→shells_degraded 不 exhausted（不误报
// 全荒不杀席）；角色全部注册壳不可用→shells_exhausted。
func TestT3_逐壳判荒(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		c.ShellRows = []ShellSeed{
			{Name: "a", Model: "m1", Role: "施工", Priority: 1},
			{Name: "b", Model: "m2", Role: "施工", Priority: 2},
		}
	})
	ctx := context.Background()
	mkTicketAt(t, g, "tq", db.PhaseAssigned)
	if _, err := g.shellsFor(ctx, "施工"); err != nil { // 先落种再标死
		t.Fatal(err)
	}
	// 单壳 a 尽（health=dead）＋failSet 已试——余 b 健康：degraded 不 exhausted。
	_ = g.DB().SetShellHealth(ctx, db.Shell{Name: "a", Model: "m1", Role: "施工"},
		route.HealthDead)
	g.mu.Lock()
	g.failSet["tq"] = map[string]bool{db.Shell{Name: "a", Model: "m1", Role: "施工"}.ShellKey(): true}
	g.mu.Unlock()
	// T26 病5：首尽＝degraded_retry（清 failSet 回落重试，非误报全荒）；
	// 再尽才 shells_degraded——无 spec 重试消费端撞「无派发参数」闸不真起席。
	g.shellsExhausted(ctx, spawnReq{ticketID: "tq", role: "施工"})
	waitEvent(t, g, "degraded_retry", "tq", 3*time.Second)
	g.mu.Lock()
	if len(g.failSet["tq"]) != 0 {
		g.mu.Unlock()
		t.Fatal("degraded_retry 应清 failSet 回落重试")
	}
	g.mu.Unlock()
	g.shellsExhausted(ctx, spawnReq{ticketID: "tq", role: "施工"})
	waitEvent(t, g, "shells_degraded", "tq", 3*time.Second)
	evs, _ := g.DB().Events(ctx, "tq")
	for _, e := range evs {
		if e.Kind == "shells_exhausted" {
			t.Fatal("单壳尽误报全荒——D-75 复现")
		}
	}
	// 全壳尽→exhausted。
	_ = g.DB().SetShellHealth(ctx, db.Shell{Name: "b", Model: "m2", Role: "施工"},
		route.HealthDead)
	g.shellsExhausted(ctx, spawnReq{ticketID: "tq", role: "施工"})
	waitEvent(t, g, "shells_exhausted", "tq", 3*time.Second)
}

// TestT3_去run术语 锚：ruling/stall/零写 note 无 run= 术语（决策23/48）。
func TestT3_去run术语(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "tj", db.PhaseWorking)
	st := &runState{run: &db.Run{RunID: "rx", TicketID: "tj", Role: "CTO"}}
	g.ruleLanded(ctx, "tj", st, "defer", &db.Run{RunID: "ry"}, "延期")
	evs, _ := g.DB().Events(ctx, "tj")
	for _, e := range evs {
		if e.Kind == "ruling" && strings.Contains(e.Note, "run=") {
			t.Fatalf("ruling note 泄漏 run= 术语: %s", e.Note)
		}
	}
}
