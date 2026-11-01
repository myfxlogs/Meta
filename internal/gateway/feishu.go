package gateway

import (
	"context"

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
func (g *Gateway) FeishuInbound(senderOpenID, text string) {
	prov := "feishu:" + senderOpenID
	ctx := context.Background()
	_ = g.database.AppendEvent(ctx, "", "feishu_inbound", db.Event{
		Actor: "feishu", Note: text, Provenance: prov})
	g.emit(Event{Kind: "feishu_inbound", Note: text, Provenance: prov})
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
	return feishu.New(feishu.Config{Cred: cred, Feed: g.FeishuInbound})
}

// StartFeishu 装配并起双环（serve 径）：入站 WS 环（有界退避）＋
// 出站观察环（订进程内事件口，ceo_say/ceo_last_resort→owner chat）。
// 环随 ctx/g.done 收；凭证坏时通道不启已在 FeishuChannel 报过。
func (g *Gateway) StartFeishu(ctx context.Context) *feishu.Channel {
	ch := g.FeishuChannel(ctx)
	if ch == nil {
		return nil
	}
	go ch.Run(ctx)
	go g.feishuOutbound(ctx, ch)
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

// feishuOutbound 出站消费环：订进程内事件观察口逐事件喂通道；
// 随 ctx/g.done 收（网关退场环停）。
func (g *Gateway) feishuOutbound(ctx context.Context, ch *feishu.Channel) {
	evCh := g.Subscribe()
	defer g.Unsubscribe(evCh)
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
