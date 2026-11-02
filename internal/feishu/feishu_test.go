package feishu

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	pbv1 "meta/internal/pb/metav1"
)

// captured 伪 send/feed 捕获面（锚 2/4：伪事件单测的捕器）。
type captured struct {
	mu     sync.Mutex
	sents  []string // "chatID|text"
	feeds  []string // "sender|text"
	states []string
}

func (c *captured) send(_ context.Context, chatID, text string) error {
	c.mu.Lock()
	c.sents = append(c.sents, chatID+"|"+text)
	c.mu.Unlock()
	return nil
}

func (c *captured) feed(sender, text string) {
	c.mu.Lock()
	c.feeds = append(c.feeds, sender+"|"+text)
	c.mu.Unlock()
}

func (c *captured) nsent() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.sents)
}

func testCred() *pbv1.FeishuApp {
	return &pbv1.FeishuApp{AppId: "cli_a", AppSecret: "s",
		OwnerOpenId: "ou_owner", ChatId: "oc_owner"}
}

// TestHandle_Owner 锚2 正径＋对抗锚 a 验证面：owner 消息→Feed 馈达＋
// 机械回执；非 owner→拒答且绝不进泵（等值比较行翻＝本条红）。
func TestHandle_Owner(t *testing.T) {
	cap := &captured{}
	ch := New(Config{Cred: testCred(), Send: cap.send, Feed: cap.feed,
		Logf: func(string, ...any) {}})

	ch.handle("ou_owner", "oc_x", `{"text":"做个新功能"}`)
	if len(cap.feeds) != 1 || cap.feeds[0] != "ou_owner|做个新功能" {
		t.Fatalf("owner 消息未馈泵: %v", cap.feeds)
	}
	if cap.nsent() != 1 || cap.sents[0] != "oc_x|"+receiptIn {
		t.Fatalf("机械回执未发/发错: %v", cap.sents)
	}

	// 非业主：拒答回执，馈口零调用（对抗锚 a：白名单等值比较翻→本条红）。
	ch.handle("ou_stranger", "oc_y", `{"text":"混进来"}`)
	if len(cap.feeds) != 1 {
		t.Fatalf("非业主进了泵: %v", cap.feeds)
	}
	if cap.nsent() != 2 || cap.sents[1] != "oc_y|"+receiptReject {
		t.Fatalf("非业主拒答未发: %v", cap.sents)
	}

	// 空正文/非 text 型：静默忽略（无回执无馈）。
	ch.handle("ou_owner", "oc_x", `{"text":"   "}`)
	ch.handle("ou_owner", "oc_x", `{"image_key":"img_x"}`)
	if len(cap.feeds) != 1 || cap.nsent() != 2 {
		t.Fatalf("空/非文本未静默: feeds=%v sents=%v", cap.feeds, cap.sents)
	}
}

// TestParseCred 锚3：双格式 fixture——proto blob／KEY=VALUE 行；
// 坏 blob 与缺键各归错。
func TestParseCred(t *testing.T) {
	blob, err := proto.Marshal(testCred())
	if err != nil {
		t.Fatal(err)
	}
	app, err := ParseCred(blob)
	if err != nil || app.AppId != "cli_a" || app.OwnerOpenId != "ou_owner" || app.ChatId != "oc_owner" {
		t.Fatalf("proto blob 解析败: %v %+v", err, app)
	}

	kv := "# 注释行\nFEISHU_MANAGE_APP_ID=cli_k\nFEISHU_MANAGE_APP_SECRET=sk\n" +
		"FEISHU_MANAGE_OWNER_OPEN_ID=ou_k\nFEISHU_MANAGE_CHAT_ID=oc_k\n"
	app, err = ParseCred([]byte(kv))
	if err != nil || app.AppId != "cli_k" || app.OwnerOpenId != "ou_k" || app.ChatId != "oc_k" {
		t.Fatalf("KEY=VALUE 解析败: %v %+v", err, app)
	}

	for _, bad := range [][]byte{
		[]byte("garbage-no-keys"),
		[]byte("FEISHU_MANAGE_APP_ID=cli_only\n"),
		{0x0a, 0x03, 'a', 'b', 'c'}, // 形似 wire 但字段不全
	} {
		if _, err := ParseCred(bad); err == nil {
			t.Fatalf("坏凭证未拒: %q", bad)
		}
	}
}

// TestObserve 锚4：say 消费伪 sender 捕文本——ceo_say/ceo_last_resort
// →owner chat；其余 kind 过手不动作；败仅记日志不炸。
func TestObserve(t *testing.T) {
	cap := &captured{}
	failSend := func(_ context.Context, _, _ string) error { return errors.New("net down") }
	ch := New(Config{Cred: testCred(), Send: cap.send, Logf: func(string, ...any) {}})

	ch.Observe("ceo_say", "票已部署")
	ch.Observe("ceo_last_resort", "无头唤起连败")
	ch.Observe("phase:claim", "不该发")
	ch.Observe("feishu_inbound", "也不该发")
	if cap.nsent() != 2 || cap.sents[0] != "oc_owner|票已部署" || cap.sents[1] != "oc_owner|无头唤起连败" {
		t.Fatalf("出站消费错: %v", cap.sents)
	}

	// 出站失败不炸环：Observe 正常返回。
	ch2 := New(Config{Cred: testCred(), Send: failSend,
		Logf: func(string, ...any) {}})
	ch2.Observe("ceo_say", "败也过了")
}

// TestRun_BoundedReconnect 锚5：WS Start 返回→退避重起**有界**断言——
// 连败达 MaxRestart 即放弃（次数界）＋退避封 BackoffCap（上界），非无界续订。
func TestRun_BoundedReconnect(t *testing.T) {
	var calls int
	var mu sync.Mutex
	start := func(ctx context.Context) error {
		mu.Lock()
		calls++
		mu.Unlock()
		return errors.New("ws 终态")
	}
	ch := New(Config{Cred: testCred(), StartWS: start, Logf: func(string, ...any) {},
		BackoffBase: time.Millisecond, BackoffCap: 4 * time.Millisecond, MaxRestart: 3})
	began := time.Now()
	ch.Run(context.Background())
	mu.Lock()
	defer mu.Unlock()
	if calls != 3 {
		t.Fatalf("连败未达次数界即停/越界重试: calls=%d want 3", calls)
	}
	if time.Since(began) > 3*time.Second {
		t.Fatalf("退避未封上界: %v", time.Since(began))
	}
}

// TestRun_LongLivedResets 锚5 侧证：上轮存活>5min＝瞬断归零连败（长跑后
// 被踢不算终态恶化）；无法真睡 5min——以 liveness 缝伪时钟代。
// 本票实现以 time.Since 判活>5min——测试改走短存活连败语义已在上条坐实；
// 此条只断 ctx 取消即退（不空转）。
func TestRun_CtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := New(Config{Cred: testCred(), Logf: func(string, ...any) {},
		BackoffBase: time.Hour, // 取消前若睡在退避里也要立刻醒
		StartWS: func(ctx context.Context) error {
			cancel()
			return ctx.Err()
		}})
	done := make(chan struct{})
	go func() { ch.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ctx 取消未退 Run")
	}
}
