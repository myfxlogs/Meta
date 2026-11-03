package feishu

// 票 T38 锚测：九相位图标上卡（业主要求）＋哈希闸不误刷。

import (
	"strings"
	"testing"
)

// 锚1：九相位（含 blocked）各一图标——相位行带图标前缀；表外回空降级。
func TestT38_PhaseIcons(t *testing.T) {
	want := map[string]string{
		"已立": "📌", "设计中": "📐", "已派": "📋", "施工中": "🔄",
		"待审": "⏳", "审计中": "🔍", "待部署": "📦", "部署中": "🚀",
		"完成": "✅", "作废": "🗑️", "blocked": "🛑",
	}
	for phase, icon := range want {
		s := renderCard(testView("t38", phase, "", ""))
		if !strings.Contains(s, "**相位**："+icon+phase) {
			t.Fatalf("相位 %s 缺图标 %s: %s", phase, icon, s)
		}
	}
	if s := renderCard(testView("t38", "未知态", "", "")); !strings.Contains(s, "**相位**：未知态") {
		t.Fatalf("表外相位应裸行降级: %s", s)
	}
}

// 锚2：哈希闸零误刷——同相位同视图重渲逐字节同（图标确定性不漂移）。
func TestT38_IconDeterministic(t *testing.T) {
	v := testView("t38", "blocked", "", "")
	if renderCard(v) != renderCard(v) {
		t.Fatal("同视图重渲不一致——图标应确定性")
	}
}
