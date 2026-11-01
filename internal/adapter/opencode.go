package adapter

import (
	"encoding/json"
)

// opencode 适配器：`opencode run --format json` 无头；--auto 自动批准
// 未显式拒绝的许可（禁区#12 不等人）；--standalone 免后台服务依赖。
// 提示词经 stdin 重定向。流为 JSONL：step_start/text/step_finish；
// 用量在 step_finish.part.tokens。
type opencode struct {
	tokens  int64
	costUSD float64
}

func (o *opencode) Name() string { return "opencode" }

func (o *opencode) Build(s Spec) (Command, error) {
	argv := []string{"opencode", "run", "--format", "json", "--auto", "--standalone"}
	if s.Model != "" {
		argv = append(argv, "-m", s.Model)
	}
	return Command{Argv: argv, Dir: s.WorkDir, StdinFile: s.PromptFile, Env: s.Env}, nil
}

type ocLine struct {
	Type string `json:"type"`
	Part struct {
		Type    string  `json:"type"`
		Text    string  `json:"text"`
		Reason  string  `json:"reason"`
		Cost    float64 `json:"cost"`
		Message string  `json:"message"`
		Tokens  struct {
			Input     int64 `json:"input"`
			Output    int64 `json:"output"`
			Reasoning int64 `json:"reasoning"`
			Cache     struct {
				Read  int64 `json:"read"`
				Write int64 `json:"write"`
			} `json:"cache"`
		} `json:"tokens"`
	} `json:"part"`
}

// Parse opencode JSONL：text→活性行；step_finish→用量帧（tokens 全计，
// 含 reasoning 与 cache）；error→错误帧；不可解析行按活性行透传。
func (o *opencode) Parse(line []byte) []Frame {
	var m ocLine
	if err := json.Unmarshal(line, &m); err != nil {
		return []Frame{{Kind: FrameText, Note: preview(string(line), 200), Text: string(line)}}
	}
	switch m.Type {
	case "text":
		if m.Part.Text != "" {
			return []Frame{{Kind: FrameText, Note: preview(m.Part.Text, 200), Text: m.Part.Text}}
		}
	case "tool_use":
		return []Frame{{Kind: FrameText, Stage: "工具", Note: m.Part.Type}}
	case "step_finish":
		o.tokens += m.Part.Tokens.Input + m.Part.Tokens.Output +
			m.Part.Tokens.Reasoning + m.Part.Tokens.Cache.Read + m.Part.Tokens.Cache.Write
		o.costUSD += m.Part.Cost
		return []Frame{{Kind: FrameUsage, Note: "step_finish " + m.Part.Reason,
			Tokens: o.tokens, CostUSD: o.costUSD}}
	case "error":
		return []Frame{{Kind: FrameError, Note: preview(m.Part.Message+m.Part.Reason, 200),
			Text: m.Part.Message + m.Part.Reason}}
	}
	return nil
}

func (o *opencode) Tokens() int64 { return o.tokens }

func (o *opencode) FinalTokens(_ Spec) (int64, bool) {
	return o.tokens, o.tokens > 0
}

func (o *opencode) Classify(code int, tail string) ExitClass {
	return classifyTail(code, tailOf(tail, 2048))
}
