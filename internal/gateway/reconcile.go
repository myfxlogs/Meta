package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// ---------- v2 §3.3-A 幂等调和器＋§3.3-B 失败表 ----------
//
// reconcile() 是全仓唯一的 spawn 决策点：每 60s（或任意事件后经 kick）跑
// 一轮。对每张非终态票：
//   desired = f(相位,失败表) —— 该相位应有的角色，或应停（无行动者/blocked）
//   actual  = 活 run（SIGCHLD/EOF/进程已消的不算）
//   desired==actual → 跳过
//   desired==NONE   → 跳过（冻/泊车/blocked 零动作）
//   desired==STOP   → blockTicket（一条通知，此后零动作）
//   actual 缺 && 本相位期窗内同类失败数 < 失败表上限 → DispatchSeat 起席
// 幂等：同一世界态跑 N 次产出相同——spawn 由 DispatchSeat 活席去重闸挡，
// blocked 由 transition 边闸挡（已在 blocked 即拒）。

// FailureRule 失败表行（config 解析后的运行形态；kind="*" 兜底）。
type FailureRule struct {
	Kind   string
	Action string // next_shell|retry_same|shell_dead_next|kill_respawn|same_shell|rollback|freeze|blocked
	Cap    int
}

// failureClasses 期窗内计数的失败类事件 kind（reconcile 读事件推导）。
// 白名单外 kind 一律按 "*" 行处置（未知错误 0 次直接 blocked）。
var failureClasses = map[string]bool{
	"seat_err": true, "spawn_failed": true, "shells_exhausted": true,
	"shell_limited": true, "shell_dead": true, "stall_suspect": true,
	"reject": true, "deploy_failed": true, "budget_frozen": true,
	"clidrive_dead": true, "stall_kill": true, "seat_dead": true,
}

// edgeKindSet 相位边 kind 集——期窗锚点（最近一条边之后才算本相位期窗）。
var edgeKindSet = func() map[string]bool {
	m := map[string]bool{}
	for _, e := range db.Edges {
		m[string(e.Kind)] = true
	}
	return m
}()

// ruleFor 失败表查行；未列 kind 落 "*" 行（未知→blocked）。
func (g *Gateway) ruleFor(kind string) FailureRule {
	var star *FailureRule
	for i := range g.cfg.FailureRules {
		r := &g.cfg.FailureRules[i]
		if r.Kind == kind {
			return *r
		}
		if r.Kind == "*" {
			star = r
		}
	}
	if star != nil {
		return *star
	}
	return FailureRule{Kind: "*", Action: "blocked", Cap: 0}
}

// epochEvents 本相位期窗事件切片：最近一条边事件之后的全部行（倒回扫描——
// blocked/resume 等机制边也算锚点，失败后人工放行计数清零重启）。
func epochEvents(evs []*db.Event) []*db.Event {
	for i := len(evs) - 1; i >= 0; i-- {
		// T42：人工裁决 ruling 行同构复位（T30 cli-drive 复位径钦定同
		// 构）——resume 已含于边锚；ruling 非边须显式列，否则裁决后旧账
		// 仍超 cap→秒回 blocked（0050 实证）。
		if edgeKindSet[evs[i].Kind] || evs[i].Kind == "ruling" {
			return evs[i+1:]
		}
	}
	return evs
}

// failCount 期窗内某失败类计数。
func failCount(epoch []*db.Event, kind string) int {
	n := 0
	for _, e := range epoch {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// seatDeadExempt T37 交付后退场豁免（0050 实证）：同票最近一条 deliver
// 边在最后一笔失败类事件之后＝席交付功成正常退场，seat_dead 不入失败账。
// 未交付而死（无 deliver 或 deliver 后又有失败类）照旧计。
func seatDeadExempt(evs []*db.Event) bool {
	edge, fail := -1, -1
	for i, e := range evs {
		if edgeKindSet[e.Kind] {
			edge = i
		}
		if failureClasses[e.Kind] {
			fail = i
		}
	}
	// 豁免域＝最近一条边恰是 deliver 且其后无失败类——交付席退场。
	// 之后新边（audit_start 等）已翻期：后序期席死照旧计。
	return edge >= 0 && evs[edge].Kind == string(db.EdgeDeliver) && fail < edge
}

// lastFailure 期窗内最近一次失败类 kind（无→""）。
func lastFailure(epoch []*db.Event) string {
	for i := len(epoch) - 1; i >= 0; i-- {
		if failureClasses[epoch[i].Kind] {
			return epoch[i].Kind
		}
	}
	return ""
}

// failedShells 期窗内失败壳键集（取代 g.failSet 内存表——由 seat_run 持久账
// 推导，重启零丢失）：本相位期窗内死掉的非 adjunct run 的壳名全进排除集。
func (g *Gateway) failedShells(ctx context.Context, t *db.Ticket, epochStart time.Time) map[string]bool {
	out := map[string]bool{}
	rs, err := g.database.RunsForTicket(ctx, t.ID)
	if err != nil {
		return out
	}
	for _, r := range rs {
		if r.Adjunct || !r.Dead || r.Shell == "" {
			continue
		}
		if !r.ExitedAt.IsZero() && r.ExitedAt.Before(epochStart) {
			continue // 死在本相位期窗前的不计（无死时戳的死行照计）
		}
		out[shellNameOf(r.Shell)] = true
	}
	return out
}

// desiredFor 该票应有行动者（纯函数，相位×票面→角色）。
// 返回 ("",false)=无行动者（该相位不持席）；(role,true)=应有一席。
func desiredFor(t *db.Ticket) (string, bool) {
	switch t.Phase {
	case db.PhaseCreated:
		return "CTO", true // CTO-A 设计支
	case db.PhaseDesigning:
		return "CTO", true
	case db.PhaseAssigned:
		return "施工", true
	case db.PhaseWorking:
		return "", true // 行动者角色由最近 run 定（施工/CTO 混相）——调用方补
	case db.PhaseReview:
		return "审计", true
	case db.PhaseAuditing:
		return "审计", true
	case db.PhaseToDeploy:
		return "CTO", true // CTO-B 部署支（cli_drive 径）
	case db.PhaseDeploying:
		return "CTO", true
	}
	return "", false
}

// parkedTicket 泊车判：相位 已立 且最新事件是 req_pending——需求待裁摆位，
// 调和器不动（等 redirect/人工）。
func (g *Gateway) parkedTicket(ctx context.Context, t *db.Ticket) bool {
	if t.Phase != db.PhaseCreated {
		return false
	}
	k, err := g.database.LatestEventKind(ctx, t.ID)
	return err == nil && k == string(db.EdgeReqPending)
}

// blockTicket 置 blocked 人工态（失败表到限唯一出口）：非法边/已 blocked
// 即幂等退——一条 emit 通知，此后系统对该票零动作直到人回话（V5：同票同
// 原因通知恰一条由边闸天然保证——第二次 transition 直接拒）。
func (g *Gateway) blockTicket(ctx context.Context, ticketID, reason string) {
	if err := g.transition(ctx, ticketID, db.PhaseBlocked, db.EdgeBlocked,
		"gateway", reason); err != nil {
		return
	}
	note := "票 blocked：" + reason + "——可选动作：放行(resume)/重试/作废(void)"
	_ = g.database.AppendEvent(ctx, ticketID, "blocked", db.Event{
		Actor: "gateway", Note: note})
	g.emit(Event{Kind: "blocked", TicketID: ticketID, Note: reason})
}

// reconcile 幂等调和谐：全相位扫描，desired↔actual 对齐。
func (g *Gateway) reconcile(ctx context.Context) {
	tickets, err := g.database.NonTerminalTickets(ctx)
	if err != nil {
		return
	}
	// 部署批量闸参数（T4/E17 语义并入调和器）：待部署≥5 或流水线清空才放
	// CTO-B 起席——不足数且流水线忙时 desired 挂起不派（每轮重算，幂等）。
	pendingDeploy := 0
	pipelineBusy := false
	for _, t := range tickets {
		switch t.Phase {
		case db.PhaseToDeploy:
			pendingDeploy++
		case db.PhaseWorking, db.PhaseDesigning:
			pipelineBusy = true
		}
	}
	deployGateOpen := pendingDeploy >= 5 || !pipelineBusy

	for _, t := range tickets {
		if t.Phase == db.PhaseBlocked || t.BudgetFrozen {
			continue // 人工态/冻票零动作
		}
		if g.parkedTicket(ctx, t) {
			continue
		}
		role, wantSeat := desiredFor(t)
		if !wantSeat {
			continue
		}
		if t.Phase == db.PhaseToDeploy && !deployGateOpen {
			continue // 批部署闸：不足批且流水线忙——挂起不派
		}
		g.reconcileTicket(ctx, t, role)
	}
	// D-55 孤儿收尸：adjunct 席/终态票残留席 pid 死→dead=0 悬挂，连尸收。
	if runs, err := g.database.ActiveRuns(ctx); err == nil {
		for _, r := range runs {
			if r.PID <= 0 || processAlive(r.PID) {
				continue
			}
			_ = g.database.MarkDead(ctx, r.RunID)
			killRunGroup(r.PID)
			g.wipeSecretsDir(r.RunID)
			g.cleanSeatResidue(r)
			kind := "seat_dead"
			if r.Adjunct {
				kind = "adjunct_dead"
			}
			_ = g.database.AppendEvent(ctx, r.TicketID, kind, db.Event{
				Actor: "gateway", Note: "对账收尸: pid 已死 " + r.RunID})
			g.emit(Event{Kind: kind, TicketID: r.TicketID, RunID: r.RunID, Role: r.Role})
		}
	}
	// B1 设计锁重建（§1.8）：设计链在途票有活 CTO run→照相位恢复持锁。
	for _, t := range tickets {
		switch t.Phase {
		case db.PhaseDesigning, db.PhaseAssigned, db.PhaseWorking:
		default:
			continue
		}
		if t.Project == "" {
			continue
		}
		if r, err := g.database.ActiveRunForTicketRole(ctx, t.ID, "CTO"); err == nil && r != nil {
			g.designLocks[t.Project] = t.ID
		}
	}
}

// reconcileTicket 单票对齐：actual 活着收养/跳过；缺则按失败表起或停。
func (g *Gateway) reconcileTicket(ctx context.Context, t *db.Ticket, role string) {
	seatDied := false
	run, _ := g.database.ActiveRunForTicket(ctx, t.ID)
	if run != nil {
		g.mu.Lock()
		inFlight := g.runs[run.RunID] != nil
		g.mu.Unlock()
		// actual 存在＝进程活着（SIGCHLD/EOF 前的在管态）——desired==actual 对齐；
		// 未接管的重启遗留 run 收养一次（对账自产 adopted 事件只此一处，幂等）。
		if inFlight || (run.PID > 0 && processAlive(run.PID)) {
			if !inFlight && g.tryAdopt(ctx, run) {
				_ = g.database.MarkAdopted(ctx, run.RunID)
				st := newRunState(run)
				st.adopted = true
				g.mu.Lock()
				g.runs[run.RunID] = st
				g.mu.Unlock()
				_ = g.database.AppendEvent(ctx, t.ID, "adopted", db.Event{
					Actor: "gateway", Note: "对账收养 " + run.RunID})
				g.emit(Event{Kind: "adopted", TicketID: t.ID, RunID: run.RunID, Role: run.Role})
			}
			// 收养口不通但进程在＝保守跳过（不杀不派——避免误杀活席）。
			// 收养席的进度帧超时（闹钟随重启失）由期窗 stall 账顶：
			// LastActivityAt+ProgressTimeout 过期→stall_suspect→失败表判。
			if la, err := g.database.LastActivityAt(ctx, t.ID); err == nil &&
				!la.IsZero() && time.Since(la) > g.cfg.ProgressTimeout {
				g.onStall(t.ID)
			}
			return
		}
		// 进程已消＝actual 缺——标死放新席。
		_ = g.database.MarkDead(ctx, run.RunID)
		seatDied = true
	}
	// actual 缺。MarkDead↔seat_dead 落账竞序兜底：最近 run 死账在
	// 即本相位有席死过——kind 回退 seat_dead，排除集换壳保真。
	if lr, err := g.database.LatestRunForTicket(ctx, t.ID); err == nil && lr != nil && lr.Dead {
		seatDied = true
	}
	// Working 相位行动者角色由最近 run 血统定（非 adjunct）。
	// T45：血统仅采可派角色——CTO 在施工中不可派（DispatchSeat 硬拒
	// 静默吞＝零新席零事件，0052 手踢实证）。不可派血统退化「施工」
	// ——该相位唯一可起席的角色，血统角只负责保施工/审计血统。
	if t.Phase == db.PhaseWorking {
		role = "施工"
		if er, err := g.database.LatestRunForTicket(ctx, t.ID); err == nil && er != nil &&
			er.Role != "" && er.Role != "CTO" {
			role = er.Role
		}
	}
	// 部署中席死先排干哨兵：done/账齐/态判→deploy_done 完成边（非失败）。
	if t.Phase == db.PhaseDeploying && g.deployDyingTicketClose(ctx, t, run) {
		return
	}
	// 失败表：期窗内同类失败计数 vs 行上限。
	evs, _ := g.database.Events(ctx, t.ID)
	epoch := epochEvents(evs)
	epochStart := time.Time{}
	if len(evs) > len(epoch) {
		epochStart = evs[len(evs)-len(epoch)-1].CreatedAt
	}
	kind := lastFailure(epoch)
	if kind == "seat_dead" && seatDeadExempt(evs) {
		// T37：deliver 已落账后席退场＝功成身退——不入失败账、不判 blocked。
		kind = ""
	}
	if kind == "" && seatDied && !seatDeadExempt(evs) {
		// MarkDead↔seat_dead 落账竞序面：死行本身即凭——本轮死席照换壳，
		// 防路过调和拿空排除集重放同壳后判重锁死。
		kind = "seat_dead"
	}
	rule := g.ruleFor(kind)
	if kind != "" {
		n := failCount(epoch, kind)
		switch {
		case rule.Action == "blocked":
			g.blockTicket(ctx, t.ID, "未知/禁动失败 kind="+kind+"——人工裁定")
			return
		case rule.Cap > 0 && n > rule.Cap && rule.Action != "retry_same" &&
			rule.Action != "next_shell" && rule.Action != "shell_dead_next":
			// 有帽动作（same_shell/kill_respawn/rollback/freeze）到限→blocked
			g.blockTicket(ctx, t.ID, fmt.Sprintf("失败表到限 kind=%s 计数=%d>cap=%d：%s",
				kind, n, rule.Cap, rule.Action))
			return
		case rule.Action == "retry_same" && n > rule.Cap:
			rule.Action = "next_shell" // 同壳退避用尽→换壳
		}
	}
	// 行动→spec 推导（纯函数：票面+角色+项目配方，重启零丢失）。
	spec := g.deriveSpec(ctx, t, role, rule, epoch, epochStart)
	// 去重/相位闸拒＝已对齐（活席在或相位已走）——静默收敛不落账。
	_ = g.DispatchSeat(ctx, t.ID, role, spec)
}

// deployDyingTicketClose 部署中死席收口判（0044 排干语义并入调和器）：
// 哨兵 done／四步账齐／态判绿→deploy_done；failed 哨兵在盘→失败表 deploy
// 路径照走。返回 true＝票已收口不再走派径。
func (g *Gateway) deployDyingTicketClose(ctx context.Context, t *db.Ticket, run *db.Run) bool {
	sentinel := ""
	if run != nil {
		sentinel = g.drainDeploySentinel(run.RunID)
	}
	switch {
	case sentinel == "done":
		return g.deployDone(ctx, t.ID, "调和排干：META_DEPLOY:done 在盘（席死）")
	case sentinel == "" && g.deployStepsDone(ctx, t.ID):
		return g.deployDone(ctx, t.ID, "调和账判：四步 cmd_run 齐绿（哨兵缺席不挡收口）")
	case sentinel == "" && g.deployStateOK(ctx, t.ID):
		return g.deployDone(ctx, t.ID, "调和态判：已装+socket健康→deploy_ok")
	case sentinel == "failed":
		// 终哨兵负＝回滚已在四步径执行→deploy_failed 账＋blocked。
		note := "调和排干：哨兵在盘 META_DEPLOY:failed——回滚已执行"
		_ = g.database.AppendEvent(ctx, t.ID, "deploy_failed", db.Event{
			Actor: "gateway", Note: note})
		g.emit(Event{Kind: "deploy_failed", TicketID: t.ID})
		g.blockTicket(ctx, t.ID, note)
		return true
	case sentinel == "":
		// 席死无据→deploy_failed 账留给期窗计数（rollback≤1 由表判）。
		_ = g.database.AppendEvent(ctx, t.ID, "deploy_failed", db.Event{
			Actor: "gateway", Note: "席死无终哨兵无据——先核回滚态"})
	}
	return false
}

// deriveSpec 派发参数纯推导（v2 §3.3-A）：(票面,角色,项目配方,失败表行)→
// SeatSpec——无内存表依赖，重启零丢失。WorkDir＝项目 repo_url 或 /opt/meta；
// CliDrive＝部署支钦定径；PromptFile 空→走注入机组装。
func (g *Gateway) deriveSpec(ctx context.Context, t *db.Ticket, role string,
	rule FailureRule, epoch []*db.Event, epochStart time.Time) SeatSpec {
	spec := SeatSpec{Provenance: "reconcile"}
	spec.WorkDir = "/opt/meta"
	if t.Project != "" {
		if p, err := g.database.GetProject(ctx, t.Project); err == nil && p != nil && p.RepoURL != "" {
			spec.WorkDir = p.RepoURL
		}
	}
	if role == "CTO" && t.Phase == db.PhaseToDeploy {
		spec.CliDrive = true // T26：部署支钦定机械四步径
	}
	switch rule.Action {
	case "same_shell", "retry_same":
		// 同壳重做/退避同壳：prefer＝本票最近非 adjunct run 壳名。
		if rs, err := g.database.RunsForTicket(ctx, t.ID); err == nil {
			for i := len(rs) - 1; i >= 0; i-- {
				if !rs[i].Adjunct && rs[i].Shell != "" {
					spec.PreferShell = shellNameOf(rs[i].Shell)
					break
				}
			}
		}
	case "next_shell", "kill_respawn", "shell_dead_next":
		// 换壳：本相位期窗死壳名入排除集（failSet 退役——DB 推导）。
		spec.Env = nil
		if failed := g.failedShells(ctx, t, epochStart); len(failed) > 0 {
			spec.exclude = failed
		}
	}
	return spec
}

// runIDOf 防御小件。
func runIDOf(r *db.Run) string {
	if r == nil {
		return ""
	}
	return r.RunID
}

// tryAdopt 收养互证（§5.1 D2）：连 per-seat UDS，首帧出示 run_token——
// 它发的它知道。单次有界尝试：连不上/拒收＝行动者判死，不重试不轮询。
// T35：收养续命 armAlarm 径已删——收养席的进度超时由调和器周期查
// LastActivityAt 顶账（stall_suspect 失败表行接管）。
func (g *Gateway) tryAdopt(ctx context.Context, run *db.Run) bool {
	conn, err := grpc.NewClient("unix:"+run.SocketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return false
	}
	defer conn.Close()
	cctx, cancel := context.WithTimeout(ctx, g.cfg.AdoptDialTimeout)
	defer cancel()
	ack, err := pbv1.NewSeatAdoptClient(conn).Adopt(cctx, &pbv1.AdoptAuth{RunToken: run.RunToken})
	if err != nil || ack == nil || !ack.Accepted {
		g.emit(Event{Kind: "adopt_failed", TicketID: run.TicketID, RunID: run.RunID})
		return false
	}
	return true
}

var _ = strings.TrimSpace // 占位防未用（strings 供后续判读需要）
var _ = runIDOf

// budgetWarned 本尝试已发过 budget_warn 判——锚点＝票上最近一条 run 的
// StartedAt（幂等 claim 不重发边，claim 锚不可依赖）；事件账即边沿触发源。
func (g *Gateway) budgetWarned(ctx context.Context, ticketID string) bool {
	base := time.Time{}
	if r, err := g.database.LatestRunForTicket(ctx, ticketID); err == nil && r != nil {
		base = r.StartedAt
	}
	evs, err := g.database.Events(ctx, ticketID)
	if err != nil {
		return false
	}
	for _, e := range evs {
		if e.Kind == "budget_warn" && e.CreatedAt.After(base) {
			return true
		}
	}
	return false
}
