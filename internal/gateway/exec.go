package gateway

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// 项目三锁（B1）：project → 锁种集合。design 锁挂「已立→role=CTO 派发」点，
// integrate/deploy 锁挂部署支（待部署→CTO 派发）点。锁＝相位占用语义：
// 启动按票相位重建（设计中→design，部署中→integrate+deploy）。
const (
	lockDesign    = "design"
	lockIntegrate = "integrate"
	lockDeploy    = "deploy"
)

// deployRunSteps 部署支序驱的四步（配方步名；rollback 是失败自驱步）。
// ExecReq.step 必须落在 Project.CmdFor 已声明的配方命令上——配方外不得代跑。

// rebuildProjectLocks 启动按票相位重建内存锁（New 内、Reconcile 后一次）。
func (g *Gateway) rebuildProjectLocks(ctx context.Context) {
	tickets, err := g.database.NonTerminalTickets(ctx)
	if err != nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, t := range tickets {
		switch t.Phase {
		case db.PhaseDeploying:
			g.projectLocks[t.Project] = map[string]bool{lockIntegrate: true, lockDeploy: true}
		}
	}
}

// acquireProjectLocks 全取或全不取（原子——防同项目并发部署各占一半）。
// 取到返回 true；任一种已占用→取失败且不留残。
func (g *Gateway) acquireProjectLocks(project string, kinds ...string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	held := g.projectLocks[project]
	for _, k := range kinds {
		if held[k] {
			return false
		}
	}
	if held == nil {
		held = map[string]bool{}
		g.projectLocks[project] = held
	}
	for _, k := range kinds {
		held[k] = true
	}
	return true
}

// releaseProjectLocks 放锁＋（backfill 时）查同项目候票补发 spawn_needed
// （事件驱动解锁，零定时）。design 释放→同项目 已立 票（CTO 设计支）；
// integrate/deploy 释放→同项目 待部署 票（CTO-B 部署支）。只发实际放掉的
// 锁种——没占锁的调用不产噪音。skipTicket 排除刚 deploy_failed 的本票
// （回 待部署＝CEO 裁径，不是自动重试闸）。
func (g *Gateway) releaseProjectLocks(ctx context.Context, project, skipTicket string, backfill bool, kinds ...string) {
	g.mu.Lock()
	held := g.projectLocks[project]
	var freed []string
	for _, k := range kinds {
		if held[k] {
			delete(held, k)
			freed = append(freed, k)
		}
	}
	if len(held) == 0 {
		delete(g.projectLocks, project)
	}
	g.mu.Unlock()
	if len(freed) == 0 || !backfill {
		return
	}
	// 候补相位集：设计锁候 已立，部署锁候 待部署。
	var waitPhases []db.Phase
	for _, k := range freed {
		if k == lockDesign {
			waitPhases = append(waitPhases, db.PhaseCreated)
		} else {
			waitPhases = append(waitPhases, db.PhaseToDeploy)
		}
	}
	tickets, err := g.database.TicketsByProjectPhase(ctx, project, waitPhases)
	if err != nil {
		return
	}
	for _, t := range tickets {
		if t.ID == skipTicket {
			continue
		}
		_ = g.database.AppendEvent(ctx, t.ID, "spawn_needed", db.Event{
			Actor: "gateway", Note: "锁释补发: CTO"})
		g.emit(Event{Kind: "spawn_needed", TicketID: t.ID, Role: "CTO"})
	}
}

// handleExec 命令代跑闸（票0010 §10/D1——席位直碰部署树的唯一径，禁区#8）：
// 三重闸 role==CTO＋票相位=部署中＋非 adjunct，缺一即拒（拒＝exec_resp 负码
// 回席不断流＋exec_denied 事件留痕——让部署席自己裁下一步）。过闸→查项目
// 配方→沙箱 run_as→sudo -n 代跑→cmd_run 落痕→exec_resp 回席。
func (g *Gateway) handleExec(ctx context.Context, st *runState, stream pbv1.SeatGateway_AttachServer, req *pbv1.ExecReq) error {
	deny := func(why string) error {
		_ = g.database.AppendEvent(ctx, st.run.TicketID, "exec_denied", db.Event{
			Actor: st.run.RunID, Note: "step=" + req.Step + " " + why})
		g.emit(Event{Kind: "exec_denied", TicketID: st.run.TicketID, RunID: st.run.RunID,
			Role: st.run.Role, Note: why})
		return stream.Send(&pbv1.GatewayMsg{Body: &pbv1.GatewayMsg_ExecResp{
			ExecResp: &pbv1.ExecResp{Step: req.Step, ExitCode: -1, Note: why}}})
	}
	if st.run.Adjunct || st.run.Role != "CTO" {
		return deny(fmt.Sprintf("exec 闸拒：role=%s adjunct=%v（部署支仅 CTO 正席）", st.run.Role, st.run.Adjunct))
	}
	phase, err := g.database.PhaseOf(ctx, st.run.TicketID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	// 对抗性锚提名行：相位判等——非 部署中 不得代跑（翻成 != 则闸反）。
	if phase != db.PhaseDeploying {
		return deny("exec 闸拒：票相位 " + string(phase) + " 非 部署中")
	}
	g.armAlarm(st.run.TicketID, time.Now().Add(g.cfg.ProgressTimeout)) // exec 是活性帧

	t, err := g.database.GetTicket(ctx, st.run.TicketID)
	if err != nil || t == nil {
		return status.Error(codes.Internal, "取票失败")
	}
	proj, err := g.database.GetProject(ctx, t.Project)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if proj == nil {
		return deny("项目 " + t.Project + " 无配方登记")
	}
	cmd, ok := proj.CmdFor(req.Step)
	if !ok {
		return deny("配方无步 " + req.Step + "——配方外命令不得代跑")
	}
	sb, err := parseSandbox(proj.SandboxSpec)
	if err != nil {
		return deny("sandbox_spec 解析: " + err.Error())
	}
	runAs := runAsFor(sb, req.Step)
	if runAs == "" {
		return deny("沙箱形制未声明 run_as——代跑禁网关自身用户（D1）")
	}
	ref := req.Ref
	if ref == "" {
		ref = g.lastDeliverRef(ctx, st.run.TicketID)
	}
	resp := g.runExecStep(ctx, st, proj, sb, req.Step, cmd, runAs, ref)
	return stream.Send(&pbv1.GatewayMsg{Body: &pbv1.GatewayMsg_ExecResp{ExecResp: resp}})
}

// runExecStep 执行一步：干净 clone（runner 以 run_as 自 clone 交付 ref）→
// sudo -n -u <run_as> -- env … bash -lc <cmd>（cwd=clone）→一次性闹钟杀→
// 退码+输出文件+尾段+diff 文件清单→cmd_run+exec 事件。
func (g *Gateway) runExecStep(ctx context.Context, st *runState, proj *db.Project, sb *pbv1.SandboxSpec, step, cmdline, runAs, ref string) *pbv1.ExecResp {
	tid := st.run.TicketID
	workDir, cerr := g.ensureDeployClone(ctx, tid, runAs, proj.RepoURL, ref)
	rec := func(code int, outPath, tail, files, note string) *pbv1.ExecResp {
		_ = g.database.InsertCmdRun(ctx, &db.CmdRun{
			TicketID: tid, RunID: st.run.RunID, Step: step, ExitCode: code,
			OutPath: outPath, Tail: tail, DiffFiles: files})
		_ = g.database.AppendEvent(ctx, tid, "exec", db.Event{
			Actor: st.run.RunID,
			Note:  fmt.Sprintf("step=%s run_as=%s exit=%d out=%s %s", step, runAs, code, outPath, note)})
		g.emit(Event{Kind: "exec", TicketID: tid, RunID: st.run.RunID, Role: st.run.Role,
			Note: fmt.Sprintf("step=%s exit=%d", step, code)})
		return &pbv1.ExecResp{Step: step, ExitCode: int32(code), OutPath: outPath, Tail: tail, Note: note}
	}
	if cerr != nil {
		return rec(-1, "", cerr.Error(), "", "clone 失败——fail loud 不静默降级")
	}

	outPath := filepath.Join(g.cfg.RunDir, "exec",
		fmt.Sprintf("%s-%s-%d.log", tid, step, time.Now().UnixNano()))
	outf, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return rec(-1, "", "输出文件: "+err.Error(), "", "exec 起不来")
	}
	defer outf.Close()

	timeout := g.cfg.ExecTimeout
	if sb.GetExecTimeout().AsDuration() > 0 {
		timeout = sb.GetExecTimeout().AsDuration()
	}
	argv := []string{"-n", "-u", runAs, "--",
		"env", "META_TICKET_ID=" + tid, "META_EXEC_STEP=" + step,
		"META_EXEC_REF=" + ref, "META_PROJECT=" + proj.ID,
		"META_GATEWAY_SOCK=" + g.cfg.SocketPath,
		"bash", "-lc", cmdline}
	cmd := exec.CommandContext(ctx, g.cfg.SudoBin, argv...)
	cmd.Dir = workDir
	cmd.Stdout, cmd.Stderr = outf, outf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return rec(-1, "", err.Error(), "", "exec 起不来")
	}
	// 一次性闹钟杀：超时杀整进程组（sudo 链下的配方命令同组）。
	killed := make(chan struct{})
	timer := time.AfterFunc(timeout, func() {
		close(killed)
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	})
	werr := cmd.Wait()
	timer.Stop()
	select {
	case <-killed:
		return rec(-1, outPath, tailOfFile(outPath, 4096), "", "超时杀（一次性闹钟）")
	default:
	}
	code := 0
	if werr != nil {
		code = -1
		var ee *exec.ExitError
		if errors.As(werr, &ee) {
			code = ee.ExitCode()
		}
	}
	return rec(code, outPath, tailOfFile(outPath, 4096), g.diffFiles(workDir), "")
}

// ensureDeployClone 部署 cwd＝干净 clone（run/deploy/<tid>，runner 以 run_as
// 自 clone 交付 ref）：已存在即复用（同一部署支的四步共享）；repo_url 空＝
// 干净空目录（无仓配方）。clone/checkout 同样走 sudo -n——禁网关自身用户写仓。
func (g *Gateway) ensureDeployClone(ctx context.Context, tid, runAs, repoURL, ref string) (string, error) {
	dir := filepath.Join(g.cfg.RunDir, "deploy", tid)
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir, nil
	}
	if repoURL == "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		return dir, nil
	}
	for _, a := range [][]string{
		{"-n", "-u", runAs, "--", "git", "clone", "-q", repoURL, dir},
		{"-n", "-u", runAs, "--", "git", "-C", dir, "checkout", "-q", ref},
	} {
		if a[len(a)-1] == "" { // ref 空→只 clone 不 checkout
			continue
		}
		c := exec.CommandContext(ctx, g.cfg.SudoBin, a...)
		if out, err := c.CombinedOutput(); err != nil {
			return "", fmt.Errorf("clone: %w %s", err, tailOf(string(out), 512))
		}
	}
	return dir, nil
}

// diffFiles 步后 clone 的文件变动清单（git status --porcelain——透明非闸；
// 非 git 目录/权限失败均返回空，不阻断代跑径）。
func (g *Gateway) diffFiles(dir string) string {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		return ""
	}
	var files []string
	for _, ln := range strings.Split(string(out), "\n") {
		if len(ln) > 3 {
			files = append(files, strings.TrimSpace(ln[3:]))
		}
	}
	return strings.Join(files, ",")
}

// lastDeliverRef 该票最近 deliver 事件 note 首段＝交付 ref（Deliver.ref 的
// 落库位置——d.Ref+" "+d.Note）。席未附 ref 时的机械取证径。
func (g *Gateway) lastDeliverRef(ctx context.Context, ticketID string) string {
	evs, err := g.database.Events(ctx, ticketID)
	if err != nil {
		return ""
	}
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Kind == string(db.EdgeDeliver) {
			f, _, _ := strings.Cut(evs[i].Note, " ")
			return f
		}
	}
	return ""
}

// parseSandbox project.sandbox_spec（prototext）→ SandboxSpec。
func parseSandbox(s string) (*pbv1.SandboxSpec, error) {
	sb := &pbv1.SandboxSpec{}
	if strings.TrimSpace(s) == "" {
		return sb, nil
	}
	if err := prototext.Unmarshal([]byte(s), sb); err != nil {
		return nil, err
	}
	return sb, nil
}

// runAsFor 步→执行身份：按步覆盖优先，缺省 run_as 兜底。
func runAsFor(sb *pbv1.SandboxSpec, step string) string {
	if u := sb.GetStepRunAs()[step]; u != "" {
		return u
	}
	return sb.GetRunAs()
}

// tailOfFile 读文件尾段（≤n 字节）。
func tailOfFile(path string, n int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return ""
	}
	off := int64(0)
	if fi.Size() > n {
		off = fi.Size() - n
	}
	if _, err := f.Seek(off, 0); err != nil {
		return ""
	}
	buf := make([]byte, fi.Size()-off)
	if _, err := f.Read(buf); err != nil && len(buf) == 0 {
		return ""
	}
	return string(buf)
}

func tailOf(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
