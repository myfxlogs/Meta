package inject

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureInput(t *testing.T) Input {
	t.Helper()
	raw, err := os.ReadFile("testdata/ticket.md")
	if err != nil {
		t.Fatal(err)
	}
	spec, err := ParseTicketMD(raw)
	if err != nil {
		t.Fatalf("fixture 票面解析: %v", err)
	}
	return Input{
		Root:     "testdata/repo",
		Role:     "施工",
		TicketID: "9999",
		Title:    "fixture 票",
		Spec:     spec,
		Recipes:  []string{"recipe.md"},
		Now:      time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
	}
}

// TestAssemble_Content 注入内容锚（验收锚2＋对抗锚）：组装输出必含角色卡
// 关键行、票面八栏各栏值、两库索引文件名、配方内容——任一拼接行被删/翻，
// 对应对言必红。
func TestAssemble_Content(t *testing.T) {
	out, err := Assemble(fixtureInput(t))
	if err != nil {
		t.Fatal(err)
	}
	must := []string{
		"你是一个**施工实例**，被派来完成一张票", // 角色卡关键行
		"fixture 目标行",                       // 栏1
		"fixture 为什么行",                     // 栏2
		"fixture 范围甲",                       // 栏3
		"fixture 禁线行",                       // 栏4
		"fixture/引用.md",                      // 栏5
		"fixture 锚一",                         // 栏6
		"fixture check --all",                 // 栏7
		"t/9999-fixture",                      // 栏8
		"预算：fixture",                        // 附
		"检查命令：go test ./...",               // 配方
		"2026-11-01-坑甲.md",                   // 经验库索引
		"2026-11-01-坑乙.md",
		"技甲.md", // 技能库索引
	}
	for _, s := range must {
		if !strings.Contains(out, s) {
			t.Fatalf("注入前缀缺 %q:\n%s", s, out)
		}
	}
	// 索引只注文件名不注全文：经验库正文不许漏进前缀。
	if strings.Contains(out, "现象：坑甲") {
		t.Fatal("索引越权：经验库正文混入前缀")
	}
}

// TestAssemble_Verdicts 动态判决史拼入（驳回理由累积，重做者必读）。
func TestAssemble_Verdicts(t *testing.T) {
	in := fixtureInput(t)
	in.Verdicts = []string{"- reject: 审计驳回理由 fixture", "- yield: 受阻 fixture"}
	out, err := Assemble(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range in.Verdicts {
		if !strings.Contains(out, s) {
			t.Fatalf("判决史缺 %q", s)
		}
	}
}

// TestAssemble_MissingRole 角色卡缺即错。
func TestAssemble_MissingRole(t *testing.T) {
	in := fixtureInput(t)
	in.Role = "CTO" // fixture 无此卡
	if _, err := Assemble(in); err == nil {
		t.Fatal("角色卡缺失应报错")
	}
}

// TestCheckSpec 缺一不派闸：任一栏空即列明缺栏。
func TestCheckSpec(t *testing.T) {
	in := fixtureInput(t)
	in.Spec.Scope = ""
	in.Spec.Deliver = "  "
	err := CheckSpec(in.Spec)
	if err == nil || !strings.Contains(err.Error(), "第3栏") || !strings.Contains(err.Error(), "第8栏") {
		t.Fatalf("缺栏报错不符: %v", err)
	}
	if err := CheckSpec(nil); err == nil {
		t.Fatal("nil spec 应报错")
	}
}

// TestParseTicketMD_缺栏 解析闸：票面缺栏在入库径即被拒。
func TestParseTicketMD_缺栏(t *testing.T) {
	raw, err := os.ReadFile("testdata/ticket.md")
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(raw), "### 4. 范围外（禁线）\n- fixture 禁线行\n", "", 1)
	if _, err := ParseTicketMD([]byte(bad)); err == nil {
		t.Fatal("缺第4栏应报错")
	}
}

// TestParseTicketMD_真票 真票面格式回归：票0003 档必过八栏解析。
func TestParseTicketMD_真票(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "实现", "票-0003-注入机与自升级回滚.md"))
	if err != nil {
		t.Skip("真票档不在")
	}
	spec, err := ParseTicketMD(raw)
	if err != nil {
		t.Fatalf("真票解析: %v", err)
	}
	if !strings.Contains(spec.Goal, "注入机") || !strings.Contains(spec.Verify, "upgrade-drill") {
		t.Fatalf("真票栏值漂移: goal=%q verify=%q", spec.Goal, spec.Verify)
	}
}
