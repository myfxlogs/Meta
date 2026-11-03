package gateway

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"meta/internal/db"
	"meta/internal/upgrade"
)

// 票0044 部署态判三补件（设计-0034-p4 唯一权威）：
//
//	机制一 死席 sentinel.log 网关侧排干——drainDeploySentinel；
//	机制二 deployStateOK「已装」证据换源——exec 日志「已装」行＋装着戳
//	       对期戳为必需件，cmd_run 行降旁证，socket Probe 保留；
//	机制三 席死径挂态判——attach/reconcile 两挂点共用 deployDone 收口序。
//
// 桩缝（生产＝真件，测试经缝替身——「Probe/stamp/日志面经缝桩测」锚件）：
var (
	binVersion   = runBinVersion // <bin> version 输出第二段（装戳/候选戳同径）
	repoCodeHead = gitCodeHead   // repo 内 code-head pathspec 现算
	deployProbe  = upgrade.Probe // socket 探活腿
)

// drainDeploySentinel 死席 sentinel.log 网关侧排干（机制一）：席死＝文件
// 终态，整文件一次读取末条 META_DEPLOY: 行定边——"done"/"failed"；无终
// 哨兵返 ""（交 deployStateOK 兜底）；META_DELIVER 在盘不消费成边（部署
// 票终哨兵＝META_DEPLOY，DELIVER 只是载体）。事件驱动单发——死判时刻
// 一次读，禁扫描轮（红线#1 同构）。
// 对抗锚提名行：末哨兵取行——删/翻则哨兵定边锚红。
func (g *Gateway) drainDeploySentinel(runID string) string {
	b, err := os.ReadFile(filepath.Join(g.cfg.RunDir, "artifacts", runID, "sentinel.log"))
	if err != nil {
		return ""
	}
	last := ""
	for _, ln := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(ln), "META_DEPLOY:"); ok {
			last, _, _ = strings.Cut(strings.TrimSpace(v), " ")
		}
	}
	switch last {
	case "done", "failed":
		return last
	}
	return ""
}

// deployDone 部署实成收口序（机制三共用）：完成边＋deploy_ok 事件＋放
// integrate/deploy 双锁＋消闹钟——两径（attach 席死判/reconcile 对账判）
// 同形，不造第二种完成式。迁移失败返 false 交调用方落兜底径。
// 对抗锚提名行：deploy_ok 锁放行——删/翻则锁释断言锚红。
func (g *Gateway) deployDone(ctx context.Context, tid, note string) bool {
	project := ""
	if t, err := g.database.GetTicket(ctx, tid); err == nil && t != nil {
		project = t.Project
	}
	if err := g.transition(ctx, tid, db.PhaseDone, db.EdgeDeployDone,
		"gateway", note); err != nil {
		return false
	}
	g.emit(Event{Kind: "deploy_ok", TicketID: tid})
	g.releaseProjectLocks(ctx, project, tid, true, lockIntegrate, lockDeploy)
	g.cancelAlarm(tid)
	return true
}

// deployStepsDone T26 病4 完成态判事件化：四步配方（integrate/check/
// deploy/probe）各最近一次 cmd_run 全 exit=0＝部署实成——持久账跨
// restart 存活，证据不依赖席位存活/哨兵尾帧（「席死径绕开判定」与
// 「死席哨兵蒸发」两病的公共根：完成证据绑在席载体上）。
func (g *Gateway) deployStepsDone(ctx context.Context, tid string) bool {
	runs, err := g.database.CmdRuns(ctx, tid)
	if err != nil {
		return false
	}
	last := map[string]int{}
	for _, r := range runs {
		last[r.Step] = r.ExitCode // 后行盖前行＝最近行
	}
	for _, s := range []string{"integrate", "check", "deploy", "probe"} {
		code, ok := last[s] // 缺步非零值——存在性判（漏判=缺步即算齐绿）
		if !ok || code != 0 {
			return false
		}
	}
	return true
}

// maybeDeployDone 事件驱动完成收口（T26）：四步账齐＋相位仍 部署中→
// deploy_done。幂等——相位已走即无操作。
func (g *Gateway) maybeDeployDone(ctx context.Context, tid string) {
	if !g.deployStepsDone(ctx, tid) {
		return
	}
	p, err := g.database.PhaseOf(ctx, tid)
	if err != nil || p != db.PhaseDeploying {
		return
	}
	g.deployDone(ctx, tid, "四步配方齐绿（cmd_run 事件账收口——非席位依赖）")
}

// deployInstall 已装证据（机制二要件1 产出）：install＝装着位锚（已装行
// → 段 或 META_INSTALL/部署脚本缺省位）；newBin＝已装行 <new> 段绝对化
// 后（空＝无行锚，期戳走 code-head 次选源）；repo＝项目 repo_url（相对
// newBin 解析基＋code-head 现算 cwd）。
type deployInstall struct {
	install string
	newBin  string
	repo    string
}

// expected 期戳（机制二要件2，钦定优先序）：首选已装行锚定 NewBin 文件
// version 第二段——同文件对互证，免 git、免 master 漂移；次选 repo 内
// code-head 现算＋-deploy（pathspec 同源 meta-deploy.sh:20）。俱不可得
// →""（判负方向保守）。
func (d *deployInstall) expected() string {
	if d.newBin != "" {
		if s, err := binVersion(d.newBin); err == nil && s != "" {
			return s
		}
	}
	if d.repo != "" {
		if s, err := repoCodeHead(d.repo); err == nil && s != "" {
			return s + "-deploy"
		}
	}
	return ""
}

// deployStateOK 0034-p3 收口态判（0044 换源后三要件联合，全真才判部署
// 实成——restart 杀席是必然不证败）：
//
//	①已装证据：最新 exec 日志「已装」行 或 cmd_run 末次 deploy exit=0
//	  旁证（二取一，俱无→假——D-2 无已装证据仍翻负语义不退）；
//	②装着戳==期戳：<install> version 第二段 == expected()；
//	③socket 探活绿（部署效果独立证据位，Probe 腿保留原样）。
func (g *Gateway) deployStateOK(ctx context.Context, tid string) bool {
	ev := g.deployInstallEvidence(ctx, tid)
	if ev == nil || ev.install == "" {
		return false
	}
	cur, err := binVersion(ev.install)
	if err != nil || cur == "" {
		return false
	}
	// 对抗锚提名行：装戳判等（票0048-p1 双侧剥 +* 尾——+hash 是构建
	// 检出戳非码面身份，次选径 repoCodeHead+"-deploy" 无 hash 对
	// stamp+hash 不剥必假负）——删/翻则「戳不符翻负」锚红。
	if want := ev.expected(); want == "" || stampKey(cur) != stampKey(want) {
		return false
	}
	return deployProbe(g.cfg.SocketPath, 2*time.Second) == nil
}

// stampKey 戳核钥：剥 version 第二段 <stamp>+<vcs短哈希> 的构建哈希
// 尾，判等只比码面 stamp 段（meta-deploy.sh 幂等闸同式剥尾）。
func stampKey(s string) string {
	if i := strings.IndexByte(s, '+'); i >= 0 {
		return s[:i]
	}
	return s
}

// deployInstallEvidence 要件1 取证（二取一）：首选最新 exec 日志「已装」
// 行——跨 restart 存活源，行内自带 install/newbin 锚；次选 cmd_run 末次
// deploy exit=0——非 restart 径仍落库（幂等短路退 0 / RestartCmd="" 径），
// 旁证通道不拆，install 位落 META_INSTALL/部署脚本缺省。俱无→nil。
func (g *Gateway) deployInstallEvidence(ctx context.Context, tid string) *deployInstall {
	repo := ""
	if t, err := g.database.GetTicket(ctx, tid); err == nil && t != nil && t.Project != "" {
		if p, err := g.database.GetProject(ctx, t.Project); err == nil && p != nil {
			repo = p.RepoURL
		}
	}
	if ev := g.deployInstalledLine(tid, repo); ev != nil {
		return ev
	}
	runs, err := g.database.CmdRuns(ctx, tid)
	if err != nil {
		return nil
	}
	var last *db.CmdRun
	for _, r := range runs {
		if r.Step == "deploy" {
			last = r
		}
	}
	if last == nil || last.ExitCode != 0 {
		return nil
	}
	install := os.Getenv("META_INSTALL")
	if install == "" {
		install = "/opt/meta/bin/meta" // meta-deploy.sh INSTALL 缺省同源
	}
	return &deployInstall{install: install, repo: repo}
}

// deployInstalledLine 最新 run/exec/<tid>-deploy-*.log 的末条
// 「upgrade: 已装 <new> → <install>」行取证（upgrade.go:62 钦定格式）：
// 名带 UnixNano 字典序即时间序→glob 取最新份；newBin 相对径相对项目
// repo_url 解析。无文件/无已装行/行形不符→nil。
func (g *Gateway) deployInstalledLine(tid, repo string) *deployInstall {
	files, err := filepath.Glob(filepath.Join(g.cfg.RunDir, "exec",
		tid+"-deploy-*.log"))
	if err != nil || len(files) == 0 {
		return nil
	}
	b, err := os.ReadFile(files[len(files)-1])
	if err != nil {
		return nil
	}
	line := ""
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(ln, "upgrade: 已装 ") {
			line = ln
		}
	}
	if line == "" {
		return nil
	}
	newPart, instPart, ok := strings.Cut(
		strings.TrimPrefix(line, "upgrade: 已装 "), " → ")
	if !ok {
		return nil
	}
	install := strings.TrimSpace(instPart)
	if i := strings.Index(install, "（"); i >= 0 { // 剥（prev=…）尾段
		install = strings.TrimSpace(install[:i])
	}
	if install == "" {
		return nil
	}
	newBin := strings.TrimSpace(newPart)
	if newBin != "" && !filepath.IsAbs(newBin) && repo != "" {
		newBin = filepath.Join(repo, newBin)
	}
	return &deployInstall{install: install, newBin: newBin, repo: repo}
}

// runBinVersion <bin> version 输出第二段＝stamp（meta <stamp> 钦定格式）。
func runBinVersion(path string) (string, error) {
	out, err := exec.Command(path, "version").CombinedOutput()
	if err != nil {
		return "", err
	}
	f := strings.Fields(string(out))
	if len(f) < 2 {
		return "", fmt.Errorf("version 输出不可判: %q", out)
	}
	return f[1], nil
}

// gitCodeHead 部署码面 code-head 现算（pathspec 同源 meta-deploy.sh:20）：
// 空→rev-parse --short HEAD 兜底（脚本同式）。
func gitCodeHead(repo string) (string, error) {
	out, err := exec.Command("git", "-C", repo, "log", "-1", "--format=%h", "--",
		"*.go", "proto", "cmd", "internal", "scripts", "go.mod", "go.sum").Output()
	if err != nil {
		return "", err
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		return s, nil
	}
	out, err = exec.Command("git", "-C", repo, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
