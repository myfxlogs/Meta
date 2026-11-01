package gateway

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// feishuCfg 票0013 测试配置：CEO 面 fixture＋feishu_inbound 入 spawn/放行 kind 集。
func feishuCfg(dir string) Config {
	c := ceoTestCfg(dir, true)
	c.CeoSpawnKinds["feishu_inbound"] = true
	c.CeoEvents["feishu_inbound"] = true
	return c
}

// seedVault 网关建仓前先落 vault.key——New 起才载得进 vaultKey（自解径前置）。
func seedVault(t *testing.T, runDir string) []byte {
	t.Helper()
	k, err := db.LoadOrCreateVaultKey(filepath.Join(runDir, "vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// TestFeishuInbound 锚2：owner 消息→feishu_inbound 事件行（ticket_id 空串、
// 戳=feishu:<open_id>）＋泵馈达——spawn_kind 触发无头唤起，run 盖同戳
// （§1.3/§2 链：feishu 入站唤起的 CEO 不盖 system_event）。
func TestFeishuInbound(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "ceo")
	t.Setenv("META_ADOPT_GRACE", "30")
	g, _ := newTestGateway(t, func(c *Config) { *c = feishuCfg(filepath.Dir(c.DBPath)) })
	ctx := context.Background()

	g.FeishuInbound("ou_owner1", "做个新功能")
	ev := hasEvent(t, g, "", "feishu_inbound")
	if ev == nil {
		t.Fatal("feishu_inbound 行未落账")
	}
	if ev.TicketID != "" || ev.Provenance != "feishu:ou_owner1" {
		t.Fatalf("入站行形错: %+v", ev)
	}
	// 泵馈达实证：无头唤起发生且 run 戳＝feishu:ou_owner1（非 system_event）。
	waitFor2(t, 15*time.Second, func() bool {
		r, _ := g.DB().ActiveRunForRole(ctx, "CEO")
		return r != nil && r.Provenance == "feishu:ou_owner1"
	})
	r, _ := g.DB().ActiveRunForRole(ctx, "CEO")
	if r != nil {
		g.killRun(g.bindRun(r))
	}
}

// TestFeishuCredBad 锚3＋对抗锚 b：坏凭证→feishu_cred_bad 事件行＋通道不启
// （早退分支删→nil 断言红＝坏凭证通道照启）。
func TestFeishuCredBad(t *testing.T) {
	ctx := context.Background()

	// 情形一：无 vault.key/行缺——FeishuChannel 返回 nil＋cred_bad 行。
	g, _ := newTestGateway(t, func(c *Config) { *c = feishuCfg(filepath.Dir(c.DBPath)) })
	if ch := g.FeishuChannel(ctx); ch != nil {
		t.Fatal("凭证行缺仍出通道")
	}
	if hasEvent(t, g, "", "feishu_cred_bad") == nil {
		t.Fatal("行缺未发 feishu_cred_bad")
	}

	// 情形二：行在但密文里是垃圾——同径 fail loud。
	g2, _ := newTestGateway(t, func(c *Config) {
		*c = feishuCfg(filepath.Dir(c.DBPath))
		seedVault(t, c.RunDir)
	})
	ct, err := db.EncryptSecret(g2.vaultKey, []byte("garbage-not-a-cred"))
	if err != nil {
		t.Fatal(err)
	}
	if err := g2.DB().PutSecret(ctx, FeishuCredName, ct, nil); err != nil {
		t.Fatal(err)
	}
	if ch := g2.FeishuChannel(ctx); ch != nil {
		t.Fatal("坏凭证仍出通道（cred_bad 早退被删）")
	}
	if hasEvent(t, g2, "", "feishu_cred_bad") == nil {
		t.Fatal("坏凭证未发 feishu_cred_bad")
	}

	// 情形三：proto blob 好凭证→通道装配（不起环——真 WS 非单测面）。
	blob, err := proto.Marshal(&pbv1.FeishuApp{AppId: "cli_a", AppSecret: "s",
		OwnerOpenId: "ou_o", ChatId: "oc_o"})
	if err != nil {
		t.Fatal(err)
	}
	ct, err = db.EncryptSecret(g2.vaultKey, blob)
	if err != nil {
		t.Fatal(err)
	}
	if err := g2.DB().PutSecret(ctx, FeishuCredName, ct, nil); err != nil {
		t.Fatal(err)
	}
	if ch := g2.FeishuChannel(ctx); ch == nil {
		t.Fatal("好凭证未装通道")
	}
}

// TestFeishuCredBad_KeyValue 锚3 侧证：KEY=VALUE 兜底格式入库亦解得动。
func TestFeishuCredBad_KeyValue(t *testing.T) {
	ctx := context.Background()
	g, _ := newTestGateway(t, func(c *Config) {
		*c = feishuCfg(filepath.Dir(c.DBPath))
		seedVault(t, c.RunDir)
	})
	plain := "FEISHU_MANAGE_APP_ID=cli_k\nFEISHU_MANAGE_APP_SECRET=sk\n" +
		"FEISHU_MANAGE_OWNER_OPEN_ID=ou_k\nFEISHU_MANAGE_CHAT_ID=oc_k\n"
	ct, err := db.EncryptSecret(g.vaultKey, []byte(plain))
	if err != nil {
		t.Fatal(err)
	}
	if err := g.DB().PutSecret(ctx, FeishuCredName, ct, nil); err != nil {
		t.Fatal(err)
	}
	app, ok := g.feishuCred(ctx)
	if !ok || app.AppId != "cli_k" || app.OwnerOpenId != "ou_k" {
		t.Fatalf("KEY=VALUE 凭证未解: %v %+v", ok, app)
	}
}

// TestStartFeishu_SyncSubscribe 返修锚（接替席·经验库命中）：StartFeishu
// 返回＝出站订户已同步注册（先订阅后起泵——经验库 2026-11-01-订阅在
// goroutine里注册；注册挪回消费者 goroutine 内＝本条红：返回时订户快照
// 少一，首拍 ceo_say 丢窗）。ctx 先杀＝Run/出站环即退不碰网。
func TestStartFeishu_SyncSubscribe(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = feishuCfg(filepath.Dir(c.DBPath))
		seedVault(t, c.RunDir)
	})
	blob, err := proto.Marshal(&pbv1.FeishuApp{AppId: "cli_a", AppSecret: "s",
		OwnerOpenId: "ou_o", ChatId: "oc_o"})
	if err != nil {
		t.Fatal(err)
	}
	ct, err := db.EncryptSecret(g.vaultKey, blob)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.DB().PutSecret(context.Background(), FeishuCredName, ct, nil); err != nil {
		t.Fatal(err)
	}

	dead, cancel := context.WithCancel(context.Background())
	g.mu.Lock()
	before := len(g.observers)
	g.mu.Unlock()
	// 活 ctx 装配（死 ctx 连凭证 DB 读一并杀——cred_bad 路径混测）；断言后
	// 立即 cancel：Run 环/出站环即收，Run 首轮伪凭证拨号随 ctx 中断无副作用。
	if ch := g.StartFeishu(dead); ch == nil {
		cancel()
		t.Fatal("好凭证未起通道")
	}
	g.mu.Lock()
	after := len(g.observers)
	g.mu.Unlock()
	cancel()
	if after != before+1 {
		t.Fatalf("StartFeishu 返回时出站订户未注册（先订阅后起泵被破）: %d→%d", before, after)
	}
}
