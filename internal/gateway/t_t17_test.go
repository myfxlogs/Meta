package gateway

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// TestT17_挂起席必报 锚①：attach 后零帧挂起席——停滞钟响出
// stall_suspect（挂起≠活，不等 claim 才起算）。
func TestT17_挂起席必报(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.ProgressTimeout = 400 * time.Millisecond
	})
	ctx := context.Background()
	mkTicketAt(t, g, "t17", db.PhaseAssigned)
	if err := g.DB().CreateRun(ctx, &db.Run{
		RunID: "r17", TicketID: "t17", Role: "施工", Shell: "devin",
		RunToken: "tok17"}); err != nil {
		t.Fatal(err)
	}
	stream := attachClient(t, g, filepath.Join(dir, "run", "meta.sock"), "r17", "tok17")
	defer stream.CloseSend()
	// 零 claim 零 progress——闹钟必响。
	waitEvent(t, g, "stall_suspect", "t17", 5*time.Second)
}

// TestT17_收养不复位 锚②：收养席重领 claim 帧不复位停滞钟；
// 真 progress 帧仍复位（对账重收养不续命）。
func TestT17_收养不复位(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.ProgressTimeout = 500 * time.Millisecond
	})
	ctx := context.Background()
	mkTicketAt(t, g, "t17b", db.PhaseWorking)
	if err := g.DB().CreateRun(ctx, &db.Run{
		RunID: "r17b", TicketID: "t17b", Role: "施工", Shell: "devin",
		RunToken: "tok17b"}); err != nil {
		t.Fatal(err)
	}
	// 收养态席重领（st.adopted=true 直接置位模拟对账收养）。
	g.mu.Lock()
	st := &runState{run: &db.Run{RunID: "r17b", TicketID: "t17b",
		Role: "施工", Adjunct: true}, adopted: true}
	g.runs["r17b"] = st
	g.mu.Unlock()
	// 预挂短钟（原点 deadline=now+300ms）；adopted claim 不复位→
	// 原点照响；若被复位→deadline 后延 500ms 窗内不响。
	g.armAlarm("t17b", time.Now().Add(300*time.Millisecond))
	_ = g.handleClaim(ctx, st, &pbv1.Claim{TicketId: "t17b"})
	waitEvent(t, g, "stall_suspect", "t17b", 3*time.Second)
	// 对照：非收养席同径 claim 必复位（deadline 远推不响——用
	// alarms 表验新 deadline>原点）。
	mkTicketAt(t, g, "t17c", db.PhaseWorking)
	st2 := &runState{run: &db.Run{RunID: "r17c", TicketID: "t17c",
		Role: "施工", Adjunct: true}, adopted: false}
	g.mu.Lock()
	g.runs["r17c"] = st2
	g.mu.Unlock()
	if err := g.DB().CreateRun(ctx, &db.Run{
		RunID: "r17c", TicketID: "t17c", Role: "施工", Shell: "devin",
		RunToken: "tok17c", Adjunct: true}); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	_ = g.handleClaim(ctx, st2, &pbv1.Claim{TicketId: "t17c"})
	g.mu.Lock()
	al, ok2 := g.alarms["t17c"]
	g.mu.Unlock()
	if !ok2 || !al.Deadline().After(before.Add(300*time.Millisecond)) {
		t.Fatalf("非收养 claim 未复位闹钟: ok=%v", ok2)
	}
	_ = dir
}
