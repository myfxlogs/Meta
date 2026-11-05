package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
	"meta/internal/seat"
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
	newAttach := !st.attached
	if newAttach {
		st.attached = true
		close(st.attachedCh)
	}
	st.mu.Unlock()
	// T17①：attach 即挂停滞钟——挂起席（进程活零输出）必报，
	// 不等 claim 才起算；收养生不重挂（st.adopted 径——重领不
	// 算活性防刷钟）。
	if newAttach && !st.adopted && st.run.TicketID != "" {
		g.armAlarm(st.run.TicketID, time.Now().Add(g.cfg.ProgressTimeout))
	}
	// 票0030：存活流句柄存 runState——PostMsg 的 feed 直投径；发送经
	// sendToSeat（sendMu 互斥，与本流各回执 Send 并发安全）。
	st.sendMu.Lock()
	st.stream = stream
	st.sendMu.Unlock()
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
	case *pbv1.SeatMsg_ExecReq:
		return g.handleExec(ctx, st, stream, b.ExecReq)
	case *pbv1.SeatMsg_Admin:
		return g.handleAdmin(ctx, st, stream, b.Admin)
	case *pbv1.SeatMsg_Session:
		// 票0030 续传：壳会话 id 落 seat_run.session_id（续会锚）。
		return g.database.SetRunSession(ctx, st.run.RunID, b.Session.SessionId)
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
	if st.run.TicketID == "" {
		return status.Error(codes.InvalidArgument, "无票 run（CEO 面）不可领票")
	}
	if c.TicketId != st.run.TicketID {
		return status.Errorf(codes.PermissionDenied, "票 %s 非本 run 所派", c.TicketId)
	}
	if st.run.Adjunct {
		// 对抗性锚提名行：adjunct claim 的「不转相位」早退分支。
		_ = g.database.AppendEvent(ctx, st.run.TicketID, "adjunct_claim", db.Event{
			Actor: st.run.RunID, Note: "role=" + st.run.Role})
		// T17②：收养席重领不算活性——adopted 连发不得复位停滞钟。
		if !st.adopted {
			g.armAlarm(st.run.TicketID, time.Now().Add(g.cfg.ProgressTimeout))
		}
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
	// CTO-B 部署支领票：待部署→deploy_start→部署中（票0010——相位步行席）。
	if st.run.Role == "CTO" && phase == db.PhaseToDeploy {
		to, kind, idem = db.PhaseDeploying, db.EdgeDeployStart, db.PhaseDeploying
	}
	// T35 调和器重派径：部署中/设计中席重领＝幂等（相位稳守）。
	if st.run.Role == "CTO" && (phase == db.PhaseDeploying || phase == db.PhaseDesigning) {
		idem = phase
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
	// 板变化帧源（§9：席位板「正在做什么」列）——落行后 emit，订户重读 DB 即现态。
	g.emit(Event{Kind: "progress", TicketID: st.run.TicketID, RunID: st.run.RunID,
		Role: st.run.Role, Note: p.Stage + " " + p.Note})
	g.armAlarm(st.run.TicketID, time.Now().Add(g.cfg.ProgressTimeout)) // 随帧重置
	return nil
}

// handleUsage 用量帧：累计＋闹钟重置（用量帧亦活性帧）＋预算三闸
// （80% 警→CEO / 100% 硬停 / 票级硬顶→先杀后冻，§5.5）。
func (g *Gateway) handleUsage(ctx context.Context, st *runState, u *pbv1.Usage) error {
	st.mu.Lock()
	dead := st.dead
	st.mu.Unlock()
	if !dead {
		if r, err := g.database.RunByToken(ctx, st.run.RunToken); err == nil && r != nil && r.Dead {
			dead = true // 内存标滞后于 DB 账——双源核死
		}
	}
	if dead {
		return nil // 死席帧不收——濒死时序旧账不得染指新尝试（R1 病灶防径）
	}
	tid := st.run.TicketID
	// adjunct 席只计票级 used_total 不进尝试内 used（§1.3：不占尝试帽）；
	// 审计席是相位步行席——used/used_total 双计（§15.5，真尝试吃审计帽）。
	if err := g.database.AddUsage(ctx, tid, u.TokensUsed, !st.run.Adjunct); err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	_ = g.database.AppendEvent(ctx, tid, "usage", db.Event{
		Actor: st.run.RunID, Note: fmt.Sprintf("+%d tokens", u.TokensUsed)})
	// 板变化帧源（预算用量列随帧刷新）。
	g.emit(Event{Kind: "usage", TicketID: tid, RunID: st.run.RunID, Role: st.run.Role})
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
		// T35：warned 内存表退役——本尝试期窗（最近 claim 边后）无
		// budget_warn 行才发，事件账即边沿判据，重启零丢失。
		if g.budgetWarned(ctx, tid) {
			break
		}
		g.emit(Event{Kind: "budget_warn", TicketID: tid, RunID: st.run.RunID,
			Note: fmt.Sprintf("尝试内用量 %d/%d", b.Used, b.Cap)})
		_ = g.database.AppendEvent(ctx, tid, "budget_warn", db.Event{
			Actor: "gateway", Note: fmt.Sprintf("used=%d cap=%d", b.Used, b.Cap)})
	}
	return nil
}

// budgetStop 尝试内 100% 硬停：杀本尝试＋事件（③级归 CTO 裁——
// 票回队不自动重派，摆位等裁决）。先记 killByGW 再杀：非壳之错不罚健康。
func (g *Gateway) budgetStop(ctx context.Context, st *runState) {
	st.killByGW = true
	st.noRespawn = true
	// D-34：先落行后 emit（对齐其余七处——emit 是门铃不是真源）。
	_ = g.database.AppendEvent(ctx, st.run.TicketID, "budget_stop", db.Event{
		Actor: "gateway", Note: "尝试内用量达 100% 硬顶"})
	g.emit(Event{Kind: "budget_stop", TicketID: st.run.TicketID, RunID: st.run.RunID})
	g.killRun(st)
	// T35：尝试内硬顶＝安全停人工态（P3：不是冻是 blocked——待人放行/作废）。
	g.blockTicket(ctx, st.run.TicketID, "尝试内用量达 100% 硬顶")
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
		st.killCause = "freeze_or_requeue"
		g.killRun(st)
	}
	// 后冻。
	if err := g.database.SetBudgetFrozen(ctx, ticketID, true); err != nil {
		return
	}
	g.cancelAlarm(ticketID)
	// D-34：先落行后 emit。
	_ = g.database.AppendEvent(ctx, ticketID, "budget_frozen", db.Event{
		Actor: "gateway", Note: "票级累计硬顶到达：先杀后冻"})
	g.emit(Event{Kind: "budget_frozen", TicketID: ticketID})
	// T35 失败表：预算到顶→冻→blocked 人工态。
	g.blockTicket(ctx, ticketID, "票级预算硬顶——放行/重试/作废待人裁")
}

// handleDeliver 交付/判决（§15.5 码位表）：
//   - adjunct 席：仅记 adjunct_deliver 行不转相位，Ack 照回；
//   - 审计席：verdict 必填——pass→audit_pass→待部署；reject→EdgeReject→已派
//     （reject_count 由 Transition 既有行累加；随后网关内 enqueueSpawn 重派
//     施工，spec 键含 role 原 prefer 保）；
//   - 其余角色：verdict 必须空（语义混装即拒）→deliver→待审。
func (g *Gateway) handleDeliver(ctx context.Context, st *runState, stream pbv1.SeatGateway_AttachServer, d *pbv1.Deliver) error {
	ack := func(note string) error {
		return g.sendToSeat(st, &pbv1.GatewayMsg{Body: &pbv1.GatewayMsg_Ack{
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
		// T35：reject 后不就地重派——票回 已派，调和器下轮按失败表
		// same_shell（≤3→blocked）起席。
		return ack("verdict " + d.Verdict)
	}
	// CTO-B 部署支交付（票0010）：note 哨兵驱动边选择——对抗性锚 a 提名区：
	// META_DEPLOY:done→deploy_done→完成；META_DEPLOY:failed→deploy_failed→
	// 待部署（翻映射＝失败票落完成）。缺哨兵即拒，不猜。
	if st.run.Role == "CTO" {
		phase, err := g.database.PhaseOf(ctx, st.run.TicketID)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if phase == db.PhaseDeploying {
			var to db.Phase
			var kind db.EdgeKind
			switch {
			case strings.Contains(d.Note, "META_DEPLOY:done"):
				to, kind = db.PhaseDone, db.EdgeDeployDone
			case strings.Contains(d.Note, "META_DEPLOY:failed"):
				// T35：部署哨兵负＝四步回滚已跑→blocked 人工态（非回待部署）
				to, kind = db.PhaseBlocked, db.EdgeBlocked
			default:
				return status.Error(codes.InvalidArgument,
					"部署席 Deliver 缺 META_DEPLOY:done|failed 哨兵")
			}
			if err := g.transition(ctx, st.run.TicketID, to, kind,
				st.run.RunID, d.Ref+" "+d.Note); err != nil {
				var ille *db.ErrIllegalEdge
				if errors.As(err, &ille) {
					return status.Error(codes.FailedPrecondition, err.Error())
				}
				return status.Error(codes.Internal, err.Error())
			}
			g.cancelAlarm(st.run.TicketID)
			return ack("deploy " + string(kind))
		}
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
	// CTO-A Yield＝需求待裁（§1.4）：泊车边 施工中→已立（req_pending 行＝泊车标，
	// 调和器抑制补派）；其余角色 v2 钦定 Yield＝blocked 人工态。
	to, kind := db.PhaseBlocked, db.EdgeBlocked
	if st.run.Role == "CTO" {
		to, kind = db.PhaseCreated, db.EdgeReqPending
	}
	if err := g.transition(ctx, st.run.TicketID, to, kind,
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
				// T7 scoped 凭据注入：放行即落 run/secrets/<runid>/<name>
				// （目录0700 文件0600），席位经 HOME/seat-secrets 只读挂载
				// 径读——值仍随流回执（兼容既有消费面）。
				secDir := filepath.Join(g.cfg.RunDir, "secrets", st.run.RunID)
				// T18①真只读：目录临时松 0700 写文件 0444→回落 0555——
				// 同 uid 席 open(O_WRONLY)/新建文件俱 EACCES（chmod 回改
				// 是二步升级，符号链近似面收敛为权限位闸）。
				_ = os.MkdirAll(secDir, 0o700)
				_ = os.Chmod(secDir, 0o700)
				werr := os.WriteFile(filepath.Join(secDir, req.Name), plain, 0o444)
				_ = os.Chmod(secDir, 0o555)
				if werr != nil {
					_ = g.database.AppendEvent(ctx, st.run.TicketID, "secret_write_failed", db.Event{
						Actor: "gateway", Note: req.Name + ": " + werr.Error()})
				}
				resp = &pbv1.GatewayMsg{Body: &pbv1.GatewayMsg_SecretResp{
					SecretResp: &pbv1.SecretResp{Name: req.Name, Value: plain},
				}}
			}
		}
	}
	if resp.GetSecretResp().GetDenied() {
		// T7 ACL fail-closed 留痕：越权读必拒＋denied 事件（不静默）。
		_ = g.database.AppendEvent(ctx, st.run.TicketID, "secret_denied", db.Event{
			Actor: st.run.RunID, Note: "name=" + req.Name + " role=" + st.run.Role})
		g.emit(Event{Kind: "secret_denied", TicketID: st.run.TicketID,
			RunID: st.run.RunID, Role: st.run.Role, Note: req.Name})
	}
	if err := g.sendToSeat(st, resp); err != nil {
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
	st.sendMu.Lock()
	st.stream = nil // 票0030：流死除名——feed 直投不再往死流写
	st.sendMu.Unlock()
	g.maybeDeclareDead(st)
}

// sendToSeat 座流互斥发送（票0030：feed 直投与各回执 Send 并发安全）。
func (g *Gateway) sendToSeat(st *runState, m *pbv1.GatewayMsg) error {
	st.sendMu.Lock()
	defer st.sendMu.Unlock()
	if st.stream == nil {
		return status.Error(codes.Unavailable, "席流未立")
	}
	return st.stream.Send(m)
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
	// T7/T18 死席凭据目录回收：run/secrets/<runid> 全删——凭据作用域
	// =run 生命周期；wipeSecretsDir 先回权限位再删（T18 只读面）。
	g.wipeSecretsDir(st.run.RunID)
	g.cleanSeatResidue(st.run) // T27④：插座+工作区随收尸连清（日志留档）

	if st.attempt.Shell != "" {
		g.mu.Lock()
		if g.active[st.attempt.Shell] > 0 {
			g.active[st.attempt.Shell]--
		}
		g.mu.Unlock()
	}
	if st.run.Adjunct {
		// T24-B 语义闸：短窗 exit=0＝空勤退场（无事可报正常退）——
		// 记 patrol_skip 静默账，不记 adjunct_dead 不通知（19:23-
		// 19:38 480 岗/746 死实证：干净退场被记死+通知成风暴源）。
		if st.exitCode == 0 && time.Since(st.run.StartedAt) < 90*time.Second {
			_ = g.database.AppendEvent(ctx, st.run.TicketID, "patrol_skip", db.Event{
				Actor: "gateway", Note: "空勤退场（短窗 exit=0）role=" + st.run.Role})
			return
		}
		_ = g.database.AppendEvent(ctx, st.run.TicketID, "adjunct_dead", db.Event{
			Actor: "gateway", Note: "role=" + st.run.Role})
		g.emit(Event{Kind: "adjunct_dead", TicketID: st.run.TicketID, RunID: st.run.RunID,
			Role: st.run.Role})
		return
	}
	_ = g.database.AppendEvent(ctx, st.run.TicketID, "seat_dead", db.Event{
		Actor: "gateway", Note: "席死 " + st.run.RunID + " role=" + st.run.Role + " cause=" + deadCauseOf(st)})
	g.emit(Event{Kind: "seat_dead", TicketID: st.run.TicketID, RunID: st.run.RunID, Role: st.run.Role})

	if st.run.Role == "CEO" {
		// D-22 收口比对（判死点）：派发给它的事件数 vs ceo_memory 写数。
		g.ceoDeadHook(ctx, st)
		return
	}

	if st.attempt.Shell != "" && !st.killByGW {
		// 中途死：壳级/机器级分级记 seat_err 账（席位级死因 reportShell
		// 内早退——只落 seat_err 不记壳账）。
		g.reportShell(ctx, st.run.TicketID, st.run.Role, st.attempt,
			seat.ClassOfExit(st.exitCode), st)
	}
	// T35：席死不就地转相位/重派/补发——相位保持（行动者缺），调和器
	// 下轮读 seat_run 死账＋失败表决定起席还是 blocked。唯一例外：
	// 待部署 CTO 席死未 claim——部署双锁在派发点取走，此处放还防永占。
	if st.run.Role == "CTO" && !st.run.Adjunct {
		if tk, err := g.database.GetTicket(ctx, st.run.TicketID); err == nil && tk != nil &&
			tk.Phase == db.PhaseToDeploy {
			g.releaseProjectLocks(ctx, tk.Project, st.run.TicketID, true,
				lockIntegrate, lockDeploy)
		}
	}
	// 部署中席死先排干哨兵（done→deploy_done 收口；failed→失败表径，
	// cli-drive 相位稳守——调和器按 clidrive_dead 账重派继任）。
	if tk, err := g.database.GetTicket(ctx, st.run.TicketID); err == nil && tk != nil &&
		tk.Phase == db.PhaseDeploying {
		switch g.drainDeploySentinel(st.run.RunID) {
		case "done":
			if g.deployDone(ctx, st.run.TicketID, "席死排干：META_DEPLOY:done 在盘") {
				return
			}
		case "failed":
			// 终哨兵负＝四步径回滚已执行→记 deploy_failed 账＋blocked 人工态。
			// T49：期窗去重——与 reconcile 径同窗双计是 cap 凑限 blocked
			// 的活锁根（0051 实证同秒双账），已有账则不再追加/再拦截。
			note := fmt.Sprintf("部署中席死：哨兵在盘 META_DEPLOY:failed——回滚已在四步径执行 run=%s",
				st.run.RunID)
			deployFailMu.Lock()
			had := g.epochHasKind(ctx, st.run.TicketID, "deploy_failed")
			deployFailMu.Unlock()
			if had {
				return
			}
			g.appendDeployFailedOnce(ctx, st.run.TicketID, note)
			g.blockTicket(ctx, st.run.TicketID, note)
			return
		default:
			if g.deployStepsDone(ctx, st.run.TicketID) && g.deployDone(ctx,
				st.run.TicketID, "席死径账判：四步 cmd_run 齐绿→deploy_ok") {
				return
			}
			if g.deployStateOK(ctx, st.run.TicketID) && g.deployDone(ctx,
				st.run.TicketID, "席死径态判：已装+socket健康→deploy_ok") {
				return
			}
			if st.run.Shell == "cli-drive" {
				note := "cli-drive 席死相位稳守部署中——继任由调和器派 exit=" +
					fmt.Sprint(st.exitCode)
				_ = g.database.AppendEvent(ctx, st.run.TicketID, "clidrive_dead", db.Event{
					Actor: "gateway", Note: note})
				g.emit(Event{Kind: "clidrive_dead", TicketID: st.run.TicketID,
					Role: "CTO", Note: note})
			} else {
				// 非机械部署席死无收口据→deploy_failed 账（失败表 rollback
				// ≤1：调和器重派一轮，再败→blocked）。T49：期窗去重同 T48。
				g.appendDeployFailedOnce(ctx, st.run.TicketID,
					fmt.Sprintf("部署中席死无据——先核回滚态 run=%s", st.run.RunID))
			}
		}
	}
}

// deadCauseOf T47 死因帧取值：killCause 戳在＝网关令杀（戳即凶名）；
// 无戳＝自退/外因（exit=-1 猝死之谜的下一判据——非网关杀即外因）。
func deadCauseOf(st *runState) string {
	if st == nil {
		return "unknown"
	}
	if st.killCause != "" {
		return st.killCause
	}
	if st.killByGW {
		return "gateway_kill"
	}
	return "self_exit_or_external"
}
