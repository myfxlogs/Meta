package gateway

import (
	"context"
	"testing"
	"time"

	pbv1 "meta/internal/pb/metav1"
)

// TestT31_PatrolIdle 票0031④ 锚：静默期闹钟——无事件到点 emit patrol_tick；
// 有事件即重置（事件后短窗内不响）；timer 文件删净（仓内无 systemd 件）。
func TestT31_PatrolIdle(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		c.Subs = []SubSeed{{Kind: "patrol_tick", Role: "巡检"}}
	})
	g.patrolIdleSpan = 80 * time.Millisecond
	g.armPatrolIdle()

	// 真事件武装：event→重置，80ms 窗内不响→再等必响。
	g.emit(Event{Kind: "noise", Note: "活性证据"})
	<-time.After(40 * time.Millisecond)
	g.emit(Event{Kind: "noise2", Note: "再来"})
	// 距第二事件 50ms<80ms 收集一窗：见 patrol_tick＝重置失效即红。
	for _, e := range collectEvents(g, 50*time.Millisecond) {
		if e.Kind == "patrol_tick" {
			t.Fatal("事件重置失效——未到点 patrol_tick 响了")
		}
	}
	waitEvent(t, g, "patrol_tick", "", 3*time.Second)

	// 静默期心跳事件落库（ticket_id 空串行——外部 inject 同径）。
	evs, _ := g.DB().EventsAfter(context.Background(), 0, 200)
	var saw bool
	for _, e := range evs {
		if e.Kind == "patrol_tick" && e.Provenance == "idle_alarm" {
			saw = true
		}
	}
	if !saw {
		t.Fatal("idle_alarm 心跳未落 ticket_event 行")
	}
}

// TestT31_PatrolDispatchRegression ④回归锚：patrol_tick 注入仍走订阅→
// 批收→派发径（闹钟源与 inject 源同消费——改 dispatcher 即红）。
func TestT31_PatrolDispatchRegression(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		c.Subs = []SubSeed{{Kind: "patrol_tick", Role: "巡检"}}
		c.Filter.BatchWindow = 50 * time.Millisecond
	})
	ctx := context.Background()
	ack, err := g.InjectEvent(ctx, &pbv1.EventReq{Kind: "patrol_tick"})
	if err != nil || !ack.Ok {
		t.Fatal(err)
	}
	// 无票信号→立 patrol-* 票→派发（无 InjectRoot→票0034 兜底件过 D-40
	// 抵壳路由 shells_exhausted 取证到达派发径，同 TestT30_PatrolRoute）。
	deadline := time.Now().Add(15 * time.Second)
	var pid string
	for time.Now().Before(deadline) && pid == "" {
		if ts, err := g.DB().NonTerminalTickets(ctx); err == nil {
			for _, tk := range ts {
				if len(tk.ID) >= 7 && tk.ID[:7] == "patrol-" {
					pid = tk.ID
				}
			}
		}
		if pid == "" {
			<-time.After(20 * time.Millisecond)
		}
	}
	if pid == "" {
		t.Fatal("patrol_tick 未立值班票")
	}
	waitEvent(t, g, "shells_degraded", pid, 15*time.Second)
}
