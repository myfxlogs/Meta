package inject

import (
	"strings"
	"testing"
)

// TestT30_MsgsSection 票内留言节锚：Msgs 行进注入前缀「票内留言」节；
// 空→「（空）」。删该节拼接→本锚红。
func TestT30_MsgsSection(t *testing.T) {
	in := fixtureInput(t)
	in.Msgs = []string{"- [ceo] 改方案X（11-02 09:00）", "- [patrol] 信号注记"}
	out, err := Assemble(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## 票内留言") ||
		!strings.Contains(out, "[ceo] 改方案X") ||
		!strings.Contains(out, "[patrol] 信号注记") {
		t.Fatalf("票内留言节缺失/漏行:\n%s", out)
	}
	out2, err := Assemble(fixtureInput(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out2, "## 票内留言") {
		t.Fatal("空留言时节头也该在")
	}
}
