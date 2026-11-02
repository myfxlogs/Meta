package db

import (
	"context"
	"testing"
	"time"
)

// TestT36_PhaseEnteredAt 票0036 对抗锚 b：PhaseEnteredAt 取「最近一次进入
// 当前相位」变迁事件的 created_at——无关 kind 事件不扰（取任意最新事件者红）、
// UpdateSpec 拨 updated_at 不扰（取 ticket.updated_at 者红）、回摆取本段
// 最近一次（ORDER BY id ASC 者红）。构造全经代码径＋SetClock（D-56 禁
// INSERT）。
func TestT36_PhaseEnteredAt(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	base := time.Unix(1762000000, 0)
	now := base
	d.SetClock(func() time.Time { return now })
	mkTicket(t, d, "te") // created 事件 to_phase=已立 @base

	step := func(to Phase, kind EdgeKind, at time.Duration) {
		now = base.Add(at)
		if _, err := d.Transition(ctx, "te", to, kind, "test", ""); err != nil {
			t.Fatalf("→%s: %v", to, err)
		}
	}
	step(PhaseDesigning, EdgeDesignStart, time.Minute)
	step(PhaseAssigned, EdgeDispatch, 2*time.Minute)
	step(PhaseWorking, EdgeClaim, 5*time.Minute) // 施工中本段进入 @base+5m

	// 无关事件（progress/usage，to_phase=''）——取「任意 kind 最新事件」
	// 的变异实现在此红：返回值必须是进入事件而非 @base+6m。
	now = base.Add(6 * time.Minute)
	if err := d.AppendEvent(ctx, "te", "progress", Event{Actor: "r1", Note: "写码 落表"}); err != nil {
		t.Fatal(err)
	}
	if err := d.AppendEvent(ctx, "te", "usage", Event{Actor: "r1"}); err != nil {
		t.Fatal(err)
	}
	// UpdateSpec 拨 updated_at→@base+7m——取 ticket.updated_at 的变异在此红。
	now = base.Add(7 * time.Minute)
	if err := d.UpdateSpec(ctx, "te", nil); err != nil {
		t.Fatal(err)
	}
	want := base.Add(5 * time.Minute)
	if got, err := d.PhaseEnteredAt(ctx, "te", PhaseWorking); err != nil || !got.Equal(want) {
		t.Fatalf("施工中进入时刻: got=%v want=%v err=%v", got, want, err)
	}

	// 回摆：施工中→已派→施工中——两段同相位进入，取最近一次（本段）。
	// ORDER BY id ASC 的变异在此红（会取回 @base+5m）。
	step(PhaseAssigned, EdgeYield, 8*time.Minute)
	step(PhaseWorking, EdgeClaim, 9*time.Minute)
	want = base.Add(9 * time.Minute)
	if got, err := d.PhaseEnteredAt(ctx, "te", PhaseWorking); err != nil || !got.Equal(want) {
		t.Fatalf("回摆后进入时刻: got=%v want=%v err=%v", got, want, err)
	}
}

// TestT36_PhaseEnteredAtFallback 降级链：所查相位无对应事件→最近一条
// to_phase<>'' 真实变迁行；票不在→err（装配径自行降 updated_at）。
func TestT36_PhaseEnteredAtFallback(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	base := time.Unix(1762000000, 0)
	now := base
	d.SetClock(func() time.Time { return now })
	mkTicket(t, d, "tf")
	now = base.Add(time.Minute)
	if _, err := d.Transition(ctx, "tf", PhaseDesigning, EdgeDesignStart, "test", ""); err != nil {
		t.Fatal(err)
	}
	// 待审无对应事件 → 最近 to_phase<>'' 行＝design_start @base+1m
	if got, err := d.PhaseEnteredAt(ctx, "tf", PhaseReview); err != nil || !got.Equal(base.Add(time.Minute)) {
		t.Fatalf("降级取最近变迁: got=%v err=%v", got, err)
	}
	// 已立相位 → created 事件（to_phase=已立）@base
	if got, err := d.PhaseEnteredAt(ctx, "tf", PhaseCreated); err != nil || !got.Equal(base) {
		t.Fatalf("已立进入时刻: got=%v want=%v err=%v", got, base, err)
	}
	// 无票 → err（cardView 败径降 updated_at）
	if _, err := d.PhaseEnteredAt(ctx, "nope", PhaseWorking); err == nil {
		t.Fatal("无票应 err")
	}
}
