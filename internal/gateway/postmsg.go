package gateway

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// PostMsg 票内留言入站（票0030 续传，meta msg）：落 ticket_msg＋msg 事件行
// →票上活 attached 非 adjunct 席顺流直投 Feed（seat 落 inbox.log）；
// 无活席则留言躺行——下次注入前缀/续传提示带走，不丢。
func (g *Gateway) PostMsg(ctx context.Context, r *pbv1.MsgReq) (*pbv1.Ack, error) {
	if r.TicketId == "" || strings.TrimSpace(r.Text) == "" {
		return nil, status.Error(codes.InvalidArgument, "ticket_id/text 必填")
	}
	t, err := g.database.GetTicket(ctx, r.TicketId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if t == nil {
		return &pbv1.Ack{Ok: false, Note: "票 " + r.TicketId + " 不存在"}, nil
	}
	actor := r.Actor
	if actor == "" {
		actor = "terminal"
	}
	id, err := g.database.AppendMsg(ctx, r.TicketId, actor, r.Text)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	_ = g.database.AppendEvent(ctx, r.TicketId, "msg", db.Event{
		Actor: actor, Note: r.Text})
	g.emit(Event{Kind: "msg", TicketID: r.TicketId, Note: r.Text})
	runID := g.deliverFeed(r.TicketId, &pbv1.Feed{Id: id, Actor: actor, Text: r.Text})
	if runID != "" {
		_ = g.database.MarkMsgDelivered(ctx, id, runID)
		return &pbv1.Ack{Ok: true, Note: "msg 落行＋直投活席 " + runID}, nil
	}
	return &pbv1.Ack{Ok: true, Note: "msg 落行（无活席，随下次注入/续传带走）"}, nil
}

// InjectEvent 外部事件注入（票0030，meta inject）：emit 全渠道＋带票落
// ticket_event 行——systemd timer 心跳等外源拍，网内零 cron。
func (g *Gateway) InjectEvent(_ context.Context, r *pbv1.EventReq) (*pbv1.Ack, error) {
	if strings.TrimSpace(r.Kind) == "" {
		return nil, status.Error(codes.InvalidArgument, "kind 必填")
	}
	actor := r.Actor
	if actor == "" {
		actor = "inject"
	}
	ctx := context.Background()
	if r.TicketId != "" {
		_ = g.database.AppendEvent(ctx, r.TicketId, r.Kind, db.Event{
			Actor: actor, Note: r.Note, Provenance: actor})
	}
	g.emit(Event{Kind: r.Kind, TicketID: r.TicketId, Note: r.Note, Provenance: actor})
	return &pbv1.Ack{Ok: true, Note: "event " + r.Kind + " injected"}, nil
}

// deliverFeed 票内留言直投：票上最新活非 adjunct run 有存活流即推 Feed
// 帧，返回消费 run_id；无活席/流未立→""（调用方不标投递，留言待续径）。
func (g *Gateway) deliverFeed(ticketID string, f *pbv1.Feed) string {
	run, err := g.database.ActiveRunForTicket(context.Background(), ticketID)
	if err != nil || run == nil {
		return ""
	}
	g.mu.Lock()
	st := g.runs[run.RunID]
	g.mu.Unlock()
	if st == nil {
		return ""
	}
	st.sendMu.Lock()
	defer st.sendMu.Unlock()
	if st.stream == nil {
		return ""
	}
	if err := st.stream.Send(&pbv1.GatewayMsg{
		Body: &pbv1.GatewayMsg_Feed{Feed: f}}); err != nil {
		return ""
	}
	return run.RunID
}
