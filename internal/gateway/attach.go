package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meta/internal/db"
	"meta/internal/seat"
	pbv1 "meta/internal/pb/metav1"
)

// Attach 席位面双向流（§12）。首帧必为 RunAuth——验不过断流（§3.4 A7）。
func (g *Gateway) Attach(stream pbv1.SeatGateway_AttachServer) error {
	ctx := stream.Context()

	first, err := recvFirst(stream, g.cfg.AuthTimeout)
	if err != nil {
		return err
	}
	auth := first.GetAuth()
	if auth == nil || auth.RunToken == "" {
		return status.Error(codes.Unauthenticated, "首帧须为 RunAuth")
	}
	run, err := g.database.RunByToken(ctx, auth.RunToken)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if run == nil || run.Dead {
		return status.Error(codes.Unauthenticated, "run_token 无效")
	}

	st := g.bindRun(run)
	st.mu.Lock()
	if !st.attached {
		st.attached = true
		close(st.attachedCh)
	}
	st.mu.Unlock()
	_ = g.database.MarkAttached(ctx, run.RunID)
	g.emit(Event{Kind: "attach", TicketID: run.TicketID, RunID: run.RunID, Role: run.Role})

	for {
		msg, err := stream.Recv()
		if err != nil {
			g.onStreamClosed(st)
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if err := g.handleSeatMsg(ctx, st, stream, msg); err != nil {
			g.onStreamClosed(st)
			return err
		}
	}
}

// recvFirst 等首帧，超时即拒——一次性闹钟（time.AfterFunc 等价物），非周期。
func recvFirst(stream pbv1.SeatGateway_AttachServer, timeout time.Duration) (*pbv1.SeatMsg, error) {
	type res struct {
		m *pbv1.SeatMsg
		e error
	}
	ch := make(chan res, 1)
	go func() {
		m, e := stream.Recv()
		ch <- res{m, e}
	}()
	fired := make(chan struct{})
	t := time.AfterFunc(timeout, func() { close(fired) })
	defer t.Stop()
	select {
	case r := <-ch:
		if r.e != nil {
			return nil, status.Error(codes.Unauthenticated, "首帧未至")
		}
		return r.m, nil
	case <-fired:
		return nil, status.Error(codes.Unauthenticated, "首帧超时")
	case <-stream.Context().Done():
		return nil, stream.Context().Err()
	}
}

func (g *Gateway) bindRun(run *db.Run) *runState {
	g.mu.Lock()
	defer g.mu.Unlock()
	st, ok := g.runs[run.RunID]
	if !ok {
		st = newRunState(run)
		g.runs[run.RunID] = st
	}
	return st
}

func (g *Gateway) handleSeatMsg(ctx context.Context, st *runState, stream pbv1.SeatGateway_AttachServer, msg *pbv1.SeatMsg) error {
	switch b := msg.GetBody().(type) {
	case *pbv1.SeatMsg_Claim:
		return g.handleClaim(ctx, st, b.Claim)
	case *pbv1.SeatMsg_Progress:
		return g.handleProgress(ctx, st, b.Progress)
	case *pbv1.SeatMsg_Usage:
		return g.handleUsage(ctx, st, b.Usage)
	case *pbv1.SeatMsg_Deliver:
		return g.handleDeliver(ctx, st, stream, b.Deliver)
	case *pbv1.SeatMsg_Yield:
		return g.handleYield(ctx, st, b.Yield)
	case *pbv1.SeatMsg_PermissionReply:
		return g.database.AppendEvent(ctx, st.run.TicketID, "permission_reply", db.Event{
			Actor: st.run.RunID, Note: fmt.Sprintf("ask=%s allow=%v", b.PermissionReply.AskId, b.PermissionReply.Allow),
		})
	case *pbv1.SeatMsg_SecretReq:
		return g.handleSecret(ctx, st, stream, b.SecretReq)
	case *pbv1.SeatMsg_Rule:
		return g.handleRule(ctx, st, b.Rule)
	case *pbv1.SeatMsg_Auth:
		return status.Error(codes.InvalidArgument, "重复 RunAuth")
	default:
		return status.Error(codes.InvalidArgument, "空消息")
	}
}

// handleClaim 先连后领（§5.1）：已派→施工中 只发生在连接内。
// 三分支（§15.5 码位表）：adjunct 席仅记 adjunct_claim 行不转相位（早退——
// 对抗性锚 b：删本支裁决席把票推离施工中）但重挂票闹钟；审计席走
// audit_start→审计中（幂等相位＝审计中，收养重领放行）；其余照旧 claim→施工中。
func (g *Gateway) handleClaim(ctx context.Context, st *runState, c *pbv1.Claim) error {
	if c.TicketId != st.run.TicketID {
		return status.Errorf(codes.PermissionDenied, "票 %s 非本 run 所派", c.TicketId)
	}
	if st.run.Adjunct {
		// 对抗性锚提名行：adjunct claim 的「不转相位」早退分支。
		_ = g.database.AppendEvent(ctx, st.run.TicketID, "adjunct_claim", db.Event{
			Actor: st.run.RunID, Note: "role=" + st.run.Role})
		g.armAlarm(st.run.TicketID, time.Now().Add(g.cfg.ProgressTimeout))
		g.emit(Event{Kind: "adjunct_claim", TicketID: st.run.TicketID, RunID: st.run.RunID,
			Role: st.run.Role})
		return nil
	}
	phase, err := g.database.PhaseOf(ctx, st.run.TicketID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	// 对抗性锚提名行：按 run.Role 选边——审计→audit_start（翻成统一
	// EdgeClaim 则审计领票撞 待审→施工中 非法边）。
	to, kind, idem := db.PhaseWorking, db.EdgeClaim, db.PhaseWorking
	if st.run.Role == "审计" {
		to, kind, idem = db.PhaseAuditing, db.EdgeAuditStart, db.PhaseAuditing
	}
	if phase == idem {
		return nil // 收养后重领：幂等
	}
	if err := g.transition(ctx, st.run.TicketID, to, kind, st.run.RunID, ""); err != nil {
		var ille *db.ErrIllegalEdge
		if errors.As(err, &ille) {
			return status.Error(codes.FailedPrecondition, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
	g.armAlarm(st.run.TicketID, time.Now().Add(g.cfg.ProgressTimeout))
	return nil
}

func (g *Gateway) handleProgress(ctx context.Context, st *runState, p *pbv1.Progress) error {
	if err := g.database.AppendEvent(ctx, st.run.TicketID, "progress", db.Event{
		Actor: st.run.RunID, Note: p.Stage + " " + p.Note,
	}); err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	g.armAlarm(st.run.TicketID, time.Now().Add(g.cfg.ProgressTimeout)) // 随帧重置
	return nil
}

// handleUsage 用量帧：累计＋闹钟重置（用量帧亦活性帧）＋预算三闸
// （80% 警→CEO / 100% 硬停 / 票级硬顶→先杀后冻，§5.5）。
func (g *Gateway) handleUsage(ctx context.Context, st *runState, u *pbv1.Usage) error {
	tid := st.run.TicketID
	// adjunct 席只计票级 used_total 不进尝试内 used（§1.3：不占尝试帽）；
	// 审计席是相位步行席——used/used_total 双计（§15.5，真尝试吃审计帽）。
	if err := g.database.AddUsage(ctx, tid, u.TokensUsed, !st.run.Adjunct); err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	_ = g.database.AppendEvent(ctx, tid, "usage", db.Event{
		Actor: st.run.RunID, Note: fmt.Sprintf("+%d tokens", u.TokensUsed)})
	g.armAlarm(tid, time.Now().Add(g.cfg.ProgressTimeout))

	b, err := g.database.GetBudget(ctx, tid)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	switch {
	case b.HardCap > 0 && b.UsedTotal >= b.HardCap: // 对抗性锚提名行：票级硬顶比较
		g.freezeTicket(ctx, tid, st)
	case b.Cap > 0 && b.Used >= b.Cap: // 对抗性锚提名行：尝试内 100% 比较
		g.budgetStop(ctx, st)
	case b.Cap > 0 && b.Used*5 >= b.Cap*4: // 80% 警（整数比较避浮点）
		g.mu.Lock()
		if !g.warned[tid] {
			g.warned[tid] = true
			g.mu.Unlock()
			g.emit(Event{Kind: "budget_warn", TicketID: tid, RunID: st.run.RunID,
				Note: fmt.Sprintf("尝试内用量 %d/%d", b.Used, b.Cap)})
			_ = g.database.AppendEvent(ctx, tid, "budget_warn", db.Event{
				Actor: "gateway", Note: fmt.Sprintf("used=%d cap=%d", b.Used, b.Cap)})
		} else {
			g.mu.Unlock()
		}
	}
	return nil
}

// budgetStop 尝试内 100% 硬停：杀本尝试＋事件（③级归 CTO 裁——
// 票回队不自动重派，摆位等裁决）。先记 killByGW 再杀：非壳之错不罚健康。
func (g *Gateway) budgetStop(ctx context.Context, st *runState) {
	st.killByGW = true
	st.noRespawn = true
	g.emit(Event{Kind: "budget_stop", TicketID: st.run.TicketID, RunID: st.run.RunID})
	_ = g.database.AppendEvent(ctx, st.run.TicketID, "budget_stop", db.Event{
		Actor: "gateway", Note: "尝试内用量达 100% 硬顶"})
	g.killRun(st)
}

// freezeTicket 票级硬顶：先杀后冻（§5.5 D8 钦定序）——席位仍活先按 100%
// 硬停杀本尝试，再置 budget_frozen；同时取消该票未决闹钟与排队事件。
// 相位不动；追加预算复活＝抬帽不清零（SetBudgetCaps 同径解冻）。
func (g *Gateway) freezeTicket(ctx context.Context, ticketID string, st *runState) {
	// 先杀（若调用方未带 run 柄，取票上活 run）。
	if st == nil {
		if run, _ := g.database.ActiveRunForTicket(ctx, ticketID); run != nil {
			st = g.bindRun(run)
		}
	}
	if st != nil {
		st.killByGW = true
		g.killRun(st)
	}
	// 后冻。
	if err := g.database.SetBudgetFrozen(ctx, ticketID, true); err != nil {
		return
	}
	g.mu.Lock()
	g.frozen[ticketID] = true
	g.mu.Unlock()
	g.cancelAlarm(ticketID) // 取消未决闹钟；排队 spawn 事件由 dispatcher/emit 判弃
	g.emit(Event{Kind: "budget_frozen", TicketID: ticketID})
	_ = g.database.AppendEvent(ctx, ticketID, "budget_frozen", db.Event{
		Actor: "gateway", Note: "票级累计硬顶到达：先杀后冻"})
}

// handleDeliver 交付/判决（§15.5 码位表）：
//   - adjunct 席：仅记 adjunct_deliver 行不转相位，Ack 照回；
//   - 审计席：verdict 必填——pass→audit_pass→待部署；reject→EdgeReject→已派
//     （reject_count 由 Transition 既有行累加；随后网关内 enqueueSpawn 重派
//     施工，spec 键含 role 原 prefer 保）；
//   - 其余角色：verdict 必须空（语义混装即拒）→deliver→待审。
func (g *Gateway) handleDeliver(ctx context.Context, st *runState, stream pbv1.SeatGateway_AttachServer, d *pbv1.Deliver) error {
	ack := func(note string) error {
		return stream.Send(&pbv1.GatewayMsg{Body: &pbv1.GatewayMsg_Ack{
			Ack: &pbv1.Ack{Ok: true, Note: note}}})
	}
	if st.run.Role != "审计" && d.Verdict != "" {
		return status.Error(codes.InvalidArgument, "verdict 仅审计席可携——语义混装")
	}
	if st.run.Adjunct {
		_ = g.database.AppendEvent(ctx, st.run.TicketID, "adjunct_deliver", db.Event{
			Actor: st.run.RunID, Note: d.Ref + " " + d.Note})
		g.emit(Event{Kind: "adjunct_deliver", TicketID: st.run.TicketID, RunID: st.run.RunID,
			Role: st.run.Role})
		return ack("adjunct_delivered")
	}
	if st.run.Role == "审计" {
		// 对抗性锚提名行：reject→EdgeReject 映射（翻成 audit_pass 则驳回票
		// 落待部署放行）。
		var to db.Phase
		var kind db.EdgeKind
		switch d.Verdict {
		case "pass":
			to, kind = db.PhaseToDeploy, db.EdgeAuditPass
		case "reject":
			to, kind = db.PhaseAssigned, db.EdgeReject
		default:
			// 判不出判决的审计席＝死席（§15.2）：不落相位不回 Ack——
			// 席死后走 audit_dead 自纠正径重起审计。
			return status.Error(codes.InvalidArgument, "审计 verdict 必填 pass|reject")
		}
		note := d.Verdict + " " + d.Ref + " " + d.Note
		if err := g.transition(ctx, st.run.TicketID, to, kind, st.run.RunID, note); err != nil {
			var ille *db.ErrIllegalEdge
			if errors.As(err, &ille) {
				return status.Error(codes.FailedPrecondition, err.Error())
			}
			return status.Error(codes.Internal, err.Error())
		}
		g.cancelAlarm(st.run.TicketID)
		if d.Verdict == "reject" {
			// 驳回重派施工径①（§15.2）：spec 仍在（键含 role）原 prefer 保；
			// 径②路由表 reject→dispatch 施工在对账/重启失 spec 后补位。
			g.enqueueSpawn(st.run.TicketID, "施工")
		}
		return ack("verdict " + d.Verdict)
	}
	if err := g.transition(ctx, st.run.TicketID, db.PhaseReview, db.EdgeDeliver,
		st.run.RunID, d.Ref+" "+d.Note); err != nil {
		var ille *db.ErrIllegalEdge
		if errors.As(err, &ille) {
			return status.Error(codes.FailedPrecondition, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
	g.cancelAlarm(st.run.TicketID)
	// Deliver 回执：席位等此帧再退——否则尾帧可能丢在 gRPC 传输缓冲里。
	return ack("delivered")
}

// handleYield 补边：施工中→已派，理由入判决史并触发 CTO 裁决事件（§6.2）。
// adjunct 席仅记 adjunct_yield 行不转相位（§1.3）。
func (g *Gateway) handleYield(ctx context.Context, st *runState, y *pbv1.Yield) error {
	note := y.Kind + ": " + y.Reason
	if st.run.Adjunct {
		_ = g.database.AppendEvent(ctx, st.run.TicketID, "adjunct_yield", db.Event{
			Actor: st.run.RunID, Note: note})
		g.emit(Event{Kind: "adjunct_yield", TicketID: st.run.TicketID, RunID: st.run.RunID,
			Role: st.run.Role})
		return nil
	}
	if err := g.transition(ctx, st.run.TicketID, db.PhaseAssigned, db.EdgeYield,
		st.run.RunID, note); err != nil {
		var ille *db.ErrIllegalEdge
		if errors.As(err, &ille) {
			return status.Error(codes.FailedPrecondition, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
	g.cancelAlarm(st.run.TicketID)
	g.emit(Event{Kind: "yield", TicketID: st.run.TicketID, RunID: st.run.RunID, Note: note})
	return nil
}

// handleSecret Secret(name) 读径（凭证清单）：按 run 角色对 role_acl 放行。
func (g *Gateway) handleSecret(ctx context.Context, st *runState, stream pbv1.SeatGateway_AttachServer, req *pbv1.SecretReq) error {
	resp := &pbv1.GatewayMsg{Body: &pbv1.GatewayMsg_SecretResp{
		SecretResp: &pbv1.SecretResp{Name: req.Name, Denied: true},
	}}
	if g.vaultKey != nil {
		if ct, ok, err := g.database.GetSecret(ctx, req.Name, st.run.Role); err == nil && ok {
			if plain, err := db.DecryptSecret(g.vaultKey, ct); err == nil {
				resp = &pbv1.GatewayMsg{Body: &pbv1.GatewayMsg_SecretResp{
					SecretResp: &pbv1.SecretResp{Name: req.Name, Value: plain},
				}}
			}
		}
	}
	if err := stream.Send(resp); err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

// onStreamClosed socket EOF 源：与 SIGCHLD 互证后才判死（§5.2）。
func (g *Gateway) onStreamClosed(st *runState) {
	st.mu.Lock()
	if !st.streamClosed {
		st.streamClosed = true
		_ = g.database.MarkStreamClosed(context.Background(), st.run.RunID)
	}
	st.mu.Unlock()
	g.maybeDeclareDead(st)
}

// maybeDeclareDead 双源互证：进程退出（SIGCHLD→Wait）＋ 流 EOF 齐到才判死；
// 从未 attach 的 run 进程死了即死（无第二源可等）。收养 run 无 Wait 柄，
// EOF 时按 kill(pid,0) 一次性核实（机械判，非周期探活）。
func (g *Gateway) maybeDeclareDead(st *runState) {
	st.mu.Lock()
	exited := st.exited
	if !exited && st.adopted && st.streamClosed {
		exited = !processAlive(st.run.PID)
	}
	dead := exited && (st.streamClosed || !st.attached)
	if dead && !st.dead {
		st.dead = true
	}
	st.mu.Unlock()
	if dead {
		g.onRunDead(st)
	}
}

// onRunDead 判死落库＋分相位处置（§15.5 码位表）：
//   - adjunct 席：仅发 adjunct_dead——不回队、不罚壳健康、不占 failSet；
//   - 施工中死：票回队（→已派）＋failSet 分级＋自动重派（换壳 redo）；
//   - 审计中死：audit_dead→待审不计驳回＋failSet 分级照旧＋重派审计；
//   - 其他相位只出 seat_dead 事件交启动对账/上层处置——网关不做语义裁决。
func (g *Gateway) onRunDead(st *runState) {
	ctx := context.Background()
	_ = g.database.MarkDead(ctx, st.run.RunID)

	if st.attempt.Shell != "" {
		g.mu.Lock()
		if g.active[st.attempt.Shell] > 0 {
			g.active[st.attempt.Shell]--
		}
		g.mu.Unlock()
	}
	if st.run.Adjunct {
		_ = g.database.AppendEvent(ctx, st.run.TicketID, "adjunct_dead", db.Event{
			Actor: "gateway", Note: "role=" + st.run.Role})
		g.emit(Event{Kind: "adjunct_dead", TicketID: st.run.TicketID, RunID: st.run.RunID,
			Role: st.run.Role})
		return
	}
	g.emit(Event{Kind: "seat_dead", TicketID: st.run.TicketID, RunID: st.run.RunID, Role: st.run.Role})

	if st.attempt.Shell != "" && !st.killByGW {
		// 中途死＝①机器级（含外部 SIGKILL）：不标健康，进本票 failSet 换下一壳。
		g.reportShell(ctx, st.run.TicketID, st.run.Role, st.attempt, seat.ClassOfExit(st.exitCode))
	}

	phase, err := g.database.PhaseOf(ctx, st.run.TicketID)
	if err != nil {
		return
	}
	var to db.Phase
	var kind db.EdgeKind
	switch phase {
	case db.PhaseWorking:
		to, kind = db.PhaseAssigned, db.EdgeRequeue
	case db.PhaseAuditing:
		to, kind = db.PhaseReview, db.EdgeAuditDead
	default:
		return
	}
	if err := g.transition(ctx, st.run.TicketID, to, kind,
		"gateway", "席位死亡：SIGCHLD+EOF 双源互证"); err != nil {
		return
	}
	g.cancelAlarm(st.run.TicketID)
	g.emit(Event{Kind: "requeue", TicketID: st.run.TicketID, RunID: st.run.RunID,
		Role: st.run.Role})
	// 回队即重派（换壳 redo；审计席死→重起审计——spec 在则秒接，失则
	// 路由表 audit_dead→dispatch 审计补位）；预算硬停摆位与冻票不重派。
	g.mu.Lock()
	frozen := g.frozen[st.run.TicketID]
	g.mu.Unlock()
	if !st.noRespawn && !frozen {
		g.enqueueSpawn(st.run.TicketID, st.run.Role)
	}
}
