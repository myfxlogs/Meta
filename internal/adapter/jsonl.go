package adapter

import (
	"encoding/json"
	"strings"
)

// JSONLText JSONL 行集→文本载荷：每行按 JSON 对象解码、递归抽全部字符串
// 值拼接（opencode --format json / claude stream-json 的助手文本都在
// 字符串字段里——裸扫描判不出 KEEP/DROP 行首）。
func JSONLText(out string) string {
	var b strings.Builder
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			b.WriteString(t)
			b.WriteByte('\n')
		case map[string]any:
			for _, x := range t {
				walk(x)
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "{") {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(ln), &v); err == nil {
			walk(v)
		}
	}
	return b.String()
}
