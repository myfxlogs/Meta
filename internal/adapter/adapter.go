// Package adapter 是壳适配层：统一契约四件——起（Build）/流读（Parse）/
// 用量（Tokens、FinalTokens）/退码（Classify）。壳 CLI 的 JSON/文本输出
// 只在本包解析并即解即转 proto 语义帧，不许流出本包（禁区#3）。
// 壳差异止于本包；三壳无头档位的实测结论见 docs/经验库/。
package adapter

import (
	"fmt"
	"regexp"
	"strings"
)

// Spec 起一次壳会话的输入（壳无关）。
type Spec struct {
	Shell       string // devin|claude|opencode
	Model       string // 空=壳自身缺省
	Role        string
	PromptFile  string // 提示词文件路径；无 --prompt-file 旗的壳走 stdin 等价径
	WorkDir     string // 席位工作目录
	ArtifactDir string // 适配器产物落点（devin ATIF 等），须已存在
	Env         []string
}

// Command 起：exec 参数。StdinFile 非空时提示词经 stdin 重定向
// （无 --prompt-file 旗壳的「一律文件径」等价实现——决策13定向④）。
type Command struct {
	Argv      []string
	Dir       string
	StdinFile string
	Env       []string
}

// FrameKind 流读产物的帧类。
type FrameKind int

const (
	FrameText   FrameKind = iota // 透出行：壳活着干活的活性证据
	FrameUsage                   // 用量帧：Tokens=累计值（非增量）
	FrameResult                  // 收尾帧：壳正常产出终态
	FrameError                   // 壳自报错误
)

// Frame 流读产物——壳输出转出的语义帧。
type Frame struct {
	Kind    FrameKind
	Stage   string  // 阶段语义（可空）
	Note    string  // 原文预览
	Tokens  int64   // Usage/Result 帧：累计 token
	CostUSD float64 // 可读到则填，读不到为 0
}

// ExitClass 壳进程退码的机器分级（§7 失效分级①②a②b）。
type ExitClass int

const (
	ExitOK        ExitClass = iota // 正常退出
	ExitSpawn                      // ① 起不来/秒退/外部杀死——机器级，不标壳
	ExitTransient                  // ②a 限流/瞬态 API 错——标 limited，错峰懒探
	ExitFatal                      // ②b 配额尽/授权过期/持续 API 错——标 dead，换壳懒探
)

// Adapter 统一适配器接口（决策13①四件）。实例与一次会话一一对应：
// Parse 是有状态的累计器（Tokens 随流增长）。
type Adapter interface {
	Name() string
	// Build 起：由 Spec 产 argv/stdin/env——不调外部命令，纯构造。
	Build(Spec) (Command, error)
	// Parse 流读：喂入一行完整 stdout，产 0..n 帧。
	Parse(line []byte) []Frame
	// Tokens 流内累计用量；壳流不带用量时恒 0。
	Tokens() int64
	// FinalTokens 退出后终值：流内已有则回流内值；devin 类走产物文件终读。
	// 读不到＝ok=false——如实标「不可得」，禁估算（决策13②）。
	FinalTokens(Spec) (int64, bool)
	// Classify 退码分级：非零退码+stderr 尾 → ①/②a/②b。
	Classify(code int, tail string) ExitClass
}

// New 按壳名建适配器实例。
func New(shell string) (Adapter, error) {
	switch shell {
	case "devin":
		return &devin{}, nil
	case "claude":
		return &claude{}, nil
	case "opencode":
		return &opencode{}, nil
	}
	return nil, fmt.Errorf("adapter: 未知壳 %q", shell)
}

// ---------- 退码分级共享件 ----------

var (
	reTransient = regexp.MustCompile(`(?i)rate.?limit|429|throttl|too many requests|overloaded|temporarily|try again|timeout|timed out|connection (reset|refused)|econnreset`)
	reFatal     = regexp.MustCompile(`(?i)unauthorized|401|invalid (api.?)?key|permission denied|forbidden|quota|insufficient|billing|payment|credit balance|expired|not logged in|login required`)
	reSpawn     = regexp.MustCompile(`(?i)not installed|untrusted workspace|command not found|no such file|sandbox initialization failed`)
)

// classifyTail 共享尾部分级：瞬态→配额/授权死→机器级，次序即语义
// （②a 先于 ②b 判——同文双命中时取较轻处置，换壳损失最小）。
func classifyTail(code int, tail string) ExitClass {
	if code == 0 {
		return ExitOK
	}
	if code < 0 { // 信号杀死（含外部 SIGKILL）——非壳之过的机器级失效
		return ExitSpawn
	}
	switch {
	case reTransient.MatchString(tail):
		return ExitTransient
	case reFatal.MatchString(tail):
		return ExitFatal
	case reSpawn.MatchString(tail):
		return ExitSpawn
	}
	return ExitSpawn // 未识别非零退＝① 机器级（起不来/秒退同桶）
}

// tailOf 取输出尾部（分级输入），最长 n 字节按行截。
func tailOf(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// preview 单行预览（Note 字段上限）。
func preview(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
