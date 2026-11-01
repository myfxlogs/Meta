// Package feishu 飞书双向通道（票0013，禁区#16：飞书全 I/O 归网关一进程三件）——
// larkws WS 入站（im.message.receive_v1→open_id==owner 白名单→inbound 馈口→
// CEO 泵）＋出站（ceo_say/ceo_last_resort→owner chat、入站机械回执）＋
// 凭证解析（cred.go）。本包是 lark API 的边界：JSON 止于本包即转 proto/
// 原生串（禁区#3 豁免面）；借形 /opt/meta-archive/mbusd feishu-ingress
// （白名单/退避/静默忽略非 text 型），不借码——env 读文件段换成 secret 自解。
package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	pbv1 "meta/internal/pb/metav1"
)

const (
	// receiptIn 入站机械回执——通道即发不等 CEO（对话感第一拍，令§1.6）。
	receiptIn = "收到，这就去办——办完回你"
	// receiptReject 白名单外拒答（同 v1 逐字——出站模板钦定件）。
	receiptReject = "不好意思，这条通道只服务老板本人——您的话我带到了"

	sendTimeout = 20 * time.Second
)

// Config 通道装配参数；Send/StartWS 为注入缝（缺省＝lark REST/larkws 实件）。
type Config struct {
	Cred *pbv1.FeishuApp
	// Feed 入站馈口（网关注入＝0011 泵接口消费端：落 feishu_inbound 事件行＋emit）。
	Feed func(senderOpenID, text string)
	// Send 出站发送（chatID→文本）；nil＝lark REST 实发。
	Send func(ctx context.Context, chatID, text string) error
	// StartWS 一轮 WS 入站生命周期；nil＝larkws 实连。返回＝终态（SDK 内建
	// 重连已弃/认证失效/ctx 取消），由 Run 外层有界退避重起。
	StartWS     func(ctx context.Context) error
	Logf        func(format string, args ...any)
	BackoffBase time.Duration // 退避基数，缺省 1s
	BackoffCap  time.Duration // 退避上界，缺省 60s
	MaxRestart  int           // 连败上界（有界重试链 R2），缺省 5
}

// Channel 飞书通道：入站 WS 环＋出站消费步共持同一 lark client（设计 §1.6）。
type Channel struct {
	cfg  Config
	api  *lark.Client
	hand *dispatcher.EventDispatcher
}

// New 装配通道（不起环——Run/Consume/Observe 由调用方驱动）。
func New(cfg Config) *Channel {
	c := &Channel{cfg: cfg}
	if c.cfg.Logf == nil {
		c.cfg.Logf = func(f string, a ...any) { log.Printf("feishu: "+f, a...) }
	}
	if c.cfg.BackoffBase <= 0 {
		c.cfg.BackoffBase = time.Second
	}
	if c.cfg.BackoffCap <= 0 {
		c.cfg.BackoffCap = 60 * time.Second
	}
	if c.cfg.MaxRestart <= 0 {
		c.cfg.MaxRestart = 5
	}
	if cfg.Cred != nil {
		c.api = lark.NewClient(cfg.Cred.AppId, cfg.Cred.AppSecret)
	}
	if c.cfg.Send == nil {
		c.cfg.Send = c.larkSend
	}
	if c.cfg.StartWS == nil {
		c.cfg.StartWS = c.larkStartWS
	}
	return c
}

// ---------- 入站（WS→白名单→馈口；回执机械即发不等 CEO） ----------

// handle 入站分诊（对抗锚 a 提名行＝白名单等值比较——翻成 !=/恒真则
// 非业主也进泵）：业主→Feed 入泵＋机械回执；非业主→礼貌回绝不进泵。
// 非 text 型（图/文件等）静默忽略（同 v1）。
func (c *Channel) handle(senderOpenID, chatID, contentJSON string) {
	if senderOpenID != c.cfg.Cred.OwnerOpenId {
		c.sendTo(chatID, receiptReject)
		return
	}
	text := extractText(contentJSON)
	if text == "" {
		return
	}
	if c.cfg.Feed != nil {
		c.cfg.Feed(senderOpenID, text)
	}
	c.sendTo(chatID, receiptIn)
}

// extractText 取 text 消息正文；非 text 型/坏 JSON 返回空（静默忽略同 v1）。
func extractText(contentJSON string) string {
	var content struct {
		Text string `json:"text"`
	}
	if contentJSON == "" || json.Unmarshal([]byte(contentJSON), &content) != nil {
		return ""
	}
	return strings.TrimSpace(content.Text)
}

// dispatcher lark 事件分诊器（懒建）：字段抽取出 Event 即喂 handle——
// lark JSON 止于本包（禁区#3 豁免面）。
func (c *Channel) dispatcher() *dispatcher.EventDispatcher {
	if c.hand == nil {
		c.hand = dispatcher.NewEventDispatcher("", "").
			OnP2MessageReceiveV1(func(_ context.Context, ev *larkim.P2MessageReceiveV1) error {
				sender, chat, content := "", "", ""
				if ev.Event != nil {
					if ev.Event.Sender != nil && ev.Event.Sender.SenderId != nil &&
						ev.Event.Sender.SenderId.OpenId != nil {
						sender = *ev.Event.Sender.SenderId.OpenId
					}
					if ev.Event.Message != nil {
						if ev.Event.Message.ChatId != nil {
							chat = *ev.Event.Message.ChatId
						}
						if ev.Event.Message.Content != nil {
							content = *ev.Event.Message.Content
						}
					}
				}
				c.handle(sender, chat, content)
				return nil // nil＝SDK 按官方协议自动 ack
			})
	}
	return c.hand
}

// larkStartWS 实连一轮：WS client 每轮重建（SDK 内建 autoReconnect 默认开；
// Start 返回＝终态，外层退避重起是新一轮生命周期）。
func (c *Channel) larkStartWS(ctx context.Context) error {
	wsCli := larkws.NewClient(c.cfg.Cred.AppId, c.cfg.Cred.AppSecret,
		larkws.WithEventHandler(c.dispatcher()),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
		larkws.WithOnReady(func() { c.cfg.Logf("ws 入站就绪") }),
		larkws.WithOnReconnecting(func() { c.cfg.Logf("ws 重连中…") }),
		larkws.WithOnReconnected(func() { c.cfg.Logf("ws 已重连") }),
	)
	return wsCli.Start(ctx)
}

// Run 入站环：StartWS 返回＝终态→指数退避重起；连败达 MaxRestart→放弃返回
// （有界重试链 R2 边界条款——非无界续订）。上轮存活>5min 归零连败与退避
// （长跑后被踢＝网络瞬断非终态恶化）。ctx 取消即退。
func (c *Channel) Run(ctx context.Context) {
	backoff := c.cfg.BackoffBase
	fails := 0
	for {
		start := time.Now()
		err := c.cfg.StartWS(ctx)
		if ctx.Err() != nil {
			return
		}
		if time.Since(start) > 5*time.Minute {
			fails, backoff = 0, c.cfg.BackoffBase
		}
		fails++
		if fails >= c.cfg.MaxRestart {
			c.cfg.Logf("ws 连败 %d 达上界——入站通道停（出站 REST 面不受影响）: %v",
				fails, err)
			return
		}
		c.cfg.Logf("ws run 终态：%v（连败 %d/%d，%v 后重起）",
			err, fails, c.cfg.MaxRestart, backoff)
		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
		backoff *= 2
		if backoff > c.cfg.BackoffCap {
			backoff = c.cfg.BackoffCap
		}
	}
}

// ---------- 出站（ceo_say/ceo_last_resort→owner chat；败仅记日志不炸环） ----------

// Observe 出站消费步：网关事件观察口逐事件喂入——
// ceo_say（CEO 说话，§1.9）与 ceo_last_resort（无头唤起连败呼救，D3 径）
// →文本发 owner chat；其余 kind 过手不动作。
func (c *Channel) Observe(kind, note string) {
	switch kind {
	case "ceo_say", "ceo_last_resort":
	default:
		return
	}
	c.sendTo(c.cfg.Cred.ChatId, note)
}

// sendTo 出站一步：chatID 空＝回落 owner chat（凭证行钦定落点）；
// 20s 超时；失败仅记日志不炸环（事件环不被出站拖死）。
func (c *Channel) sendTo(chatID, text string) {
	if chatID == "" {
		chatID = c.cfg.Cred.ChatId
	}
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	if err := c.cfg.Send(ctx, chatID, text); err != nil {
		c.cfg.Logf("出站发送失败（仅记不炸环）chat=%s: %v", chatID, err)
	}
}

// larkSend 实发：im.v1 Message.Create（receive_id_type=chat_id，text 正文）。
func (c *Channel) larkSend(ctx context.Context, chatID, text string) error {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	resp, err := c.api.Im.V1.Message.Create(ctx, larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.CreateMessageV1ReceiveIDTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).MsgType("text").Content(string(body)).Build()).
		Build())
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("lark 发消息被拒: code=%d %s", resp.Code, resp.Msg)
	}
	return nil
}
