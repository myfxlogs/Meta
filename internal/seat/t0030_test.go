package seat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meta/internal/adapter"
	pbv1 "meta/internal/pb/metav1"
)

// TestT30_RelayFeed 验收锚2：Feed 帧→$META_ART_DIR/inbox.log 追加
// 「[actor] text」原文行。
func TestT30_RelayFeed(t *testing.T) {
	dir := t.TempDir()
	s := &session{spec: spec{Spec: adapter.Spec{ArtifactDir: dir}}}
	s.relayFeed(&pbv1.Feed{Id: 7, Actor: "ceo", Text: "改方案X"})
	s.relayFeed(&pbv1.Feed{Id: 8, Actor: "ceo", Text: "再补一句"})
	b, err := os.ReadFile(filepath.Join(dir, "inbox.log"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || lines[0] != "[ceo] 改方案X" || lines[1] != "[ceo] 再补一句" {
		t.Fatalf("inbox.log 内容错: %q", b)
	}
}

// TestT30_SpecFromEnv_Resume 验收锚3（env 透传径）：META_RESUME_SESSION
// 落 adapter.Spec.ResumeSession。
func TestT30_SpecFromEnv_Resume(t *testing.T) {
	t.Setenv("META_SHELL", "claude")
	t.Setenv("META_GATEWAY_SOCK", "/tmp/g.sock")
	t.Setenv("META_SEAT_SOCK", "/tmp/s.sock")
	t.Setenv("META_RUN_TOKEN", "tok")
	t.Setenv("META_TICKET_ID", "t1")
	t.Setenv("META_RESUME_SESSION", "sess-77")
	sp, err := specFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if sp.ResumeSession != "sess-77" {
		t.Fatalf("META_RESUME_SESSION 未透传: %q", sp.ResumeSession)
	}
}
