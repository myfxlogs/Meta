package gateway

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// ---------- 票0046 配方行真源归一：启动回种覆写＋漂移检出 ----------

// seedProto 测试种行：八字段全给，sandbox 经 prototext 落 spec——与
// gateway.go 回种环同序列化径，同文比对才有意义。
func seedProto(id string) *pbv1.ProjectSeed {
	return &pbv1.ProjectSeed{
		Id: id, RepoUrl: "/repo/x", DefaultBranch: "master",
		IntegrateCmd: "cmd-i", CheckCmd: "cmd-c", DeployCmd: "cmd-d",
		ProbeCmd: "cmd-p", RollbackCmd: "cmd-r",
		Sandbox: &pbv1.SandboxSpec{RunAs: "u"},
	}
}

// seedRow 与回种环同构的种子文面行（assert 期望值与同文预置件）。
func seedRow(t *testing.T, ps *pbv1.ProjectSeed) db.Project {
	t.Helper()
	sb := ""
	if ps.Sandbox != nil {
		if b, err := prototext.Marshal(ps.Sandbox); err == nil {
			sb = string(b)
		}
	}
	return db.Project{
		ID: ps.Id, RepoURL: ps.RepoUrl, DefaultBranch: ps.DefaultBranch,
		IntegrateCmd: ps.IntegrateCmd, CheckCmd: ps.CheckCmd,
		DeployCmd: ps.DeployCmd, ProbeCmd: ps.ProbeCmd,
		RollbackCmd: ps.RollbackCmd, SandboxSpec: sb,
	}
}

func seedCfg(dir string, seeds ...*pbv1.ProjectSeed) Config {
	return Config{
		DBPath:     filepath.Join(dir, "meta.db"),
		RunDir:     filepath.Join(dir, "run"),
		SocketPath: filepath.Join(dir, "run", "meta.sock"),
		ShellRows:  testShellRows(),
		Projects:   seeds,
	}
}

// presetProject 在 New() 之前直写库行——模拟「库行先于本次启动存在」的
// 历史态（测试径开库写后即关，非生产写径）。
func presetProject(t *testing.T, dbPath string, p db.Project) {
	t.Helper()
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.UpsertProject(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func newSeedGateway(t *testing.T, cfg Config) *Gateway {
	t.Helper()
	shortSkipInteg(t) // T51④：直建 New(cfg) 径——闸口径豁免
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.Close)
	return g
}

// TestReseedOverwrite 锚1：DB 预置异值行→New()→行被覆写为种子文面。
// （对抗提名行：gateway.go 回种调用行——改回「行在即不动」SeedProject
// 语义即红：异值原样留存，want 比对必炸。）
func TestReseedOverwrite(t *testing.T) {
	ctx := context.Background()
	ps := seedProto("P")
	cfg := seedCfg(t.TempDir(), ps)
	presetProject(t, cfg.DBPath, db.Project{
		ID: "P", RepoURL: "/old/repo", DefaultBranch: "oldbranch",
		IntegrateCmd: "old-i", CheckCmd: "old-c", DeployCmd: "old-d",
		ProbeCmd: "old-p", RollbackCmd: "x", SandboxSpec: "old-sb",
	})
	g := newSeedGateway(t, cfg)
	got, err := g.DB().GetProject(ctx, "P")
	if err != nil || got == nil {
		t.Fatalf("回种后取行: %v %v", got, err)
	}
	if want := seedRow(t, ps); *got != want {
		t.Fatalf("行未被文面覆写\ngot:  %+v\nwant: %+v", *got, want)
	}
}

// TestReseedDriftEvent 锚2：异值预置→project_recipe_sync 事件行在且 note
// 含漂移字段名；同文预置→零事件。
// （对抗提名行：比对/emit 行——删则事件恒无，半「异值产事件」必红。）
func TestReseedDriftEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("异值产事件", func(t *testing.T) {
		ps := seedProto("P")
		cfg := seedCfg(t.TempDir(), ps)
		presetProject(t, cfg.DBPath, db.Project{
			ID: "P", RepoURL: "/old/repo", RollbackCmd: "x", ProbeCmd: "old-p",
		})
		g := newSeedGateway(t, cfg)
		evs, err := g.DB().Events(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		var note string
		for _, e := range evs {
			if e.Kind == "project_recipe_sync" {
				note = e.Note
			}
		}
		if note == "" {
			t.Fatal("异值预置未产 project_recipe_sync 事件")
		}
		for _, f := range []string{"repo_url", "default_branch", "integrate_cmd",
			"check_cmd", "deploy_cmd", "probe_cmd", "rollback_cmd", "sandbox_spec"} {
			if !strings.Contains(note, f) {
				t.Fatalf("事件 note 缺漂移字段 %s: %s", f, note)
			}
		}
	})

	t.Run("同文零事件", func(t *testing.T) {
		ps := seedProto("P")
		cfg := seedCfg(t.TempDir(), ps)
		presetProject(t, cfg.DBPath, seedRow(t, ps))
		g := newSeedGateway(t, cfg)
		evs, err := g.DB().Events(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range evs {
			if e.Kind == "project_recipe_sync" {
				t.Fatalf("同文预置产漂移事件: %s", e.Note)
			}
		}
	})
}

// TestReseedEmptyKeepsRows 锚3：cfg.Projects 空＋DB 有行→行不被清
// （防「回种」误成「清表」）。
func TestReseedEmptyKeepsRows(t *testing.T) {
	ctx := context.Background()
	cfg := seedCfg(t.TempDir())
	presetProject(t, cfg.DBPath, db.Project{ID: "P", RollbackCmd: "x"})
	g := newSeedGateway(t, cfg)
	got, err := g.DB().GetProject(ctx, "P")
	if err != nil || got == nil {
		t.Fatalf("空种子后行丢失: %v %v", got, err)
	}
	if got.RollbackCmd != "x" {
		t.Fatalf("空种子后行被改: %+v", *got)
	}
}
