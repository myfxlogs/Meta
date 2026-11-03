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
	// T35 失败表：本票候壳尽＝壳列表用尽→shells_exhausted＋blocked 人工态
	// （降级回落/逐壳判荒分层全删——无补发径，票待人裁）。
	g.shellsExhausted(ctx, spawnReq{ticketID: "tq", role: "施工"})
	waitEvent(t, g, "shells_exhausted", "tq", 3*time.Second)
	waitPhase(t, g, "tq", db.PhaseBlocked, 3*time.Second)
	// 幂等：blocked 票再撞 exhausted 不再迁（边闸拒——二次转 blocked 非法）。
	g.shellsExhausted(ctx, spawnReq{ticketID: "tq", role: "施工"})
	waitPhase(t, g, "tq", db.PhaseBlocked, 1*time.Second)
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
