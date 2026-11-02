package gateway

// T27 立案径四件——一件一锚。

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// 件① 链深闸族世代计数：兄弟编号 p2/p3 同层也入族——族 ≥3 员截断，
// 旧 -pN 段数判（每员各一段）绕闸实证（0034 族 p4）。
// 对抗锚提名行：FamilyTicketCount 调用——删则兄弟票绕闸回潮。
func TestT27_ChainFamilyGate(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	ctx := context.Background()
	// 族 {r, r-p1, r-p2}=3 员：派 r-p2 已截断（段数 1 的旧径放行＝绕闸）。
	for _, id := range []string{"r", "r-p1", "r-p2"} {
		if err := g.DB().CreateTicket(ctx, id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := g.DispatchSeat(ctx, "r-p2", "CTO", dispatchSpec(t)); err == nil {
		t.Fatal("族满 3 员的兄弟票应截断")
	}
	e := waitEvent(t, g, "chain_depth_limit", "r-p2", 3*time.Second)
	if e.Kind != "chain_depth_limit" {
		t.Fatal("chain_depth_limit 未发")
	}
	// 族 2 员放行：r-p1 未达帽。
	if err := g.DispatchSeat(ctx, "r-p1", "CTO", dispatchSpec(t)); err == nil {
		waitEvent(t, g, "seat_up", "r-p1", 8*time.Second)
	}
	// 根票不涉世代：族满根票照派（根票无 -pN 段）。
	mkTicketAt(t, g, "rootx", db.PhaseAssigned)
	if err := g.DispatchSeat(ctx, "rootx", "施工", dispatchSpec(t)); err != nil {
		t.Fatalf("根票不应被族闸拦: %v", err)
	}
	// familyRoot 纯函数锚。
	for in, want := range map[string]string{
		"x-p1-p1": "x", "x-p12": "x", "x": "x", "x-foo": "x-foo", "x-p1a": "x-p1a",
	} {
		if got := familyRoot(in); got != want {
			t.Fatalf("familyRoot(%s)=%s want %s", in, got, want)
		}
	}
}

// 件② 同题查重：在途同题票（关键词交集≥2）→并档返既有号不立新；
// 异题照立。对抗锚提名行：dupTicketFor/titleSimilar——删则同题复立。
func TestT27_TicketDupMerge(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	if err := g.DB().CreateTicket(ctx, "a1", "哨兵泵断流致部署尾帧蒸发", nil); err != nil {
		t.Fatal(err)
	}
	if got := g.dupTicketFor(ctx, "部署尾帧蒸发哨兵泵问题", "b2"); got != "a1" {
		t.Fatalf("同题应并档 a1: %q", got)
	}
	if got := g.dupTicketFor(ctx, "路由表热载验证新件", "b3"); got != "" {
		t.Fatalf("异题不该并档: %q", got)
	}
	// 父票豁免：本票不参与查重（父子题面近似是常态）。
	if got := g.dupTicketFor(ctx, "哨兵泵断流致部署尾帧蒸发", "a1"); got != "" {
		t.Fatalf("父票应豁免查重: %q", got)
	}
	// titleSimilar 纯函数锚。
	if !titleSimilar("abc def", "abc def") || !titleSimilar("前缀 哨兵泵 断流 修复", "哨兵泵 断流 事件源") {
		t.Fatal("相似题判漏")
	}
	if titleSimilar("哨兵泵 断流", "壳健康 迁移") {
		t.Fatal("异题误判相似")
	}
	_ = pbv1.Ack{}
}

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
