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
	st.attached = true
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
		st = &runState{run: run}
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
		return g.handleDeliver(ctx, st, b.Deliver)
	case *pbv1.SeatMsg_Yield:
		return g.handleYield(ctx, st, b.Yield)
	case *pbv1.SeatMsg_PermissionReply:
		return g.database.AppendEvent(ctx, st.run.TicketID, "permission_reply", db.Event{
			Actor: st.run.RunID, Note: fmt.Sprintf("ask=%s allow=%v", b.PermissionReply.AskId, b.PermissionReply.Allow),
		})
	case *pbv1.SeatMsg_SecretReq:
		return g.handleSecret(ctx, st, stream, b.SecretReq)
	case *pbv1.SeatMsg_Auth:
		return status.Error(codes.InvalidArgument, "重复 RunAuth")
	default:
		return status.Error(codes.InvalidArgument, "空消息")
	}
}

// handleClaim 先连后领（§5.1）：已派→施工中 只发生在连接内。
func (g *Gateway) handleClaim(ctx context.Context, st *runState, c *pbv1.Claim) error {
	if c.TicketId != st.run.TicketID {
		return status.Errorf(codes.PermissionDenied, "票 %s 非本 run 所派", c.TicketId)
	}
	phase, err := g.database.PhaseOf(ctx, st.run.TicketID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if phase == db.PhaseWorking {
		return nil // 收养后重领：幂等
	}
	if err := g.database.Transition(ctx, st.run.TicketID, db.PhaseWorking, db.EdgeClaim, st.run.RunID, ""); err != nil {
		var ille *db.ErrIllegalEdge
		if errors.As(err, &ille) {
			return status.Error(codes.FailedPrecondition, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
	g.armAlarm(st.run.TicketID, time.Now().Add(g.cfg.ProgressTimeout))
	g.emit(Event{Kind: "phase:claim", TicketID: st.run.TicketID, RunID: st.run.RunID})
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

func (g *Gateway) handleUsage(ctx context.Context, st *runState, u *pbv1.Usage) error {
	if err := g.database.AddUsage(ctx, st.run.TicketID, u.TokensUsed); err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func (g *Gateway) handleDeliver(ctx context.Context, st *runState, d *pbv1.Deliver) error {
	if err := g.database.Transition(ctx, st.run.TicketID, db.PhaseReview, db.EdgeDeliver,
		st.run.RunID, d.Ref+" "+d.Note); err != nil {
		var ille *db.ErrIllegalEdge
		if errors.As(err, &ille) {
			return status.Error(codes.FailedPrecondition, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
	g.cancelAlarm(st.run.TicketID)
	g.emit(Event{Kind: "phase:deliver", TicketID: st.run.TicketID, RunID: st.run.RunID})
	return nil
}

// handleYield 补边：施工中→已派，理由入判决史并触发 CTO 裁决事件（§6.2）。
func (g *Gateway) handleYield(ctx context.Context, st *runState, y *pbv1.Yield) error {
	note := y.Kind + ": " + y.Reason
	if err := g.database.Transition(ctx, st.run.TicketID, db.PhaseAssigned, db.EdgeYield,
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

// onRunDead 判死落库＋票回队（施工中→已派）。其他相位只出 seat_dead 事件
// 交启动对账/上层处置——网关不做语义裁决。
func (g *Gateway) onRunDead(st *runState) {
	ctx := context.Background()
	_ = g.database.MarkDead(ctx, st.run.RunID)
	g.emit(Event{Kind: "seat_dead", TicketID: st.run.TicketID, RunID: st.run.RunID, Role: st.run.Role})

	phase, err := g.database.PhaseOf(ctx, st.run.TicketID)
	if err != nil {
		return
	}
	if phase == db.PhaseWorking {
		if err := g.database.Transition(ctx, st.run.TicketID, db.PhaseAssigned, db.EdgeRequeue,
			"gateway", "席位死亡：SIGCHLD+EOF 双源互证"); err == nil {
			g.cancelAlarm(st.run.TicketID)
			g.emit(Event{Kind: "requeue", TicketID: st.run.TicketID, RunID: st.run.RunID})
		}
	}
}
