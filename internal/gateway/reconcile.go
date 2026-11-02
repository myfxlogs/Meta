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
	"meta/internal/upgrade"
)

// reconcile 启动对账（§5.3 全相位「相位×预期行动者」表，启动一次非周期）：
// 有行动者相位——活着收养、死了回队；无行动者相位——补发 spawn 事件。
// AdminGateway.Reconcile RPC 即调本函数（同径幂等，meta reconcile 手动径）。
func (g *Gateway) reconcile(ctx context.Context) {
	tickets, err := g.database.NonTerminalTickets(ctx)
	if err != nil {
		return
	}
	for _, t := range tickets {
		if t.BudgetFrozen { // 冻票无活性：不收养不回队不补 spawn（§5.5 D8）
			g.mu.Lock()
			g.frozen[t.ID] = true
			g.mu.Unlock()
			continue
		}
		switch t.Phase {
		case db.PhaseAssigned, db.PhaseReview, db.PhaseToDeploy:
			// 无行动者等 spawn：补发下一步 spawn 事件。
			// 半结构约定（防漂，§14）：note「对账补发: <支线>」
			// （施工/审计/CTO-A 设计支/CTO-B 部署支）是播报员路由表
			// spawn_needed+note_contains 行的消费契约——改文案必须同步改表。
			// patrol-* 值班票恒巡检（票0030 实证：phase 映射不含巡逻角色）。
			role := roleForPhase(t.Phase)
			line := spawnLine(t.Phase)
			if strings.HasPrefix(t.ID, "patrol-") {
				role, line = "巡检", "巡检"
			}
			_ = g.database.AppendEvent(ctx, t.ID, "spawn_needed", db.Event{
				Actor: "gateway", Note: "对账补发: " + line,
			})
			g.emit(Event{Kind: "spawn_needed", TicketID: t.ID, Role: role})
		case db.PhaseWorking, db.PhaseDesigning, db.PhaseAuditing, db.PhaseDeploying:
			g.reconcileActor(ctx, t)
		case db.PhaseCreated:
			// 已立（§5.3 钦定行）：等 CTO-A 唤起，补发 spawn 事件。
			// req_pending 泊车票机械抑制（§1.4）：最新事件为 req_pending 的票
			// 是「需求待裁」摆位，补发会无头重派空转——跳过，等 CEO redirect。
			if k, err := g.database.LatestEventKind(ctx, t.ID); err == nil &&
				k == string(db.EdgeReqPending) {
				continue
			}
			_ = g.database.AppendEvent(ctx, t.ID, "spawn_needed", db.Event{
				Actor: "gateway", Note: "对账补发: " + spawnLine(t.Phase),
			})
			// patrol-* 值班票恒巡检（D-54 同径，已立行漏覆写——票0039
			// 锚③）：立票即唤 CTO 会把系统票喂进业务流水线。
			role := "CTO"
			if strings.HasPrefix(t.ID, "patrol-") {
				role = "巡检"
			}
			g.emit(Event{Kind: "spawn_needed", TicketID: t.ID, Role: role})
		}
	}
	// D-55 孤儿收尸：行动者对账只覆盖票上相位行动者——adjunct 席与
	// 终态票残留席 pid 死后 dead=0 永续悬挂，adjunct 去重闸还会拿
	// 尸体永久挡新席派发（巡检 adjunct pid 死两行挂 dead=0 实证）。
	// 非 adjunct 相位行动者已由上方票环处置（回队语义在），此处只收
	// 票环漏网的：adjunct 席 + 终态/无票席。
	runs, err := g.database.ActiveRuns(ctx)
	if err == nil {
		for _, r := range runs {
			if r.PID <= 0 || processAlive(r.PID) {
				continue
			}
			_ = g.database.MarkDead(ctx, r.RunID)
			killRunGroup(r.PID)
			// T18②：孤儿席（不走 onRunDead）secrets 目录连收——
			// 凭据作用域=run 生命周期，收尸漏净属凭据残面；
			// wipeSecretsDir 先回权限位再删。
			g.wipeSecretsDir(r.RunID)
			kind := "seat_dead"
			if r.Adjunct {
				kind = "adjunct_dead"
			}
			_ = g.database.AppendEvent(ctx, r.TicketID, kind, db.Event{
				Actor: "gateway", Note: "对账收尸: pid 已死 " + r.RunID,
			})
			g.emit(Event{Kind: kind, TicketID: r.TicketID, RunID: r.RunID, Role: r.Role})
		}
	}
	// B1 设计锁启动重建（§1.8）：设计链在途票（设计中/已派/施工中）且有
	// 活 CTO run 者照相位恢复持锁——run 已死者随回队径自然不占（锁语义
	// ＝持锁票离开施工中/终态释放，见 transition）。
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

// deployStateOK 0034-p3 收口态判：末次 deploy cmd_run exit=0（已装）
// ＋socket 探活绿＝部署实成（restart 杀席是必然不证败）。
func (g *Gateway) deployStateOK(ctx context.Context, tid string) bool {
	runs, err := g.database.CmdRuns(ctx, tid)
	if err != nil || len(runs) == 0 {
		return false
	}
	var last *db.CmdRun
	for _, r := range runs {
		if r.Step == "deploy" {
			last = r
		}
	}
	if last == nil || last.ExitCode != 0 {
		return false
	}
	return upgrade.Probe(g.cfg.SocketPath, 2*time.Second) == nil
}

// spawnLine 无行动者相位→spawn_needed note 支线标（§14 半结构契约消费端＝
// 路由表 note_contains）：已立→CTO-A 设计支，待部署→CTO-B 部署支，
// 余相位＝角色本名（施工/审计）。
func spawnLine(p db.Phase) string {
	switch p {
	case db.PhaseCreated:
		return "CTO-A"
	case db.PhaseToDeploy:
		return "CTO-B"
	}
	return roleForPhase(p)
}

// roleForPhase 无行动者相位→下一步该起的角色。
func roleForPhase(p db.Phase) string {
	switch p {
	case db.PhaseAssigned:
		return "施工"
	case db.PhaseReview:
		return "审计"
	case db.PhaseToDeploy:
		return "CTO" // 部署归 CTO-B
	}
	return ""
}

// reconcileActor 有行动者相位：收养或回队。
func (g *Gateway) reconcileActor(ctx context.Context, t *db.Ticket) {
	run, _ := g.database.ActiveRunForTicket(ctx, t.ID)
	if run != nil && g.tryAdopt(ctx, run) {
		_ = g.database.MarkAdopted(ctx, run.RunID)
		st := newRunState(run)
		st.adopted = true
		g.mu.Lock()
		g.runs[run.RunID] = st
		g.mu.Unlock()
		g.rearmAlarm(ctx, t.ID) // 闹钟自 ticket_event 重算（A4）
		_ = g.database.AppendEvent(ctx, t.ID, "adopted", db.Event{
			Actor: "gateway", Note: "对账收养 " + run.RunID,
		})
		g.emit(Event{Kind: "adopted", TicketID: t.ID, RunID: run.RunID, Role: run.Role})
		return
	}
	if run != nil {
		_ = g.database.MarkDead(ctx, run.RunID)
		// D-55：标死连坐杀进程组——只改库行时错配席进程照跑烧 token
		// （req-f88 错派施工席标死后仍在跑的实证）。
		killRunGroup(run.PID)
	}
	var to db.Phase
	var kind db.EdgeKind
	switch t.Phase {
	case db.PhaseWorking:
		to, kind = db.PhaseAssigned, db.EdgeRequeue
		// D-51/54：施工中相位混三类行动者——CTO 设计席（req*/设计票）、
		// 施工席、巡检席（patrol-*）。行动者角色真源＝活 run 或最近
		// 非 adjunct run（相位推角色一律回已派补施工＝req-f88/巡检票
		// 双实证错配；取最近不取最早——票合法换角色后最早行是过期
		// 血统）。CTO→回 已立 补 CTO-A；巡检→回已派补巡检
		// （补发角色由 patrol- 前缀行给出）；余→施工。
		actorRole := ""
		if run != nil {
			actorRole = run.Role
		} else if er, err := g.database.LatestRunForTicket(ctx, t.ID); err == nil && er != nil {
			actorRole = er.Role
		}
		switch actorRole {
		case "CTO":
			to, kind = db.PhaseCreated, db.EdgeDesignDead
		case "巡检":
			// to=已派 保持；补发角色由 patrol- 前缀行覆写为 巡检。
		}
	case db.PhaseDesigning:
		to, kind = db.PhaseCreated, db.EdgeDesignDead
	case db.PhaseAuditing:
		to, kind = db.PhaseReview, db.EdgeAuditDead
	case db.PhaseDeploying:
		to, kind = db.PhaseToDeploy, db.EdgeDeployFailed
	}
	// 0034-p3 态判：deploy restart 灭席是必然——收口不看席死活看态：
	// 末次 deploy 步 exit=0（已装）＋现 socket 健康→deploy_ok（完成）；
	// 否则照旧 deploy_failed（回滚态未核）。
	if t.Phase == db.PhaseDeploying && g.deployStateOK(ctx, t.ID) {
		if err := g.transition(ctx, t.ID, db.PhaseDone, db.EdgeDeployDone,
			"gateway", "对账态判：已装+socket健康→deploy_ok"); err == nil {
			g.emit(Event{Kind: "deploy_ok", TicketID: t.ID})
			g.releaseProjectLocks(ctx, t.Project, t.ID, true,
				lockIntegrate, lockDeploy)
			return
		}
	}
	note := "对账：行动者无响应"
	if t.Phase == db.PhaseDeploying {
		// D-2：部署中对账死＝部署失败（回滚态未核）——note 留取证线索。
		logPath := ""
		if run != nil {
			logPath = " log=" + run.LogPath
		}
		note = fmt.Sprintf("对账：行动者无响应——先核回滚态 run=%s%s",
			runIDOf(run), logPath)
	}
	if err := g.transition(ctx, t.ID, to, kind, "gateway", note); err != nil {
		return
	}
	runID := ""
	if run != nil {
		runID = run.RunID
	}
	g.emit(Event{Kind: "requeue", TicketID: t.ID, RunID: runID, Note: "对账回队→" + string(to)})
	// 回队后新相位落入无行动者集，补 spawn 事件。deploy_failed 本票
	// 不补——回 待部署 归 CEO 裁（锁已由 transition 释放侧放掉）。
	if kind == db.EdgeDeployFailed {
		return
	}
	// 补发角色由回退后相位定（D-51 修后：设计席死回 已立→CTO，与主环
	// PhaseCreated 支同契约）；note 用 spawnLine 支线标——§14 半结构
	// 契约消费端＝路由表 note_contains（已立行须「CTO-A」非角色本名）。
	// patrol-* 值班票恒巡检（票0030 实证：phase 映射不含巡逻角色）。
	role := roleForPhase(to)
	if to == db.PhaseCreated {
		role = "CTO"
	}
	line := spawnLine(to)
	if strings.HasPrefix(t.ID, "patrol-") {
		role, line = "巡检", "巡检"
	}
	if role != "" {
		_ = g.database.AppendEvent(ctx, t.ID, "spawn_needed", db.Event{
			Actor: "gateway", Note: "对账补发: " + line,
		})
		g.emit(Event{Kind: "spawn_needed", TicketID: t.ID, Role: role})
	}
}

func runIDOf(r *db.Run) string {
	if r == nil {
		return ""
	}
	return r.RunID
}

// rearmAlarm deadline 取自 ticket_event 最近活性帧（§5.3 闹钟重算）：
// 已过期→clock.Alarm 立即响；未过期→设到原 deadline。防崩溃循环饿死卡死检测。
func (g *Gateway) rearmAlarm(ctx context.Context, ticketID string) {
	last, err := g.database.LastActivityAt(ctx, ticketID)
	if err != nil {
		last = time.Now()
	}
	g.armAlarm(ticketID, last.Add(g.cfg.ProgressTimeout))
}

// tryAdopt 收养互证（§5.1 D2）：连 per-seat UDS，首帧出示 run_token——
// 它发的它知道。单次有界尝试：连不上/拒收＝行动者判死，不重试不轮询。
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
