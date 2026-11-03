package gateway

// 票 T42 锚测：失败表复位门——resume/ruling 后各类失败计数清零重窗。
// 0050 实证：放行→秒回 blocked＝带病穿越；人工放行＝复位再观察。

import (
	"context"
	"testing"
	"time"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// 锚1：cap 超限→blocked→resume→新失败计数从零→下一失败按表处置
// （非秒回 blocked）。kill_respawn cap=2：放行后第 1/2 次失败照旧
// 处置，第 3 次才再 blocked。
func TestT42_ResumeResetsFailureCount(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) { c.ReconcileDebounce = 30 * time.Second })
	ctx := context.Background()
	mkTicketAt(t, g, "tr42", db.PhaseWorking)
	// 期窗内 3 次 stall_kill（cap=2）→blocked
	for i := 0; i < 3; i++ {
		_ = g.DB().AppendEvent(ctx, "tr42", "stall_kill", db.Event{Actor: "gateway"})
	}
	g.reconcile(ctx)
	if p, _ := g.DB().PhaseOf(ctx, "tr42"); p != db.PhaseBlocked {
		t.Fatalf("cap 超限未 blocked: %s", p)
	}
	// 人工放行→复位
	if ack, _ := g.Resume(ctx, &pbv1.TicketRef{Id: "tr42"}); ack == nil || !ack.Ok {
		t.Fatalf("Resume 未放行: %+v", ack)
	}
	if p, _ := g.DB().PhaseOf(ctx, "tr42"); p == db.PhaseBlocked {
		t.Fatal("resume 后秒回 blocked——复位未生效")
	}
	// 新计数从零：第 1/2 次 stall_kill 照表处置（kill_respawn，非 blocked）。
	for i := 0; i < 2; i++ {
		_ = g.DB().AppendEvent(ctx, "tr42", "stall_kill", db.Event{Actor: "gateway"})
		g.reconcile(ctx)
		if p, _ := g.DB().PhaseOf(ctx, "tr42"); p == db.PhaseBlocked {
			t.Fatalf("复位后第 %d 次失败即回 blocked（计数未清零）", i+1)
		}
	}
	// 第 3 次超 cap→再 blocked（表语义未废）。
	_ = g.DB().AppendEvent(ctx, "tr42", "stall_kill", db.Event{Actor: "gateway"})
	g.reconcile(ctx)
	if p, _ := g.DB().PhaseOf(ctx, "tr42"); p != db.PhaseBlocked {
		t.Fatal("复位后新 cap 超限未再 blocked")
	}
}

// 锚2：ruling 行同构复位（T30 cli-drive 复位径钦定同构）——裁决行后
// 失败计数亦清零。
func TestT42_RulingResetsCount(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) { c.ReconcileDebounce = 30 * time.Second })
	ctx := context.Background()
	mkTicketAt(t, g, "tr43", db.PhaseWorking)
	for i := 0; i < 2; i++ { // cap=2 的 stall_kill 先攒两次（未超限）
		_ = g.DB().AppendEvent(ctx, "tr43", "stall_kill", db.Event{Actor: "gateway"})
	}
	_ = g.DB().AppendEvent(ctx, "tr43", "ruling", db.Event{Actor: "CEO", Note: "裁"})
	// ruling 后两次 stall_kill——若旧账未清则 2+2>2 早 blocked；复位则恰平。
	for i := 0; i < 2; i++ {
		_ = g.DB().AppendEvent(ctx, "tr43", "stall_kill", db.Event{Actor: "gateway"})
	}
	g.reconcile(ctx)
	if p, _ := g.DB().PhaseOf(ctx, "tr43"); p == db.PhaseBlocked {
		t.Fatal("ruling 后旧账未清——复位未生效")
	}
}
