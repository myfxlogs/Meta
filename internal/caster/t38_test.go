package caster

// 票 T38 锚测：通知模板图标语义——blocked🛑（警报要像警报）、
// 部署 ✅/🚀、审计判决 ✅/⚠️。

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestT38_NotifyIcons(t *testing.T) {
	tb, err := LoadTable(filepath.Join("..", "..", "config", "caster.textproto"))
	if err != nil {
		t.Fatalf("生产路由表拒载: %v", err)
	}
	want := map[string]string{
		"blocked":      "🛑",
		"deploy_done":  "✅",
		"deploy_start": "🚀",
		"audit_pass":   "✅",
		"reject":       "⚠️",
	}
	seen := map[string]bool{}
	for _, r := range tb.Routes {
		icon, ok := want[r.Kind]
		if !ok {
			continue
		}
		for _, a := range r.Actions {
			n := a.GetNotify()
			if n == nil {
				continue
			}
			if !strings.Contains(n.Template, icon) {
				t.Fatalf("%s 通知模板缺图标 %s: %s", r.Kind, icon, n.Template)
			}
			seen[r.Kind] = true
		}
	}
	for k := range want {
		if !seen[k] {
			t.Fatalf("kind %s 无通知模板可核", k)
		}
	}
}
