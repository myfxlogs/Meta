// Package inject 是注入机（票0003）：spawn 前网关机械组装席位提示前缀——
// 角色卡全文＋票面八栏（出 DB 的 TicketSpec）＋项目配方＋经验库/技能库索引
// （§4「注入角色卡+票面+项目配方」，库注索引钦定）。纯函数、fixture 可测；
// 组装失败＝缺一不派，派发出 inject_failed 事件，不耗壳尝试。
package inject

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	pbv1 "meta/internal/pb/metav1"
)

// Input 组装输入。Root 是注入源根（含 roles/ 与 docs/经验库、docs/技能库），
// Recipes 是相对 Root 的项目配方/纪律文档清单（序即拼接序）。
type Input struct {
	Root     string
	Role     string
	TicketID string
	Title    string
	Spec     *pbv1.TicketSpec // 必给；八栏缺一即错（§6.1 缺一不派机械闸）
	Verdicts []string         // 动态判决史行（ticket_event reject/yield），可空
	Recipes  []string
	Now      time.Time // 可注入钉死；零值取 time.Now
}

// colNames 八栏位序（§6.1 契约序）。
var colNames = []string{"目标", "为什么", "范围内", "范围外（禁线）", "设计引用", "验收锚", "验证命令", "交付形制"}

// specCols 与 colNames 同序取栏值——对抗性锚提名区：删任一取值行，
// 注入内容单测对应栏断言必红。
func specCols(s *pbv1.TicketSpec) []string {
	return []string{
		s.GetGoal(), s.GetWhy(), s.GetScope(), s.GetOutOfScope(),
		s.GetRefs(), s.GetAnchors(), s.GetVerify(), s.GetDeliver(),
	}
}

// CheckSpec 缺一不派闸：spec 空或任一栏空 → 列明缺栏报错。
func CheckSpec(s *pbv1.TicketSpec) error {
	if s == nil {
		return fmt.Errorf("票面无 spec（缺八栏全量）")
	}
	var miss []string
	for i, v := range specCols(s) {
		if strings.TrimSpace(v) == "" {
			miss = append(miss, fmt.Sprintf("第%d栏(%s)", i+1, colNames[i]))
		}
	}
	if len(miss) > 0 {
		return fmt.Errorf("票面缺栏：%s", strings.Join(miss, "、"))
	}
	return nil
}

// Assemble 组装提示前缀全文。任一部件缺失（角色卡/配方文件）即错。
func Assemble(in Input) (string, error) {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	roleCard, err := os.ReadFile(filepath.Join(in.Root, "roles", in.Role+".md"))
	if err != nil {
		return "", fmt.Errorf("角色卡 %s: %w", in.Role, err)
	}
	if err := CheckSpec(in.Spec); err != nil {
		return "", err
	}
	var recipes []string
	for _, rel := range in.Recipes {
		b, err := os.ReadFile(filepath.Join(in.Root, rel))
		if err != nil {
			return "", fmt.Errorf("配方 %s: %w", rel, err)
		}
		recipes = append(recipes, string(b))
	}
	expIdx, err := libIndex(filepath.Join(in.Root, "docs", "经验库"))
	if err != nil {
		return "", err
	}
	skillIdx, err := libIndex(filepath.Join(in.Root, "docs", "技能库"))
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# 席位注入前缀（票 %s：%s · %s · %s）\n\n",
		in.TicketID, in.Title, in.Role, now.Format("2006-01-02"))
	b.WriteString("> 本前缀由网关注入机按票机械组装（票0003 起，总设计§4）。先读**范围外**——那是你的禁区。\n\n")

	// 对抗性锚提名行：角色卡拼入——删本行角色卡断言必红。
	fmt.Fprintf(&b, "## 一、角色卡：%s\n\n%s\n\n", in.Role, strings.TrimSpace(string(roleCard)))

	b.WriteString("## 二、票面\n\n")
	for i, name := range colNames {
		fmt.Fprintf(&b, "### %d. %s\n\n%s\n\n", i+1, name, strings.TrimSpace(specCols(in.Spec)[i]))
	}
	fmt.Fprintf(&b, "## 附\n\n%s\n\n", blankAs(in.Spec.GetAppendix()))
	// 判决史＝创建初值＋动态驳回/Yield 行（判决史累积，重做者必读）。
	vs := strings.TrimSpace(in.Spec.GetVerdicts())
	var vlines []string
	if vs != "" && !strings.HasPrefix(vs, "（空") {
		vlines = append(vlines, vs)
	}
	vlines = append(vlines, in.Verdicts...)
	b.WriteString("## 判决史\n\n")
	if len(vlines) == 0 {
		b.WriteString("（空——首判前无）\n\n")
	} else {
		b.WriteString(strings.Join(vlines, "\n") + "\n\n")
	}

	b.WriteString("## 三、项目配方与纪律文档\n\n")
	if len(recipes) == 0 {
		b.WriteString("（未配）\n\n")
	}
	for i, rel := range in.Recipes {
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", rel, strings.TrimSpace(recipes[i]))
	}

	// 对抗性锚提名区：两库索引拼入——删/翻任一拼接行，索引文件名言必红。
	b.WriteString("## 四、经验库索引（docs/经验库/——遇坑即写，动码前后各 grep 一遍）\n\n")
	writeIndex(&b, expIdx)
	b.WriteString("## 五、技能库索引（docs/技能库/——成套打法，在用证错同票必修）\n\n")
	writeIndex(&b, skillIdx)

	fmt.Fprintf(&b, "## 六、工作目录\n\n%s\n", in.Root)
	out := b.String()
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("注入产物为空（票 %s role=%s）——D-40 产物级兜底", in.TicketID, in.Role)
	}
	return out, nil
}

func writeIndex(b *strings.Builder, names []string) {
	if len(names) == 0 {
		b.WriteString("（空）\n\n")
		return
	}
	for _, n := range names {
		fmt.Fprintf(b, "- %s\n", n)
	}
	b.WriteString("\n")
}

// libIndex 列目录下 *.md 文件名清单（索引注入，非全文）。
func libIndex(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("库索引 %s: %w", dir, err)
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func blankAs(s string) string {
	if strings.TrimSpace(s) == "" {
		return "（空）"
	}
	return strings.TrimSpace(s)
}

// ---------- 票面 markdown → TicketSpec（meta ticket new --spec-file） ----------

var (
	reColHead = regexp.MustCompile(`^#{2,4}\s*([1-8])[\.、]`)
	reH2      = regexp.MustCompile(`^##\s+(.+?)\s*$`)
	reAnyHead = regexp.MustCompile(`^#{1,6}\s`)
)

// ParseTicketMD 解析票面 markdown 八栏成 TicketSpec——缺一栏即错（缺一不立）。
// 栏位按 `### N.` 序号取（栏名不拘），附/判决史按 `## 附` `## 判决史` 节取。
func ParseTicketMD(src []byte) (*pbv1.TicketSpec, error) {
	var cols [8]strings.Builder
	var appendix, verdicts strings.Builder
	sec := -1 // 0..7=八栏；8=附；9=判决史；-1=节外
	for _, ln := range strings.Split(string(src), "\n") {
		if m := reColHead.FindStringSubmatch(ln); m != nil {
			sec = int(m[1][0] - '1') // '1'..'8' → 0..7
			continue
		}
		if m := reH2.FindStringSubmatch(ln); m != nil {
			switch {
			case strings.HasPrefix(m[1], "附"):
				sec = 8
			case strings.HasPrefix(m[1], "判决史"):
				sec = 9
			default:
				sec = -1
			}
			continue
		}
		if reAnyHead.MatchString(ln) {
			sec = -1
			continue
		}
		switch {
		case sec == 8:
			appendix.WriteString(ln + "\n")
		case sec == 9:
			verdicts.WriteString(ln + "\n")
		case sec >= 0:
			cols[sec].WriteString(ln + "\n")
		}
	}
	spec := &pbv1.TicketSpec{
		Goal:       cols[0].String(),
		Why:        cols[1].String(),
		Scope:      cols[2].String(),
		OutOfScope: cols[3].String(),
		Refs:       cols[4].String(),
		Anchors:    cols[5].String(),
		Verify:     cols[6].String(),
		Deliver:    cols[7].String(),
		Appendix:   appendix.String(),
		Verdicts:   verdicts.String(),
	}
	return spec, CheckSpec(spec)
}
