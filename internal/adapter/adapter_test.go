package adapter

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 录制样本驱动（验收锚2）：testdata/ 下是 2026-11-01 三壳真会话实录。
// 起/流帧/用量/退码各断言其解析正确。

func lines(t *testing.T, name string) [][]byte {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out [][]byte
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		out = append(out, sc.Bytes())
	}
	return out
}

// ---- 起 ----

func TestBuild_Devin(t *testing.T) {
	a, _ := New("devin")
	c, err := a.Build(Spec{PromptFile: "/tmp/p.md", WorkDir: "/w", ArtifactDir: "/a", Model: "swe-2-high"})
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(c.Argv, " ")
	for _, want := range []string{"-p", "--prompt-file /tmp/p.md", "--permission-mode dangerous",
		"--respect-workspace-trust false", "--export /a/atif.json", "--model swe-2-high"} {
		if !strings.Contains(argv, want) {
			t.Fatalf("devin argv 缺 %q: %s", want, argv)
		}
	}
	if c.StdinFile != "" {
		t.Fatal("devin 用 --prompt-file，不应走 stdin")
	}
}

func TestBuild_Claude(t *testing.T) {
	a, _ := New("claude")
	c, err := a.Build(Spec{PromptFile: "/tmp/p.md", WorkDir: "/w"})
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(c.Argv, " ")
	for _, want := range []string{"-p", "--output-format stream-json",
		"--permission-mode acceptEdits", "--permission-prompts none"} {
		if !strings.Contains(argv, want) {
			t.Fatalf("claude argv 缺 %q: %s", want, argv)
		}
	}
	if c.StdinFile != "/tmp/p.md" {
		t.Fatal("claude 提示词应走 stdin 重定向")
	}
}

func TestBuild_Opencode(t *testing.T) {
	a, _ := New("opencode")
	c, err := a.Build(Spec{PromptFile: "/tmp/p.md", WorkDir: "/w"})
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(c.Argv, " ")
	for _, want := range []string{"run", "--format json", "--auto", "--standalone"} {
		if !strings.Contains(argv, want) {
			t.Fatalf("opencode argv 缺 %q: %s", want, argv)
		}
	}
	if c.StdinFile != "/tmp/p.md" {
		t.Fatal("opencode 提示词应走 stdin 重定向")
	}
}

// ---- 流帧＋用量（录制样本） ----

func TestParse_ClaudeRecorded(t *testing.T) {
	a, _ := New("claude")
	var sawText, sawResult bool
	for _, ln := range lines(t, "claude.jsonl") {
		for _, fr := range a.Parse(ln) {
			switch fr.Kind {
			case FrameText:
				sawText = true
			case FrameResult:
				sawResult = true
				if fr.Tokens != 15301 { // 3007+6+12288
					t.Fatalf("result tokens=%d 预期 15301", fr.Tokens)
				}
				if fr.CostUSD <= 0 {
					t.Fatal("result 应带 cost")
				}
			}
		}
	}
	if !sawText || !sawResult {
		t.Fatalf("帧缺失: text=%v result=%v", sawText, sawResult)
	}
	tok, ok := a.FinalTokens(Spec{})
	if !ok || tok != 15301 {
		t.Fatalf("claude 用量=%d ok=%v", tok, ok)
	}
}

func TestParse_OpencodeRecorded(t *testing.T) {
	a, _ := New("opencode")
	var sawText, sawUsage bool
	for _, ln := range lines(t, "opencode.jsonl") {
		for _, fr := range a.Parse(ln) {
			switch fr.Kind {
			case FrameText:
				if fr.Note == "SMOKE_OK" {
					sawText = true
				}
			case FrameUsage:
				sawUsage = true
				if fr.Tokens != 5535 { // 5253+26+0+256+0
					t.Fatalf("step_finish tokens=%d 预期 5535", fr.Tokens)
				}
			}
		}
	}
	if !sawText || !sawUsage {
		t.Fatalf("帧缺失: text=%v usage=%v", sawText, sawUsage)
	}
	if tok, ok := a.FinalTokens(Spec{}); !ok || tok != 5535 {
		t.Fatalf("opencode 用量=%d ok=%v", tok, ok)
	}
}

func TestParse_DevinRecorded(t *testing.T) {
	a, _ := New("devin")
	var sawText bool
	for _, ln := range lines(t, "devin.txt") {
		for _, fr := range a.Parse(ln) {
			if fr.Kind == FrameText && fr.Note == "SMOKE_OK" {
				sawText = true
			}
		}
	}
	if !sawText {
		t.Fatal("devin 文本帧未解析")
	}
	// devin 流内无用量；终值读 ATIF 产物。
	if a.Tokens() != 0 {
		t.Fatal("devin 流内不应有用量")
	}
	dir := t.TempDir()
	raw, _ := os.ReadFile(filepath.Join("testdata", "atif.json"))
	if err := os.WriteFile(filepath.Join(dir, "atif.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	tok, ok := a.FinalTokens(Spec{ArtifactDir: dir})
	if !ok || tok != 24748 { // 12876+32+11840
		t.Fatalf("devin ATIF 用量=%d ok=%v", tok, ok)
	}
	// 不可得即 false，禁估算。
	if _, ok := a.FinalTokens(Spec{ArtifactDir: t.TempDir()}); ok {
		t.Fatal("ATIF 缺失应标不可得")
	}
}

// ---- 退码分级 ----

func TestClassify(t *testing.T) {
	cases := []struct {
		shell string
		code  int
		tail  string
		want  ExitClass
	}{
		{"claude", 0, "", ExitOK},
		{"claude", -1, "", ExitSpawn},                                        // SIGKILL 外部杀死＝①机器级
		{"claude", 1, "Error: rate limit exceeded (429)", ExitTransient},     // ②a
		{"claude", 1, "Error: 401 unauthorized: invalid api key", ExitFatal}, // ②b
		{"opencode", 1, "ProviderError: insufficient quota", ExitFatal},
		{"opencode", 1, "throttled, try again later", ExitTransient},
		{"devin", 1, "Refusing to run in an untrusted workspace", ExitSpawn},
		{"devin", 1, "Sandbox initialization failed: bwrap is not installed", ExitSpawn},
		{"devin", 2, "unexpected crash", ExitSpawn}, // 未识别非零＝①
	}
	for _, tc := range cases {
		a, err := New(tc.shell)
		if err != nil {
			t.Fatal(err)
		}
		if got := a.Classify(tc.code, tc.tail); got != tc.want {
			t.Errorf("%s code=%d tail=%q: %d 预期 %d", tc.shell, tc.code, tc.tail, got, tc.want)
		}
	}
}

func TestNew_Unknown(t *testing.T) {
	if _, err := New("bash"); err == nil {
		t.Fatal("未知壳应报错")
	}
}
