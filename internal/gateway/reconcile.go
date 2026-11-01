package gateway

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// Reconcile 启动对账（§5.3 全相位「相位×预期行动者」表，启动一次非周期）：
// 有行动者相位——活着收养、死了回队；无行动者相位——补发 spawn 事件。
func (g *Gateway) Reconcile(ctx context.Context) {
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
			// 半结构约定（防漂，§14）：note「对账补发: <role>」是播报员路由表
			// spawn_needed+note_contains 行的消费契约——改文案必须同步改表。
			role := roleForPhase(t.Phase)
			_ = g.database.AppendEvent(ctx, t.ID, "spawn_needed", db.Event{
				Actor: "gateway", Note: "对账补发: " + role,
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
				Actor: "gateway", Note: "对账补发: CTO",
			})
			g.emit(Event{Kind: "spawn_needed", TicketID: t.ID, Role: "CTO"})
		}
	}
	// B1 设计锁启动重建（§1.8）：设计中票的 project 持锁照相位恢复。
	for _, t := range tickets {
		if t.Phase == db.PhaseDesigning && t.Project != "" {
			g.designLocks[t.Project] = t.ID
		}
	}
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
	}
	var to db.Phase
	var kind db.EdgeKind
	switch t.Phase {
	case db.PhaseWorking:
		to, kind = db.PhaseAssigned, db.EdgeRequeue
	case db.PhaseDesigning:
		to, kind = db.PhaseCreated, db.EdgeDesignDead
	case db.PhaseAuditing:
		to, kind = db.PhaseReview, db.EdgeAuditDead
	case db.PhaseDeploying:
		to, kind = db.PhaseToDeploy, db.EdgeDeployFailed
	}
	if err := g.transition(ctx, t.ID, to, kind, "gateway", "对账：行动者无响应"); err != nil {
		return
	}
	runID := ""
	if run != nil {
		runID = run.RunID
	}
	g.emit(Event{Kind: "requeue", TicketID: t.ID, RunID: runID, Note: "对账回队→" + string(to)})
	// 回队后新相位落入无行动者集，补 spawn 事件。
	role := roleForPhase(to)
	if role != "" {
		_ = g.database.AppendEvent(ctx, t.ID, "spawn_needed", db.Event{
			Actor: "gateway", Note: "对账补发: " + role,
		})
		g.emit(Event{Kind: "spawn_needed", TicketID: t.ID, Role: role})
	}
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
