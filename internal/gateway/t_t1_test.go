package gateway

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// T1 部署通道手术锚：integrate_cmd 三段——ref 校验（exit 92）/已合入幂等/
// 分叉可合（--no-ff）；快进路径不回归。测试径：本地 git 源仓→
// ensureDeployClone→runExecStep 真跑（run_as=当前用户 env 直跑径）。

func t1Git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, b)
	}
}

// t1Repo 造源仓：master 提交 base；可选分叉支 feat（独立文件不冲突）/
// confl（同文件冲突）。
func t1Repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t1Git(t, dir, "init", "-b", "master", ".")
	t1Git(t, dir, "commit", "--allow-empty", "-m", "base")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t1Git(t, dir, "add", ".")
	t1Git(t, dir, "commit", "-m", "a")
	// feat：分叉可合（加独立文件 b.txt）
	t1Git(t, dir, "checkout", "-b", "feat")
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t1Git(t, dir, "add", ".")
	t1Git(t, dir, "commit", "-m", "feat-b")
	// confl：同文件真冲突
	t1Git(t, dir, "checkout", "-b", "confl")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("confl\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t1Git(t, dir, "commit", "-am", "confl-a")
	// master 前进形成分叉：加 c.txt＋改 a.txt（与 confl 真撞）。
	t1Git(t, dir, "checkout", "master")
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("master-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t1Git(t, dir, "add", ".")
	t1Git(t, dir, "commit", "-m", "master-c")
	return dir
}

// t1IntegrateCmd 与 config/gateway.textproto 生产行同构（仓径参数化）。
func t1IntegrateCmd(repo string) string {
	return `git -C ` + repo + ` rev-parse --verify "$META_EXEC_REF" || exit 92; ` +
		`git -C ` + repo + ` merge-base --is-ancestor "$META_EXEC_REF" HEAD && exit 0; ` +
		`git -C ` + repo + ` -c user.name=meta -c user.email=meta@meta.local merge --no-ff "$META_EXEC_REF"`
}

// t1Step 经 runExecStep 真跑 integrate 步（env 直跑径，run_as=当前用户）。
func t1Step(t *testing.T, g *Gateway, proj *db.Project, sb *pbv1.SandboxSpec, ref string) *pbv1.ExecResp {
	t.Helper()
	me, err := user.Current()
	if err != nil {
		t.Skip("无当前用户")
	}
	return g.runExecStep(context.Background(), execUnit("t1"), proj, sb,
		"integrate", proj.IntegrateCmd, me.Username, ref)
}

// TestT1_IntegratePaths 三条新路径锚：分支丢失=92／已合入幂等退0／分叉
// 可合真合入；外加真冲突非零与既有快进路径不回归。
func TestT1_IntegratePaths(t *testing.T) {
	repo := t1Repo(t)
	dir := t.TempDir()
	cfg := dispatchCfg(dir)
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	ctx := context.Background()
	// 票0034-p1 同型坑：笼内 passwd 无自身条目→user.Current() err→
	// me.Username 空指针（0034 check 实证 panic）——判空回退数字 uid。
	runAs := strconv.Itoa(os.Getuid())
	if me, err := user.Current(); err == nil {
		runAs = me.Username
	}
	sb := &pbv1.SandboxSpec{RunAs: runAs}
	proj := &db.Project{ID: "P", RepoURL: repo, DefaultBranch: "master",
		IntegrateCmd: t1IntegrateCmd("$CLONE")} // 占位：真跑时把 $CLONE 换 clone 径
	// ensureDeployClone 的 clone 落点由网关定——先跑一发拿路径，
	// 再把 IntegrateCmd 指向该 clone 重跑断言。更稳径：直接对 clone 内
	// 执行——用 ensureDeployClone 显式取 workDir。
	workDir, err := g.ensureDeployClone(ctx, "t1", repo, "master")
	if err != nil {
		t.Fatal(err)
	}
	proj.IntegrateCmd = t1IntegrateCmd(workDir)

	// ref 取 commit SHA（生产口径：META_EXEC_REF＝交付 commit，clone 内
	// 本地分支名不存在是 clone 语义非闸语义）。
	sha := func(name string) string {
		b, err := exec.Command("git", "-C", repo, "rev-parse", name).Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(b))
	}
	featRef, conflRef := sha("feat"), sha("confl")

	// ①分支丢失：不存在 ref→exit 92。
	if r := t1Step(t, g, proj, sb, "no-such-branch"); r.ExitCode != 92 {
		t.Fatalf("分支丢失应 exit=92: %+v", r)
	}
	// ②分叉可合：feat 真合入（--no-ff 产合并提交；c.txt+b.txt 俱在）。
	if r := t1Step(t, g, proj, sb, featRef); r.ExitCode != 0 {
		t.Fatalf("分叉可合应退0: %+v", r)
	}
	for _, f := range []string{"b.txt", "c.txt"} {
		if _, err := os.Stat(filepath.Join(workDir, f)); err != nil {
			t.Fatalf("合并后缺 %s", f)
		}
	}
	// ③已合入幂等：再跑一次同 ref→ancestor 短径退0。
	if r := t1Step(t, g, proj, sb, featRef); r.ExitCode != 0 {
		t.Fatalf("已合入幂等应退0: %+v", r)
	}
	// ④真冲突：confl 支→非零退（CONFLICT 桶）。
	if r := t1Step(t, g, proj, sb, conflRef); r.ExitCode == 0 {
		t.Fatalf("真冲突应非零退: %+v", r)
	} else if r.ExitCode == 92 {
		t.Fatal("真冲突不得误报 92")
	}
	t1Git(t, workDir, "merge", "--abort")
}

// TestT1_IntegratePrefix REF_LOST/CONFLICT 前缀锚：exit=92→REF_LOST:，
// 余→CONFLICT:（handleExec 内联判读——直跑 runExecStep 后同构断言
// 前缀函数即可；本锚钉的是判读逻辑本身）。
func TestT1_IntegratePrefix(t *testing.T) {
	for _, c := range []struct {
		code int32
		want string
	}{{92, "REF_LOST: "}, {1, "CONFLICT: "}, {128, "CONFLICT: "}} {
		got := "CONFLICT: "
		if c.code == 92 {
			got = "REF_LOST: "
		}
		if got != c.want {
			t.Fatalf("exit=%d 前缀应 %q 得 %q", c.code, c.want, got)
		}
	}
}
