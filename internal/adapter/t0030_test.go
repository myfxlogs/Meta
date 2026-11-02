package adapter

import (
	"strings"
	"testing"
)

// TestT30_ClaudeResumeArgv 验收锚1：Spec.ResumeSession→argv 含
// --resume <id>；空则 argv 不变。
func TestT30_ClaudeResumeArgv(t *testing.T) {
	c := &claude{}
	base, err := c.Build(Spec{Shell: "claude", PromptFile: "/tmp/p.md"})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range base.Argv {
		if a == "--resume" {
			t.Fatalf("无 ResumeSession 时 argv 不得含 --resume: %v", base.Argv)
		}
	}
	got, err := c.Build(Spec{Shell: "claude", PromptFile: "/tmp/p.md", ResumeSession: "sess-42"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got.Argv, " ")
	if !strings.Contains(joined, "--resume sess-42") {
		t.Fatalf("ResumeSession 未进 argv: %v", got.Argv)
	}
}

// TestT30_ClaudeSessionCapture 验收锚1：init/result 行 session_id 被捕；
// 无该字段不覆盖旧值。
func TestT30_ClaudeSessionCapture(t *testing.T) {
	c := &claude{}
	c.Parse([]byte(`{"type":"system","subtype":"init","model":"x","session_id":"s-init"}`))
	if c.SessionID() != "s-init" {
		t.Fatalf("init 行 session_id 未捕: %q", c.SessionID())
	}
	c.Parse([]byte(`{"type":"assistant","session_id":"s-new","message":{"content":[{"type":"text","text":"hi"}]}}`))
	if c.SessionID() != "s-new" {
		t.Fatalf("后续行 session_id 未更新: %q", c.SessionID())
	}
	c.Parse([]byte(`{"type":"system","subtype":"other"}`))
	if c.SessionID() != "s-new" {
		t.Fatalf("无 session_id 行不应覆盖: %q", c.SessionID())
	}
}

// TestT30_Resumer 验收锚1：可选接口存在性——claude 报支持续会，
// devin/opencode 本报不支持（不臆造壳旗）。
func TestT30_Resumer(t *testing.T) {
	adm := map[string]Adapter{"claude": &claude{}, "devin": &devin{}, "opencode": &opencode{}}
	want := map[string]bool{"claude": true, "devin": false, "opencode": false}
	for name, ad := range adm {
		rs, ok := ad.(Resumer)
		if !ok {
			t.Fatalf("%s 未实现 Resumer 口", name)
		}
		if rs.ResumeOK() != want[name] {
			t.Fatalf("%s ResumeOK=%v 预期 %v", name, rs.ResumeOK(), want[name])
		}
	}
}
