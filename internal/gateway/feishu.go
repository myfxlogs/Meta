package gateway

import (
	"context"
	"strings"

	"meta/internal/db"
	"meta/internal/feishu"
	pbv1 "meta/internal/pb/metav1"
)

// FeishuCredName secret 表行名（业主种子动作：`meta secret put
// secret.feishu.app`；ACL 空＝不授任何席——网关以 vault.key 自解，§1.6）。
const FeishuCredName = "secret.feishu.app"

// FeishuInbound 飞书入站馈口（0011 泵接口消费端）：owner 消息→
// feishu_inbound 事件行（ticket_id 空串行）＋emit——泵按 ceo_spawn_kinds
// 唤起无头化身；provenance=feishu:<open_id>（D9 链戳起点，spawn 沿此戳）。
func (g *Gateway) FeishuInbound(senderOpenID, text, parentID string) {
	prov := "feishu:" + senderOpenID
	ctx := context.Background()
	_ = g.database.AppendEvent(ctx, "", "feishu_inbound", db.Event{
		Actor: "feishu", Note: text, Provenance: prov})
	g.emit(Event{Kind: "feishu_inbound", Note: text, Provenance: prov})
	// T28① 回复线程挂票：parent_id→feishu_card 反查父票→原句挂票内
	// 留言（回复即指向，不经票号匹配）。
	if parentID != "" {
		if hit, err := g.database.TicketByCardMsg(ctx, parentID); err == nil && hit != "" {
			_, _ = g.database.AppendMsg(ctx, hit, prov, text)
			_ = g.database.AppendEvent(ctx, hit, "msg_posted", db.Event{
				Actor: "gateway", Note: "飞书回复挂票 " + prov + " parent=" + parentID})
			return
		}
	}
	// T22 评注挂票（T28②③ 修配）：入站文本含在途票号→原句挂票内留言。
	// 最长匹配（0034-p1 提及不被 0034 歧义弃挂）＋边界匹配（长数字内嵌
	// 防误挂）；等长异号歧义不挂——留 CEO 入站径裁决。
	if ts, err := g.database.NonTerminalTickets(ctx); err == nil {
		var ids []string
		for _, t := range ts {
			ids = append(ids, t.ID)
		}
		if hit := ticketHit(text, ids); hit != "" {
			_, _ = g.database.AppendMsg(ctx, hit, prov, text)
			_ = g.database.AppendEvent(ctx, hit, "msg_posted", db.Event{
				Actor: "gateway", Note: "飞书评注挂票 " + prov})
		}
	}
}

// ticketHit 票号挂票匹配（T28②③，纯函数锚）：边界合法命中中取最长票号；
// 等长异号＝歧义弃挂（""）。
func ticketHit(text string, ids []string) string {
	best, amb := "", false
	for _, id := range ids {
		if id == "" || !containsBounded(text, id) {
			continue
		}
		switch {
		case len(id) > len(best):
			best, amb = id, false
		case len(id) == len(best) && id != best:
			amb = true
		}
	}
	if amb {
		return ""
	}
	return best
}

// containsBounded 边界匹配（T28③）：票号前后不得接字母/数字/连字符
// （连字符在内＝"0034-p1" 中 "0034" 段仍算提及——歧义由最长匹配消解；
// 内嵌长数字如 "1230034" 前接数字判误挂剔除）。
func containsBounded(text, id string) bool {
	for off := 0; ; {
		i := strings.Index(text[off:], id)
		if i < 0 {
			return false
		}
		i += off
		var b, a byte
		if i > 0 {
			b = text[i-1]
		}
		if i+len(id) < len(text) {
			a = text[i+len(id)]
		}
		if !isIDChar(b) && !isIDChar(a) {
			return true
		}
		off = i + 1
	}
}

func isIDChar(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-'
}

// FeishuChannel 凭证自解＋通道装配（不起环——测试/核检径可直调）。
// 行缺/解密败/格式坏→feishu_cred_bad 事件行＋emit＋返回 nil
// （fail loud；对抗锚 b 提名行＝本早退分支，删则坏凭证通道照启）。
func (g *Gateway) FeishuChannel(ctx context.Context) *feishu.Channel {
	cred, ok := g.feishuCred(ctx)
	if !ok {
		note := "secret.feishu.app 行缺/解密败/格式坏——飞书通道不启"
		_ = g.database.AppendEvent(ctx, "", "feishu_cred_bad", db.Event{
			Actor: "gateway", Note: note})
		g.emit(Event{Kind: "feishu_cred_bad", Note: note})
		return nil
	}
	return feishu.New(feishu.Config{Cred: cred, Feed: g.FeishuInbound,
		Ident: func() string { // T9/D-66：回执接单者身份＝CEO 化身壳
			if r, _ := g.database.ActiveRunForRole(ctx, "CEO"); r != nil && r.Shell != "" {
				return "CEO·" + r.Shell
			}
			return "CEO"
		}})
}

// StartFeishu 装配并起双环（serve 径）：入站 WS 环（有界退避）＋
// 出站观察环（订进程内事件口，ceo_say/ceo_last_resort→owner chat）。
// 环随 ctx/g.done 收；凭证坏时通道不启已在 FeishuChannel 报过。
// 出站订户**同步注册**（先订阅后起泵——经验库 2026-11-01-订阅在
// goroutine里注册；同 ceoPump 形制 gateway.go，防首拍 ceo_say 丢窗）。
func (g *Gateway) StartFeishu(ctx context.Context) *feishu.Channel {
	ch := g.FeishuChannel(ctx)
	if ch == nil {
		return nil
	}
	go ch.Run(ctx)
	g.wg.Add(1)
	sub := g.subscribe()
	go func() { defer g.wg.Done(); g.feishuOutbound(ctx, ch, sub) }()
	return ch
}

// feishuCred 网关自解凭证（不走 Secret RPC——那是席位径，本行 ACL 空不授席）。
// vault.key 缺载/行缺/解密败/格式坏一律 false。
func (g *Gateway) feishuCred(ctx context.Context) (*pbv1.FeishuApp, bool) {
	if g.vaultKey == nil {
		return nil, false
	}
	blob, ok, err := g.database.SecretBlob(ctx, FeishuCredName)
	if err != nil || !ok {
		return nil, false
	}
	plain, err := db.DecryptSecret(g.vaultKey, blob)
	if err != nil {
		return nil, false
	}
	app, err := feishu.ParseCred(plain)
	if err != nil {
		return nil, false
	}
	return app, true
}

// feishuOutbound 出站消费环：订户道由 StartFeishu 同步注册后传入
// （注册不在消费者 goroutine 内——首事件丢窗）；逐事件喂通道；
// 随 ctx/g.done 收（网关退场环停）。
func (g *Gateway) feishuOutbound(ctx context.Context, ch *feishu.Channel, evCh chan Event) {
	defer g.unsubscribe(evCh)
	for {
		select {
		case e := <-evCh:
			ch.Observe(e.Kind, e.Note)
		case <-ctx.Done():
			return
		case <-g.done:
			return
		}
	}
}
