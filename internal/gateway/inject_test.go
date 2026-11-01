package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// injectFixture 临时注入源根（roles/两库/配方）。
func injectFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("roles/施工.md", "# 角色卡：施工\n\n你是一个**施工实例**，fixture 关键行。\n")
	mk("docs/经验库/2026-11-01-注入坑.md", "x")
	mk("docs/技能库/注入技.md", "x")
	mk("recipe.md", "配方 fixture：纪律行")
	return root
}

func testSpec(t *testing.T) []byte {
	t.Helper()
	b, err := proto.Marshal(&pbv1.TicketSpec{
		Goal: "fixture 目标", Why: "fixture 为什么", Scope: "fixture 范围内",
		OutOfScope: "fixture 禁线", Refs: "fixture 引用", Anchors: "fixture 锚",
		Verify: "fixture 命令", Deliver: "fixture 形制", Appendix: "fixture 附",
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestDispatch_Inject 注入机径：无 --prompt-file 时组装前缀落 inject/<票>.md，
// 内容含角色卡行/栏值/索引；席位经该文件起（attach 证明派发走到 spawn）。
func TestDispatch_Inject(t *testing.T) {
	root := injectFixture(t)
	g, dir := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.InjectRoot = root
		c.InjectRecipes = []string{"recipe.md"}
	})
	ctx := context.Background()
	if err := g.DB().CreateTicket(ctx, "ti", "注入票", testSpec(t)); err != nil {
		t.Fatal(err)
	}
	spec := SeatSpec{WorkDir: "/tmp", Env: []string{
		"GO_HELPER_SEAT=1", "SEAT_MODE=worker", "META_ADOPT_GRACE=30",
		"PATH=" + os.Getenv("PATH")}}
	if err := g.DispatchSeat(ctx, "ti", "施工", spec); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, g, "ti", db.PhaseWorking, 10*time.Second)

	pf := filepath.Join(dir, "run", "inject", "ti-施工.md")
	waitFile(t, pf, 3*time.Second)
	out, _ := os.ReadFile(pf)
	for _, s := range []string{
		"fixture 关键行", "fixture 目标", "fixture 禁线", "fixture 命令",
		"配方 fixture：纪律行", "2026-11-01-注入坑.md", "注入技.md",
	} {
		if !strings.Contains(string(out), s) {
			t.Fatalf("注入文件缺 %q", s)
		}
	}
}

// TestDispatch_Inject缺一不派 票面无 spec＋注入机开 → inject_failed，不耗壳尝试。
func TestDispatch_Inject缺一不派(t *testing.T) {
	root := injectFixture(t)
	g, _ := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.InjectRoot = root
	})
	ctx := context.Background()
	mkTicketAt(t, g, "tn", db.PhaseAssigned) // 无 spec 票
	spec := SeatSpec{WorkDir: "/tmp", Env: []string{
		"GO_HELPER_SEAT=1", "SEAT_MODE=worker", "PATH=" + os.Getenv("PATH")}}
	if err := g.DispatchSeat(ctx, "tn", "施工", spec); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "inject_failed", "tn", 8*time.Second)
	for _, e := range collectEvents(g, 300*time.Millisecond) {
		if e.Kind == "seat_up" && e.TicketID == "tn" {
			t.Fatal("缺 spec 票不该起席")
		}
	}
}

// TestDispatch_PromptFile覆盖 --prompt-file 保留为覆盖径：注入机开也不组装。
func TestDispatch_PromptFile覆盖(t *testing.T) {
	root := injectFixture(t)
	g, dir := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.InjectRoot = root
	})
	ctx := context.Background()
	if err := g.DB().CreateTicket(ctx, "to", "覆盖票", testSpec(t)); err != nil {
		t.Fatal(err)
	}
	if err := g.DispatchSeat(ctx, "to", "施工", dispatchSpec(t)); err != nil { // dispatchSpec 带 fixture prompt
		t.Fatal(err)
	}
	waitPhase(t, g, "to", db.PhaseWorking, 10*time.Second)
	if _, err := os.Stat(filepath.Join(dir, "run", "inject", "to-施工.md")); !os.IsNotExist(err) {
		t.Fatal("覆盖径不该产出注入文件")
	}
}
