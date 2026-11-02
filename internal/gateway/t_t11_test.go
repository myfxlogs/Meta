package gateway

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"os"
)

// TestT11_点火洞 票面四锚：
// ①裸立票→≤去抖窗+5s spawn_needed 必发（无人工无重启）
// ②窗内两事件只醒一次（reconcileNow 计数）
// ③patrol-* 立票→醒来派巡检不误派 CTO（spawn_needed role 断言）
// ④新路径一次性闹钟——对账在飞期事件不续命（reconBusy 抑制）
func TestT11_点火洞(t *testing.T) {
	var calls int32
	g, _ := newTestGateway(t, func(c *Config) {
		c.ReconcileDebounce = 80 * time.Millisecond
	})
	g.reconcileNow = func(ctx context.Context) {
		atomic.AddInt32(&calls, 1)
		g.reconcile(ctx) // 真对账照跑（spawn_needed 断言需要）
	}
	ctx := context.Background()

	// ①裸立票（meta ticket new 同径）→去抖醒→spawn_needed。
	if err := g.DB().CreateTicketProv(ctx, "tA", "裸票", nil, "", ""); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "spawn_needed", "tA", 8*time.Second)

	// ②窗内连续两事件只醒一次：calls 已计一次（上段唤醒），
	// 立刻再立两票，窗内归并——醒后 calls 只 +1。
	base := atomic.LoadInt32(&calls)
	if err := g.DB().CreateTicketProv(ctx, "tB", "二", nil, "", ""); err != nil {
		t.Fatal(err)
	}
	<-time.After(30 * time.Millisecond) // 窗内
	if err := g.DB().CreateTicketProv(ctx, "tC", "三", nil, "", ""); err != nil {
		t.Fatal(err)
	}
	<-time.After(300 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != base+1 {
		t.Fatalf("窗内两事件应只醒一次，calls=%d base=%d", got, base)
	}

	// ③patrol-* 立票：reconcile 已立行给 巡检 角色（D-54 前缀行），
	// 不得误派 CTO。
	if err := g.DB().CreateTicketProv(ctx, "patrol-999", "值班", nil, "", ""); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for {
		e := waitEvent(t, g, "spawn_needed", "patrol-999", time.Until(deadline))
		if e.Role == "巡检" {
			break
		}
		if e.Role == "CTO" {
			t.Fatalf("patrol 票误派 CTO: %+v", e)
		}
	}

	// ④对账在飞期不续命＋机器记账不武装（纯函数级断言——异步事件流
	// 无法隔离，直调 armRecon 验闸本身）。
	g.mu.Lock()
	g.reconBusy = true
	g.mu.Unlock()
	g.armRecon("created")
	g.mu.Lock()
	armed := g.reconTimer != nil
	g.reconBusy = false
	g.mu.Unlock()
	if armed {
		t.Fatal("reconBusy 期竟武装闹钟——自锁环复现")
	}
	g.mu.Lock()
	g.reconTimer = nil
	g.mu.Unlock()
	g.armRecon("spawn_needed") // 机器记账 kind
	g.mu.Lock()
	armed = g.reconTimer != nil
	g.mu.Unlock()
	if armed {
		t.Fatal("spawn_needed 竟武装闹钟——R2 变相轮询复现")
	}
}

// TestT11_零周期码锚 代码锚：armRecon 用一次性 AfterFunc——源内无
// Ticker/周期 goroutine 引 recon。
func TestT11_零周期码锚(t *testing.T) {
	b, err := os.ReadFile("gateway.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "NewTicker") {
		t.Fatal("gateway.go 现 NewTicker——违 R2 零周期锚")
	}
	if !strings.Contains(s, "time.AfterFunc(span, func()") {
		t.Fatal("armRecon 非一次性闹钟形态")
	}
}
