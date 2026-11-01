package gateway

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// AdminGateway 管理/只读导出面（§12 外手工径件：smoke/CLI/审计取证）。
// 一切写径经这里进网关——CLI 不直连 DB（禁区#6）。

func (g *Gateway) CreateTicket(ctx context.Context, nt *pbv1.NewTicket) (*pbv1.Ack, error) {
	if nt.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id 空")
	}
	// 立票径挂接（票0015）：校验登记先行——未登记即拒票不立；
	// 过检则 project 与票行同 INSERT 原子落入。
	if nt.Project != "" {
		p, err := g.database.GetProject(ctx, nt.Project)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if p == nil {
			return &pbv1.Ack{Ok: false,
				Note: "项目 " + nt.Project + " 无配方登记——票不立"}, nil
		}
	}
	if err := g.database.CreateTicketProv(ctx, nt.Id, nt.Title, nt.Spec, "", nt.Project); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if nt.AttemptCap > 0 || nt.HardCap > 0 {
		if err := g.database.SetBudgetCaps(ctx, nt.Id, nt.AttemptCap, nt.HardCap); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if nt.Project != "" {
		g.emitProjectSet(ctx, nt.Id, nt.Project, "admin", "terminal")
	}
	return &pbv1.Ack{Ok: true}, nil
}

func (g *Gateway) Dispatch(ctx context.Context, req *pbv1.DispatchReq) (*pbv1.Ack, error) {
	if req.TicketId == "" || req.Role == "" {
		return nil, status.Error(codes.InvalidArgument, "ticket_id/role 空")
	}
	err := g.DispatchSeat(ctx, req.TicketId, req.Role, SeatSpec{
		PromptFile:  req.PromptFile,
		WorkDir:     req.WorkDir,
		PreferShell: req.PreferShell,
		Adjunct:     req.Adjunct,
		Provenance:  req.Provenance, // caster 透传触发事件行戳
		Project:     req.Project,    // 派发径挂接（取锁闸之前 attachProject）
	})
	if err != nil {
		return &pbv1.Ack{Ok: false, Note: err.Error()}, nil
	}
	return &pbv1.Ack{Ok: true}, nil
}

func (g *Gateway) SetBudget(ctx context.Context, req *pbv1.BudgetReq) (*pbv1.Ack, error) {
	if err := g.database.SetBudgetCaps(ctx, req.TicketId, req.AttemptCap, req.HardCap); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	// 追加预算复活＝抬帽不清零：仅当新帽确实高于累计用量才解冻
	// （降低帽不解冻——frozen 是止血态不是可绕过闸）。
	if b, err := g.database.GetBudget(ctx, req.TicketId); err == nil &&
		(b.HardCap == 0 || b.UsedTotal < b.HardCap) {
		g.mu.Lock()
		delete(g.frozen, req.TicketId)
		g.mu.Unlock()
		_ = g.database.SetBudgetFrozen(ctx, req.TicketId, false)
	}
	return &pbv1.Ack{Ok: true}, nil
}

func (g *Gateway) Budget(ctx context.Context, ref *pbv1.TicketRef) (*pbv1.BudgetMsg, error) {
	b, err := g.database.GetBudget(ctx, ref.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbv1.BudgetMsg{Cap: b.Cap, Used: b.Used, HardCap: b.HardCap, UsedTotal: b.UsedTotal}, nil
}

func (g *Gateway) Phase(ctx context.Context, ref *pbv1.TicketRef) (*pbv1.PhaseMsg, error) {
	t, err := g.database.GetTicket(ctx, ref.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if t == nil {
		return nil, status.Error(codes.NotFound, "票不存在")
	}
	return &pbv1.PhaseMsg{Phase: string(t.Phase), BudgetFrozen: t.BudgetFrozen}, nil
}

func (g *Gateway) Events(ctx context.Context, ref *pbv1.TicketRef) (*pbv1.EventList, error) {
	evs, err := g.database.Events(ctx, ref.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := &pbv1.EventList{}
	for _, e := range evs {
		out.Items = append(out.Items, eventItem(e))
	}
	return out, nil
}

// EventsAfter 水位游标回读（D-25/播报员）：跨票全序 id>since_id 页。
func (g *Gateway) EventsAfter(ctx context.Context, c *pbv1.EventCursor) (*pbv1.EventList, error) {
	limit := int(c.Limit)
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	evs, err := g.database.EventsAfter(ctx, c.SinceId, limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := &pbv1.EventList{}
	for _, e := range evs {
		out.Items = append(out.Items, eventItem(e))
	}
	return out, nil
}

func eventItem(e *db.Event) *pbv1.EventItem {
	return &pbv1.EventItem{
		Id: e.ID, Kind: e.Kind, FromPhase: string(e.FromPhase), ToPhase: string(e.ToPhase),
		Actor: e.Actor, Note: e.Note, AtUnixNano: e.CreatedAt.UnixNano(),
		TicketId: e.TicketID, Role: e.Role, Provenance: e.Provenance,
	}
}

// RetroVerdict 回翻件（决策40 §15.4）：滞留 待审 票一次性机械补录——
// 两跳皆走钦定边经 transition() 包装（事件行＋门铃照出，路由通知自动跟发）：
// 待审→审计中(audit_start)→待部署(audit_pass) 或 →已派(reject)。
// 闸：相位必须 待审（幂等——二次调用相位已走即拒）；ref 必填（无指证的
// 回翻＝无 provenance 的相位篡改）。补录性质在 actor=admin/note 显式留痕。
func (g *Gateway) RetroVerdict(ctx context.Context, r *pbv1.RetroReq) (*pbv1.Ack, error) {
	if r.TicketId == "" {
		return nil, status.Error(codes.InvalidArgument, "ticket_id 空")
	}
	if r.Ref == "" {
		return &pbv1.Ack{Ok: false, Note: "ref 必填（判决档/决策号指证）"}, nil
	}
	var to db.Phase
	var kind db.EdgeKind
	switch r.Verdict {
	case "pass":
		to, kind = db.PhaseToDeploy, db.EdgeAuditPass
	case "reject":
		to, kind = db.PhaseAssigned, db.EdgeReject
	default:
		return &pbv1.Ack{Ok: false, Note: "verdict 必填 pass|reject"}, nil
	}
	phase, err := g.database.PhaseOf(ctx, r.TicketId)
	if err != nil {
		return &pbv1.Ack{Ok: false, Note: err.Error()}, nil
	}
	if phase != db.PhaseReview {
		return &pbv1.Ack{Ok: false, Note: "回翻闸：相位 " + string(phase) + " 非 待审"}, nil
	}
	if err := g.transition(ctx, r.TicketId, db.PhaseAuditing, db.EdgeAuditStart,
		"admin", "回翻·人工判决补录 ref="+r.Ref); err != nil {
		return &pbv1.Ack{Ok: false, Note: err.Error()}, nil
	}
	// 对抗性锚提名行：第二跳 audit_pass/reject——删则票滞留审计中成
	// 「假审计中」（永远等不来的审计席）。
	if err := g.transition(ctx, r.TicketId, to, kind,
		"admin", "回翻判决 "+r.Verdict+" ref="+r.Ref); err != nil {
		return &pbv1.Ack{Ok: false, Note: err.Error()}, nil
	}
	return &pbv1.Ack{Ok: true, Note: "回翻完成→" + string(to)}, nil
}

// Ping 健康核验径（票0010）：拨通＝活——meta ping / scripts/meta-probe.sh
// 的部署后健康探针落点。
func (g *Gateway) Ping(_ context.Context, _ *pbv1.Empty) (*pbv1.Ack, error) {
	return &pbv1.Ack{Ok: true, Note: "pong"}, nil
}

// IssueCeoToken `meta ceo` 交互面开 run（terminal 戳）：建一条 pid=0 的
// role=CEO seat_run——无子进程（CLI 本体即席）；其 Events 订阅 EOF 即除名+判死。
func (g *Gateway) IssueCeoToken(ctx context.Context, _ *pbv1.CeoTokenReq) (*pbv1.CeoTokenResp, error) {
	run := g.newRun("", "CEO", "")
	run.Provenance = "terminal"
	if err := g.database.CreateRun(ctx, run); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	_ = g.database.AppendEvent(ctx, "", "ceo_token", db.Event{
		Actor: "gateway", Note: "交互面开 run " + run.RunID, Provenance: "terminal"})
	return &pbv1.CeoTokenResp{RunId: run.RunID, RunToken: run.RunToken}, nil
}

// SetTicketProject 存量回填/显式挂接径（票0015）：attachProject 五判闸——
// 未登记/部署中/终态/换挂全拒，同值幂等 ok 无重复事件。挂接只加不换；
// `meta ticket set-project` 是存量票唯一合法写径（CLI→RPC→网关，不破唯一写者）。
func (g *Gateway) SetTicketProject(ctx context.Context, r *pbv1.TicketProjectReq) (*pbv1.Ack, error) {
	if r.TicketId == "" || r.Project == "" {
		return nil, status.Error(codes.InvalidArgument, "ticket_id/project 必填")
	}
	if err := g.attachProject(ctx, r.TicketId, r.Project, "admin", "terminal"); err != nil {
		return &pbv1.Ack{Ok: false, Note: err.Error()}, nil
	}
	return &pbv1.Ack{Ok: true}, nil
}

func (g *Gateway) Runs(ctx context.Context, ref *pbv1.TicketRef) (*pbv1.RunList, error) {
	runs, err := g.database.RunsForTicket(ctx, ref.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := &pbv1.RunList{}
	for _, r := range runs {
		out.Items = append(out.Items, &pbv1.RunItem{
			RunId: r.RunID, Role: r.Role, Shell: r.Shell, Pid: int32(r.PID),
			ExitCode: int32(r.ExitCode), Dead: r.Dead,
			Attached: !r.AttachedAt.IsZero(), Adopted: !r.AdoptedAt.IsZero(),
			StartedAt: r.StartedAt.UnixNano(), ExitedAt: r.ExitedAt.UnixNano(),
			Adjunct: r.Adjunct,
		})
	}
	return out, nil
}

// Observe 事件观察口导出：订阅进程内事件源（socket 断＝订户走，无清理负担）。
func (g *Gateway) Observe(_ *pbv1.Empty, stream pbv1.AdminGateway_ObserveServer) error {
	ch := g.subscribe()
	defer g.unsubscribe(ch)
	for {
		select {
		case e := <-ch:
			if err := stream.Send(&pbv1.ObsEvent{
				Kind: e.Kind, TicketId: e.TicketID, RunId: e.RunID, Role: e.Role, Note: e.Note,
				Provenance: e.Provenance,
			}); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-g.done:
			return nil
		}
	}
}
