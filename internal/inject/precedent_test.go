package inject

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	pbv1 "meta/internal/pb/metav1"
)

// tmpPrecedentRoot 造注入根：roles/ 施工|审计 两卡＋给定两库文件（相对路径→正文）。
func tmpPrecedentRoot(t *testing.T, libs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for _, role := range []string{"施工", "审计"} {
		p := filepath.Join(root, "roles", role+".md")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# 角色卡\n"+role+" fixture 卡\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for rel, body := range libs {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func tmpSpec(goal, scope string) *pbv1.TicketSpec {
	return &pbv1.TicketSpec{
		Goal: goal, Why: "w", Scope: scope, OutOfScope: "o",
		Refs: "r", Anchors: "a", Verify: "v", Deliver: "d",
	}
}

func tmpInput(root string, role string, spec *pbv1.TicketSpec) Input {
	return Input{Root: root, Role: role, TicketID: "0037", Title: "t", Spec: spec}
}

// sentinelLibs 哨兵/JSON 命中样本（对齐对抗锚命名）。
var sentinelLibs = map[string]string{
	"docs/经验库/2026-11-02-哨兵三壳失明谱-JSON包裹与行合并.md": "# 哨兵失明三壳谱系\n哨兵行被 JSON 包裹或长行腰斩后外壳失明，教训：双发兜底。\n\n后段不扫。\n",
	"docs/经验库/2026-11-01-全不相干坑.md":              "沉积记录：配色与版式旧事。\n",
}

// TestPrecedent_命中插段 命中分支＋对抗锚：施工票 Goal/Scope 含「哨兵」「JSON」
// →产物必含 `## 判例前置` 段且段内含命中坑档路径行。len(hits)>0 翻成 ==0 或
// 删插段调用→本用例必红。
func TestPrecedent_命中插段(t *testing.T) {
	root := tmpPrecedentRoot(t, sentinelLibs)
	spec := tmpSpec("修复哨兵在 JSON 包裹下丢行", "internal/inject 哨兵输出径")
	out, err := Assemble(tmpInput(root, "施工", spec))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## 判例前置") {
		t.Fatalf("判例前置段缺失:\n%s", out)
	}
	want := "- docs/经验库/2026-11-02-哨兵三壳失明谱-JSON包裹与行合并.md — "
	if !strings.Contains(out, want) {
		t.Fatalf("命中坑档路径行缺失 %q:\n%s", want, out)
	}
	// 段落在五、技能库索引之后、六、工作目录之前。
	i5 := strings.Index(out, "## 五、技能库索引")
	ip := strings.Index(out, "## 判例前置")
	i6 := strings.Index(out, "## 六、工作目录")
	if !(i5 >= 0 && ip > i5 && i6 > ip) {
		t.Fatalf("判例前置段位置错（应在五/六之间）: i5=%d ip=%d i6=%d", i5, ip, i6)
	}
	// 无命中档不得进段。
	if strings.Contains(out, "- docs/经验库/2026-11-01-全不相干坑.md") {
		t.Fatal("零分档误入判例前置段")
	}
}

// TestPrecedent_无命中无段 无命中分支：票面关键词与两库零重叠→整段不出现。
func TestPrecedent_无命中无段(t *testing.T) {
	root := tmpPrecedentRoot(t, sentinelLibs)
	spec := tmpSpec("网关端口微调", "alpha beta")
	out, err := Assemble(tmpInput(root, "施工", spec))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "判例前置") {
		t.Fatalf("无命中仍插段:\n%s", out)
	}
}

// TestPrecedent_超三截断 >3 分支：5 档命中（跨两库）→段内恰 3 条命中行
// （评分降序、同分路径序取 top3）＋钦定索引指针行。
func TestPrecedent_超三截断(t *testing.T) {
	root := tmpPrecedentRoot(t, map[string]string{
		"docs/经验库/e1.md": "哨兵 丢行 修复\n", // 3 分
		"docs/经验库/e2.md": "哨兵 丢行\n",    // 2 分
		"docs/经验库/e3.md": "哨兵\n",       // 1 分（路径序输给 e4/e5）
		"docs/技能库/e4.md": "丢行\n",       // 1 分
		"docs/技能库/e5.md": "修复\n",       // 1 分
	})
	spec := tmpSpec("修复哨兵。丢行。", "范围无关字")
	out, err := Assemble(tmpInput(root, "施工", spec))
	if err != nil {
		t.Fatal(err)
	}
	sec := out
	if i := strings.Index(sec, "## 判例前置"); i >= 0 {
		sec = sec[i:]
	} else {
		t.Fatalf("判例前置段缺失:\n%s", out)
	}
	if i := strings.Index(sec, "## 六、"); i >= 0 {
		sec = sec[:i]
	}
	if n := strings.Count(sec, "- docs/"); n != 3 {
		t.Fatalf("段内命中行应为恰 3 条，实得 %d:\n%s", n, sec)
	}
	// top3＝e1(3)、e2(2)、e4(1，同分路径序技<e经)。
	for _, p := range []string{"docs/经验库/e1.md", "docs/经验库/e2.md", "docs/技能库/e4.md"} {
		if !strings.Contains(sec, "- "+p+" — ") {
			t.Fatalf("top3 缺 %s:\n%s", p, sec)
		}
	}
	for _, p := range []string{"docs/经验库/e3.md", "docs/技能库/e5.md"} {
		if strings.Contains(sec, p) {
			t.Fatalf("截断外档 %s 混入:\n%s", p, sec)
		}
	}
	if !strings.Contains(sec, "两库全量索引") || !strings.Contains(sec, "四/五") {
		t.Fatalf("索引指针行缺失/文本不符:\n%s", sec)
	}
}

// TestPrecedent_非施工不插 Role 门控锚：审计同 spec 产物不含判例前置
// （非施工 Role 产物字节不变禁线）。
func TestPrecedent_非施工不插(t *testing.T) {
	root := tmpPrecedentRoot(t, sentinelLibs)
	spec := tmpSpec("修复哨兵在 JSON 包裹下丢行", "internal/inject 哨兵输出径")
	out, err := Assemble(tmpInput(root, "审计", spec))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "判例前置") {
		t.Fatalf("非施工 Role 产物含判例前置:\n%s", out)
	}
}
