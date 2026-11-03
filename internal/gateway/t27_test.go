package gateway

// T27 立案径四件——一件一锚。

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"meta/internal/db"
)

// 件③ close-audit 改 DB 径：脚本不再 sqlite3 直读，改 meta memory CLI。
func TestT27_CloseAuditDBPath(t *testing.T) {
	b, err := os.ReadFile("../../scripts/close-audit.sh")
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, "meta memory") {
		t.Fatal("close-audit 未改 meta memory 读径")
	}
	if strings.Contains(body, "sqlite3") {
		t.Fatal("close-audit 仍留 sqlite3 直读死指针")
	}
}

// 件④ 死席残件连清：sock 插座连删＋git 工作区随收尸清；日志留档不动。
// 对抗锚提名行：cleanSeatResidue 调用点——删则插座/工作区残留回潮。
func TestT27_SeatResidueClean(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	runID := "resid001"
	// 插座件。
	sock := filepath.Join(g.cfg.RunDir, "seats", runID+".sock")
	if err := os.WriteFile(sock, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// 真 git worktree 工作区（主仓→linked wt）。
	repo := filepath.Join(t.TempDir(), "repo")
	run := func(args ...string) {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", repo)
	run("-C", repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init", "--allow-empty")
	wd := filepath.Join(t.TempDir(), "wt-x")
	run("-C", repo, "worktree", "add", "-q", wd, "-b", "wt-x")
	mkTicketAt(t, g, "tr27", db.PhaseWorking)
	g.mu.Lock()
	g.specs[specKey{"tr27", "施工"}] = SeatSpec{WorkDir: wd}
	g.mu.Unlock()
	g.cleanSeatResidue(&db.Run{RunID: runID, TicketID: "tr27", Role: "施工", SocketPath: sock})
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Fatal("插座未连清")
	}
	if _, err := os.Stat(wd); !os.IsNotExist(err) {
		t.Fatal("git 工作区未随收尸清")
	}
	// 非 worktree 工作区不动（手工件不误删）。
	plain := t.TempDir()
	g.mu.Lock()
	g.specs[specKey{"tr27", "施工"}] = SeatSpec{WorkDir: plain}
	g.mu.Unlock()
	g.cleanSeatResidue(&db.Run{RunID: "resid002", TicketID: "tr27", Role: "施工"})
	if _, err := os.Stat(plain); err != nil {
		t.Fatal("非 worktree 工作区被误删")
	}
}
