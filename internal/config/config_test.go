package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoad_缺坏拒起（票0004 锚 7 首句）：配置必载（D-9）——缺省路径无文件
// →错带路径与 --config 指引（serve 据此拒起，业主照错文可直做）；
// 坏内容→解析错；好配置→载入。
func TestLoad_缺坏拒起(t *testing.T) {
	d := t.TempDir()
	missing := filepath.Join(d, "gateway.textproto")
	_, err := Load(missing)
	if err == nil {
		t.Fatal("缺文件应拒")
	}
	if !strings.Contains(err.Error(), missing) || !strings.Contains(err.Error(), "--config") {
		t.Fatalf("拒起错应带路径与指引: %v", err)
	}

	bad := filepath.Join(d, "bad.textproto")
	if err := os.WriteFile(bad, []byte("not_a_field: {"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil {
		t.Fatal("坏内容应拒")
	}

	ok := filepath.Join(d, "ok.textproto")
	if err := os.WriteFile(ok, []byte("default_attempt_cap: 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(ok)
	if err != nil || c.GetDefaultAttemptCap() != 1 {
		t.Fatalf("好配置应载: %v %+v", err, c)
	}
}

// TestProdGatewayConfig_CeoKinds 票26 钉锚：生产 gateway.textproto 的
// ceo_spawn_kinds 与 ceo_events 双列同含六 kind——reject_limit/budget_frozen/
// shells_exhausted/deploy_failed（决策77：部署失败坏消息入化身判决域）＋
// defer_limit/feishu_inbound。删任一列任一种子行→红（条款级对抗锚）。
func TestProdGatewayConfig_CeoKinds(t *testing.T) {
	c, err := Load(filepath.Join("..", "..", "config", "gateway.textproto"))
	if err != nil {
		t.Fatalf("生产配置拒载: %v", err)
	}
	spawn, ev := map[string]bool{}, map[string]bool{}
	for _, k := range c.GetCeoSpawnKinds() {
		spawn[k] = true
	}
	for _, k := range c.GetCeoEvents() {
		ev[k] = true
	}
	// T34：无头化身触发集钦定缩到两种（feishu_inbound/requirement）——
	// 异常类事件不再唤起 CEO（v2 §3.1/§3.3-E）。
	for _, k := range []string{"feishu_inbound", "requirement"} {
		if !spawn[k] {
			t.Fatalf("ceo_spawn_kinds 缺 %q", k)
		}
		if !ev[k] {
			t.Fatalf("ceo_events 缺 %q", k)
		}
	}
	if len(c.GetCeoSpawnKinds()) != 2 {
		t.Fatalf("ceo_spawn_kinds 应恰为两种: %v", c.GetCeoSpawnKinds())
	}
}
