package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestT11_点火洞（T35 改义：kick＋周期双源调和器）票面锚：
// ①裸立票→事件 kick→调和器起 CTO 席（无人工无重启）
// ②kick 道缓冲 1 自并——忙期多踢不堆队
// ③机器记账 kind 不踢（自踢自转＝变相轮询放大）
func TestT11_点火洞(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "worker")
	var calls int32
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.ReconcileDebounce = 60 * time.Second // 关周期噪声：本锚只验事件径
		c.InjectRoot = injectFixture(t)
	})
	g.reconcileNow = func(ctx context.Context) {
		atomic.AddInt32(&calls, 1)
		g.reconcile(ctx)
	}
	ctx := context.Background()

	// ①裸立票（meta ticket new 同径）→事件落账踢调和→CTO-A 席起。
	if err := g.DB().CreateTicketProv(ctx, "tA", "裸票", nil, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := g.DB().UpdateSpec(ctx, "tA", testSpec(t)); err != nil {
		t.Fatal(err)
	}
	waitFor2(t, 10*time.Second, func() bool {
		r, _ := g.DB().ActiveRunForTicketRole(ctx, "tA", "CTO")
		return r != nil
	})

	// ②忙期多踢自并：向 kick 道连灌，计数只 +少量（chan cap 1）。
	base := atomic.LoadInt32(&calls)
	for i := 0; i < 8; i++ {
		g.kickRecon("created")
	}
	<-time.After(500 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got > base+2 {
		t.Fatalf("多踢未自并 calls=%d base=%d", got, base)
	}

	// ③机器记账 kind 不踢：blocked/seat_err 等不调 reconcileNow。
	base = atomic.LoadInt32(&calls)
	g.kickRecon("blocked")
	g.kickRecon("seat_err")
	<-time.After(300 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != base {
		t.Fatalf("机器记账 kind 竟踢调和 calls=%d base=%d", got, base)
	}
}

// TestT11_幂等锚：同世界态 reconcile 连跑 N 次产出相同（v2 §3.3-A
// 钦定锚——desired==actual 跳过，seat_run 数不变）。
func TestT11_幂等锚(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "worker")
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.ReconcileDebounce = 60 * time.Second
		c.InjectRoot = injectFixture(t)
	})
	ctx := context.Background()
	if err := g.DB().CreateTicketProv(ctx, "tI", "幂等", nil, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := g.DB().UpdateSpec(ctx, "tI", testSpec(t)); err != nil {
		t.Fatal(err)
	}
	// 等调和器把 CTO 席派起来。
	waitFor2(t, 10*time.Second, func() bool {
		r, _ := g.DB().ActiveRunForTicketRole(ctx, "tI", "CTO")
		return r != nil
	})
	rs1, _ := g.DB().RunsForTicket(ctx, "tI")
	g.reconcile(ctx)
	g.reconcile(ctx)
	rs2, _ := g.DB().RunsForTicket(ctx, "tI")
	if len(rs1) != len(rs2) {
		t.Fatalf("幂等破：连跑 run 数 %d→%d", len(rs1), len(rs2))
	}
}

// TestT11_R2锚 代码锚：周期循环仅此一处（reconcileLoop 的 R2′ 钦定
// 例外）＋驱动为 buffered kick 道非 sleep 轮询。
func TestT11_R2锚(t *testing.T) {
	b, err := os.ReadFile("gateway.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "NewTicker") || !strings.Contains(s, "reconKick") {
		t.Fatal("调和器驱动形态缺（ticker+kick 钦定双源）")
	}
	if strings.Contains(s, "for {") && strings.Contains(s, "time.Sleep") &&
		strings.Index(s, "time.Sleep") < strings.Index(s, "reconcileLoop") {
		t.Fatal("调和器径外现 sleep 轮询嫌疑")
	}
}
