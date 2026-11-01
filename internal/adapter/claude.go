package adapter

import (
	"encoding/json"
)

// claude 适配器：`claude -p --output-format stream-json` 无头；
// 提示词经 stdin 重定向（claude 无 --prompt-file 旗，stdin 为等价径）。
// --permission-prompts none＝许可提示自动答拒（禁区#12 席位不等人）。
// 流为 JSONL：system.init / assistant / result；用量在 result.usage。
type claude struct {
	tokens  int64
	costUSD float64
}

func (c *claude) Name() string { return "claude" }

func (c *claude) Build(s Spec) (Command, error) {
	argv := []string{"claude", "-p",
		"--output-format", "stream-json",
		"--verbose",
		"--permission-mode", "acceptEdits",
		"--permission-prompts", "none",
	}
	if s.Model != "" {
		argv = append(argv, "--model", s.Model)
	}
	return Command{Argv: argv, Dir: s.WorkDir, StdinFile: s.PromptFile, Env: s.Env}, nil
}

// stream-json 行骨架（只取要用的字段，其余原样弃——JSON 止于本包）。
type claudeLine struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Model   string `json:"model"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
	Usage   struct {
		InputTokens         int64 `json:"input_tokens"`
		OutputTokens        int64 `json:"output_tokens"`
		CacheReadTokens     int64 `json:"cache_read_input_tokens"`
		CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Message      struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
			Name string `json:"name"`
		} `json:"content"`
	} `json:"message"`
}

// Parse claude JSONL：init→活性行；assistant 文本/工具名→活性行；
// result→收尾帧（带累计用量与成本）；不可解析行按活性行透传。
func (c *claude) Parse(line []byte) []Frame {
	var m claudeLine
	if err := json.Unmarshal(line, &m); err != nil {
		return []Frame{{Kind: FrameText, Note: preview(string(line), 200), Text: string(line)}}
	}
	switch m.Type {
	case "system":
		if m.Subtype == "init" {
			return []Frame{{Kind: FrameText, Stage: "起", Note: "claude init model=" + m.Model}}
		}
	case "assistant":
		for _, p := range m.Message.Content {
			if p.Type == "text" && p.Text != "" {
				return []Frame{{Kind: FrameText, Note: preview(p.Text, 200), Text: p.Text}}
			}
			if p.Type == "tool_use" {
				return []Frame{{Kind: FrameText, Stage: "工具", Note: p.Name}}
			}
		}
	case "result":
		// 用量口径：input+output+cache 全计（处理过的 token 都算成本）。
		m.Usage.InputTokens += m.Usage.CacheReadTokens + m.Usage.CacheCreationTokens
		c.tokens = m.Usage.InputTokens + m.Usage.OutputTokens
		c.costUSD = m.TotalCostUSD
		if m.IsError {
			return []Frame{{Kind: FrameError, Note: preview(m.Result, 200), Text: m.Result}}
		}
		return []Frame{{Kind: FrameResult, Note: preview(m.Result, 200), Text: m.Result,
			Tokens: c.tokens, CostUSD: c.costUSD}}
	}
	return nil
}

func (c *claude) Tokens() int64 { return c.tokens }

func (c *claude) FinalTokens(_ Spec) (int64, bool) {
	return c.tokens, c.tokens > 0
}

func (c *claude) Classify(code int, tail string) ExitClass {
	return classifyTail(code, tailOf(tail, 2048))
}
