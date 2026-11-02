package gateway

import (
	"context"
	"strings"

	"meta/internal/db"
	"meta/internal/feishu"
)

// 活票卡消费（票0014/D-31/令#E14）——网关内第三事件消费者：
// 订户口同步注册（先订阅后对账/起环——同 StartFeishu 形制，防首事件丢窗）
// →启动对账补建（一次性）→逐事件喂 feishu.Cards（一切非 usage 票事件
// →重渲全卡→哈希变才 PATCH）。create 败记 feishu_card_err 事件行
// （DB 真源留痕；不进事件总线——回灌观察口会自触发重渲成环）。

// StartFeishuCards 装配并起卡消费环（serve 径）。凭证与文本通道同源
// （feishuCred 自解）；凭证坏时 FeishuChannel 已发 feishu_cred_bad，
// 此处静默不启（不重复报）。
func (g *Gateway) StartFeishuCards(ctx context.Context) *feishu.Cards {
	cred, ok := g.feishuCred(ctx)
	if !ok {
		return nil
	}
	cards := feishu.NewCards(feishu.CardConfig{
		Cred:        cred,
		View:        g.cardView,
		CardRow:     g.cardRow,
		SaveCardRow: g.saveCardRow,
		NonTerminal: g.nonTerminalIDs,
		RecordErr:   g.recordCardErr,
	})
	evCh := g.subscribe() // 同步注册——对账期间到的事件不丢（256 缓冲排队）
	go func() {
		cards.Reconcile(ctx)
		g.feishuCardsLoop(ctx, cards, evCh)
	}()
	return cards
}

// feishuCardsLoop 卡消费环：逐事件喂 Observe；随 ctx/g.done 收。
func (g *Gateway) feishuCardsLoop(ctx context.Context, cards *feishu.Cards, evCh chan Event) {
	defer g.unsubscribe(evCh)
	for {
		select {
		case e := <-evCh:
			cards.Observe(ctx, e.Kind, e.TicketID)
		case <-ctx.Done():
			return
		case <-g.done:
			return
		}
	}
}

// cardView 渲染输入装配（机械组装非判断——同 board.go 全态帧组装的边界）：
// 票行（相位/标题/驳回/终判）＋活席（role·shell）＋最近进度（stage＝首词、
// note＝全文）＋用量＋最近活性时刻。票不在返回 nil,nil。
func (g *Gateway) cardView(ctx context.Context, ticketID string) (*feishu.CardView, error) {
	t, err := g.database.GetTicket(ctx, ticketID)
	if err != nil || t == nil {
		return nil, err
	}
	v := &feishu.CardView{
		TicketID: t.ID,
		Title:    t.Title,
		Phase:    string(t.Phase),
		Rejects:  t.RejectCount,
		Terminal: t.Phase == db.PhaseDone || t.Phase == db.PhaseVoid,
		At:       t.UpdatedAt,
	}
	if r, err := g.database.ActiveRunForTicket(ctx, ticketID); err == nil && r != nil {
		v.Role, v.Shell = r.Role, r.Shell
	}
	if evs, err := g.database.Events(ctx, ticketID); err == nil {
		for i := len(evs) - 1; i >= 0; i-- {
			if evs[i].Kind == "progress" {
				v.Note = evs[i].Note
				v.Stage, _, _ = strings.Cut(evs[i].Note, " ") // note 形「stage 附言」
				break
			}
		}
	}
	if used, total, err := g.database.Usage(ctx, ticketID); err == nil {
		v.Used, v.UsedTotal = used, total
	}
	if at, err := g.database.LastActivityAt(ctx, ticketID); err == nil {
		v.At = at
	}
	// 自捕率行（票0038-p1/设计-0038 §3 挂点裁定 cardView——纯读径：
	// 判例流只在 ticket_event 读行不写行；非判例事件重算同值由
	// 下游哈希闸挡 PATCH）。读败留空＝卡面「暂无」（既有同构容错）。
	if rows, err := g.database.CatchRows(ctx, catchScanLimit); err == nil {
		v.Catch = classifyCatch(rows).line()
	}
	return v, nil
}

// cardRow feishu_card 读缝（注入 Cards.CardRow）。
func (g *Gateway) cardRow(ctx context.Context, ticketID string) (string, bool, error) {
	c, err := g.database.GetFeishuCard(ctx, ticketID)
	if err != nil || c == nil {
		return "", false, err
	}
	return c.MessageID, true, nil
}

// saveCardRow feishu_card 写缝（注入 Cards.SaveCardRow）。
func (g *Gateway) saveCardRow(ctx context.Context, ticketID, messageID, chatID string) error {
	return g.database.SaveFeishuCard(ctx, ticketID, messageID, chatID)
}

// nonTerminalIDs 启动对账扫描缝（注入 Cards.NonTerminal）。
func (g *Gateway) nonTerminalIDs(ctx context.Context) ([]string, error) {
	ts, err := g.database.NonTerminalTickets(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(ts))
	for _, t := range ts {
		ids = append(ids, t.ID)
	}
	return ids, nil
}

// recordCardErr create 败记事件缝（注入 Cards.RecordErr）——AppendEvent
// 单行留痕；不 g.emit：卡错事件回灌观察口会让 Cards 自触发成环。
func (g *Gateway) recordCardErr(ctx context.Context, ticketID, note string) {
	_ = g.database.AppendEvent(ctx, ticketID, "feishu_card_err", db.Event{
		Actor: "feishu_card", Note: note})
}
