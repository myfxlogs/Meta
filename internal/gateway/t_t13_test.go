package gateway

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestT13_CloneSync 票 T13 锚①③：二次 attempt 主线前进→克隆可见新提交；
// 克隆 status 净（clean -fdq 实证）。
func TestT13_CloneSync(t *testing.T) {
	repo := t1Repo(t) // 本地 git 源仓（master+分支，复用 T1 fixture）
	dir := t.TempDir()
	cfg := dispatchCfg(dir)
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	ctx := context.Background()

	w1, err := g.ensureDeployClone(ctx, "t13", repo, "master")
	if err != nil {
		t.Fatal(err)
	}
	// 源仓主线前进新提交（二次 attempt 间隙）。
	if err := os.WriteFile(filepath.Join(repo, "d.txt"), []byte("d\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t1Git(t, repo, "add", ".")
	t1Git(t, repo, "commit", "-m", "master-d")
	// 克弄脏残留（未跟踪件）——clean -fdq 应净。
	if err := os.WriteFile(filepath.Join(w1, "junk.tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w2, err := g.ensureDeployClone(ctx, "t13", repo, "master")
	if err != nil {
		t.Fatal(err)
	}
	if w1 != w2 {
		t.Fatal("克隆目录应同址")
	}
	// 锚①：新提交可见。
	log, err := exec.Command("git", "-C", w2, "log", "--oneline", "-3").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "master-d") {
		t.Fatalf("二次同步未见主线新提交: %s", log)
	}
	// 锚③：status 净（junk.tmp 被 clean 掉）。
	st, err := exec.Command("git", "-C", w2, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(st)) != "" {
		t.Fatalf("克隆 status 不净: %s", st)
	}
	if _, err := os.Stat(filepath.Join(w2, "junk.tmp")); !os.IsNotExist(err) {
		t.Fatal("脏残留未净")
	}
}
