package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// devin 适配器：`devin -p --prompt-file <f>` 无头；权限档 dangerous
// （决策4 业主点名授权；sandbox 径实测 exec 废，见经验库）。
// stdout 为文本叙述行；用量不在流内——经 --export ATIF 产物终读
// （final_metrics）。实测：docs/经验库 devin 无头四条。
type devin struct {
	tokens int64
}

func (d *devin) Name() string { return "devin" }

func (d *devin) Build(s Spec) (Command, error) {
	if s.PromptFile == "" {
		return Command{}, fmt.Errorf("devin: 缺 PromptFile（一律 --prompt-file）")
	}
	if s.ArtifactDir == "" {
		return Command{}, fmt.Errorf("devin: 缺 ArtifactDir（ATIF 落点）")
	}
	argv := []string{"devin", "-p",
		"--prompt-file", s.PromptFile,
		"--permission-mode", "dangerous",
		// untrusted workspace 在无头径直接拒跑（EXIT=1）；席位 workdir 即信任域
		"--respect-workspace-trust", "false",
		"--export", filepath.Join(s.ArtifactDir, "atif.json"),
	}
	if s.Model != "" {
		argv = append(argv, "--model", s.Model)
	}
	return Command{Argv: argv, Dir: s.WorkDir, Env: s.Env}, nil
}

var reDevinErr = regexp.MustCompile(`(?i)^(error|warning)[:：]`)

// Parse devin 文本行流：一切行皆活性证据；error/warning 前缀行升级为错误帧。
func (d *devin) Parse(line []byte) []Frame {
	s := string(line)
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if reDevinErr.MatchString(strings.TrimSpace(s)) {
		return []Frame{{Kind: FrameError, Note: preview(s, 200)}}
	}
	return []Frame{{Kind: FrameText, Note: preview(s, 200)}}
}

func (d *devin) Tokens() int64 { return d.tokens }

// atif 终读：schema ATIF-v1.x，final_metrics 三件累计为用量口径
// （prompt+completion+cached——处理过的 token 全计）。
type atifFinal struct {
	FinalMetrics struct {
		TotalPromptTokens     int64 `json:"total_prompt_tokens"`
		TotalCompletionTokens int64 `json:"total_completion_tokens"`
		TotalCachedTokens     int64 `json:"total_cached_tokens"`
	} `json:"final_metrics"`
}

// FinalTokens ATIF 产物终读。文件缺/坏＝不可得（false），禁估算。
func (d *devin) FinalTokens(s Spec) (int64, bool) {
	if s.ArtifactDir == "" {
		return 0, false
	}
	raw, err := os.ReadFile(filepath.Join(s.ArtifactDir, "atif.json"))
	if err != nil {
		return 0, false
	}
	var f atifFinal
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, false
	}
	t := f.FinalMetrics.TotalPromptTokens +
		f.FinalMetrics.TotalCompletionTokens +
		f.FinalMetrics.TotalCachedTokens
	if t <= 0 {
		return 0, false
	}
	return t, true
}

func (d *devin) Classify(code int, tail string) ExitClass {
	return classifyTail(code, tailOf(tail, 2048))
}
