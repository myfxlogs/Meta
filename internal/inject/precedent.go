package inject

// 判例前置（票0037-p2，决策90）：派单组装时刻按票面 Goal+Scope 关键词机械
// 检索两库 *.md（文件名＋正文首段），命中项以「判例前置」段插入施工提示——
// 两库只静态挂索引＝写了没人看（哨兵 JSON 包裹式复发坑），检索不上模型，
// 关键词重叠计数拿 80% 价值。同步、随 Assemble 一次完成，无缓存无轮询（R2）。

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// 检索面＝两库顶层（不递归、不读 run/ 下镜像）。
var precedentDirs = []string{
	filepath.Join("docs", "经验库"),
	filepath.Join("docs", "技能库"),
}

// precedentHit 一条命中：Root 相对路径＋首行摘要＋评分。
type precedentHit struct {
	Path    string
	Summary string
	Score   int
}

// precedents 机械检索：query＝票面 Goal+Scope 分词，扫描面＝两库每个 *.md 的
// 文件名＋正文首段；评分＝关键词重叠计数（去重后交集大小），>0 为命中。
// 返回按评分降序（同分按路径序，保证产物确定）；读不了的文件跳过——检索是
// 机会性增强，不许拖死派单（库目录本身不存在/不可读时上游 libIndex 已先报）。
func precedents(goal, scope, root string) []precedentHit {
	query := tokenize(goal + "\n" + scope)
	if len(query) == 0 {
		return nil
	}
	var hits []precedentHit
	for _, rel := range precedentDirs {
		dir := filepath.Join(root, rel)
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			body, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			surface := strings.TrimSuffix(e.Name(), ".md") + "\n" + firstPara(string(body))
			score := overlap(query, tokenize(surface))
			if score == 0 {
				continue
			}
			hits = append(hits, precedentHit{
				Path:    filepath.ToSlash(filepath.Join(rel, e.Name())),
				Summary: firstSummaryLine(string(body), e.Name()),
				Score:   score,
			})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Path < hits[j].Path
	})
	return hits
}

// tokenize 分词（钦定口径）：连续 CJK 序列按 2 字滑窗切（`哨兵JSON`→
// 哨兵/兵J/JSO/SON 式逐对入集）；英文/数字 `[A-Za-z0-9_]+` 整词小写化；
// 无停用词表（重叠计数天然抑噪）。孤立单字 CJK 以单字入集。
func tokenize(s string) map[string]struct{} {
	out := map[string]struct{}{}
	rs := []rune(s)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case unicode.Is(unicode.Han, r):
			j := i
			for j < len(rs) && unicode.Is(unicode.Han, rs[j]) {
				j++
			}
			if j-i == 1 {
				out[string(rs[i])] = struct{}{}
			}
			for k := i; k+1 < j; k++ {
				out[string(rs[k:k+2])] = struct{}{}
			}
			i = j
		case isWordByte(r):
			j := i
			for j < len(rs) && isWordByte(rs[j]) {
				j++
			}
			out[strings.ToLower(string(rs[i:j]))] = struct{}{}
			i = j
		default:
			i++
		}
	}
	return out
}

func isWordByte(r rune) bool {
	return r == '_' ||
		(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func overlap(a, b map[string]struct{}) int {
	if len(a) > len(b) {
		a, b = b, a
	}
	n := 0
	for k := range a {
		if _, ok := b[k]; ok {
			n++
		}
	}
	return n
}

// firstPara 正文首段：跳过前导空行后第一段连续非空行（命中只扫首段非全文）。
func firstPara(s string) string {
	var b strings.Builder
	seen := false
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) == "" {
			if seen {
				break
			}
			continue
		}
		seen = true
		b.WriteString(ln + "\n")
	}
	return b.String()
}

// firstSummaryLine 摘要口径：首个非空非标题行（无则取首行，再无取文件名）。
func firstSummaryLine(body, name string) string {
	lns := strings.Split(body, "\n")
	fallback := name
	for _, ln := range lns {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if fallback == name {
			fallback = t // 首个非空行兜底
		}
		if !strings.HasPrefix(t, "#") {
			return t
		}
	}
	return fallback
}
