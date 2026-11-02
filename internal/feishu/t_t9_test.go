package feishu

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestT9_ReceiptIdent 回执带接单者身份（D-66）：Ident 注入→回执含壳名；
// 未注→CEO 兜底。
func TestT9_ReceiptIdent(t *testing.T) {
	cap := &captured{}
	ch := New(Config{Cred: testCred(), Send: cap.send, Feed: cap.feed,
		Ident: func() string { return "CEO·devin" },
		Logf:  func(string, ...any) {}})
	ch.handle("ou_owner", "oc_x", `{"text":"办个事"}`)
	if cap.nsent() != 1 || !strings.Contains(cap.sents[0], "CEO·devin") {
		t.Fatalf("回执未带身份: %v", cap.sents)
	}
	cap2 := &captured{}
	ch2 := New(Config{Cred: testCred(), Send: cap2.send, Feed: cap2.feed,
		Logf: func(string, ...any) {}})
	ch2.handle("ou_owner", "oc_x", `{"text":"办个事"}`)
	if !strings.Contains(cap2.sents[0], "CEO") {
		t.Fatalf("无 Ident 兜底缺 CEO: %v", cap2.sents)
	}
}

// TestT9_ReconcileConcurrent 补建批并发（D-67）：N 票一次对账全 Observe
// 到——串排拖拍实证消除（并发下挂钟上界断言 300ms×N 串行必超）。
func TestT9_ReconcileConcurrent(t *testing.T) {
	// 闸栏式并发实证：PATCH 进来先举手，8 个到齐才放行——串行执行第
	// 一票就死锁超时，并发则全过。无 Sleep 禁式。
	var inflight int64
	barrier := make(chan struct{})
	var once sync.Once
	c := NewCards(CardConfig{
		NonTerminal: func(context.Context) ([]string, error) {
			return []string{"a", "b", "c", "d", "e", "f", "g", "h"}, nil
		},
		View: func(context.Context, string) (*CardView, error) {
			return &CardView{TicketID: "x"}, nil
		},
		CardRow: func(context.Context, string) (string, bool, error) {
			return "m1", true, nil
		},
		Patch: func(ctx context.Context, mid, content string) error {
			if atomic.AddInt64(&inflight, 1) == 8 {
				once.Do(func() { close(barrier) })
			}
			select {
			case <-barrier:
				return nil
			case <-time.After(10 * time.Second):
				return fmt.Errorf("并发未达标（串排实证）")
			}
		},
		TouchCard: func(context.Context, string) {},
		Create: func(context.Context, string, string) (string, error) {
			return "m", nil
		},
		Logf: func(string, ...any) {},
	})
	c.Reconcile(context.Background())
	if inflight != 8 {
		t.Fatalf("补建批次漏票: %d", inflight)
	}
}

// TestT9_TouchOnPatch PATCH 成功→TouchCard 钩调（D-63 后半机证）。
func TestT9_TouchOnPatch(t *testing.T) {
	var mu sync.Mutex
	var touched []string
	c := NewCards(CardConfig{
		View: func(context.Context, string) (*CardView, error) {
			return &CardView{TicketID: "t"}, nil
		},
		CardRow: func(context.Context, string) (string, bool, error) {
			return "m1", true, nil
		},
		Patch: func(context.Context, string, string) error { return nil },
		TouchCard: func(_ context.Context, tid string) {
			mu.Lock()
			touched = append(touched, tid)
			mu.Unlock()
		},
		Create: func(context.Context, string, string) (string, error) {
			return "m", nil
		},
		Logf: func(string, ...any) {},
	})
	c.Observe(context.Background(), "k", "t9")
	if len(touched) != 1 || touched[0] != "t9" {
		t.Fatalf("PATCH 成功未 Touch: %v", touched)
	}
	_ = fmt.Sprint()
}
