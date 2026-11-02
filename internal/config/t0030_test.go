package config

import (
	"path/filepath"
	"testing"
)

// TestProdGatewayConfig_T30 票30 钉锚：生产 gateway.textproto 的巡检订阅
// 种子与便宜过滤壳行必须能解析出值。
func TestProdGatewayConfig_T30(t *testing.T) {
	c, err := Load(filepath.Join("..", "..", "config", "gateway.textproto"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Subscribe) < 6 {
		t.Fatalf("巡检订阅种子行数不足: %d", len(c.Subscribe))
	}
	kinds := map[string]string{}
	for _, s := range c.Subscribe {
		kinds[s.Kind] = s.Role
	}
	for _, k := range []string{"exec_denied", "phase:deploy_failed", "spawn_blocked",
		"phase:reject", "merge_conflict", "patrol_tick"} {
		if kinds[k] != "巡检" {
			t.Fatalf("订阅种子缺/错 %s→%s", k, kinds[k])
		}
	}
	if c.FilterShell == nil || c.FilterShell.Name != "opencode" {
		t.Fatalf("filter_shell 未配或错: %+v", c.FilterShell)
	}
	var patrolShell bool
	for _, s := range c.Shell {
		if s.Role == "巡检" {
			patrolShell = true
		}
	}
	if !patrolShell {
		t.Fatal("巡检角色无壳登记行")
	}
}
