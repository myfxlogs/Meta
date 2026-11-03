package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"meta/internal/db"
	"meta/internal/inject"
	pbv1 "meta/internal/pb/metav1"
)

// assemble 注入机组装件（包级缝——测试可换 mock 产物空径，D-40 锚 3）。
var assemble = inject.Assemble

// injectPrompt 注入机落盘径：票 spec 出 DB→组装前缀→写 run/inject/<票>.md，
// 路径填进本次派发的 PromptFile。判决史＝spec 初值＋ticket_event 的
// reject/yield 行动态追加（驳回理由累积，重做者必读）。
func (g *Gateway) injectPrompt(ctx context.Context, ticketID, role string, t *db.Ticket) (string, error) {
	var ts *pbv1.TicketSpec
	if len(t.Spec) > 0 {
		ts = &pbv1.TicketSpec{}
		if err := proto.Unmarshal(t.Spec, ts); err != nil {
			return "", fmt.Errorf("票 spec 反序列化: %w", err)
		}
	}
	if err := inject.CheckSpec(ts); err != nil {
		return "", err
	}

	var verdicts []string
	if evs, err := g.database.Events(ctx, ticketID); err == nil {
		for _, e := range evs {
			if e.Kind == string(db.EdgeReject) || e.Kind == string(db.EdgeYield) {
				verdicts = append(verdicts,
					fmt.Sprintf("- %s: %s（%s）", e.Kind, e.Note, e.CreatedAt.Format("01-02 15:04")))
			}
		}
	}

	// 票内留言（票0030 续传）：全量行入注入前缀——非续会派发这是唯一
	// 带走径，续会派发兜底重述（消息不丢优先于省 token）。
	var msgs, extMsgs []string
	if ms, err := g.database.MsgsForTicket(ctx, ticketID, false); err == nil {
		for _, m := range ms {
			line := fmt.Sprintf("- [%s] %s（%s）", m.Actor, m.Text,
				m.CreatedAt.Format("01-02 15:04"))
			if extSrcActor(m.Actor) {
				extMsgs = append(extMsgs, line)
			} else {
				msgs = append(msgs, line)
			}
		}
	}

	out, err := assemble(inject.Input{
		Root:     g.cfg.InjectRoot,
		Role:     role,
		TicketID: ticketID,
		Title:    t.Title,
		Spec:     ts,
		Verdicts: verdicts,
		Msgs:     msgs,
		ExtMsgs:  extMsgs,
		ExtSrc:   extSrcTicket(ctx, g.database, ticketID),
		Recipes:  g.cfg.InjectRecipes,
	})
	if err != nil {
		return "", err
	}

	dir := filepath.Join(g.cfg.RunDir, "inject")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	// D-28：落盘按角色——同票施工+审计/多角色并发派发不互相覆写。
	p := filepath.Join(dir, ticketID+"-"+role+".md")
	if err := os.WriteFile(p, []byte(out), 0o600); err != nil {
		return "", err
	}
	return p, nil
}

// injectTokenCap T34/V7 注入前缀硬顶（est token）。估算口径：ASCII 4 字符
// ≈1 token，非 ASCII 每 rune ≈1 token（CJK 保守上偏）——无 tokenizer 依赖、
// 纯函数可测；宁可多截不漏顶。
const injectTokenCap = 8000

// estTokens 提示 token 估算（T34，纯函数）：ASCII 计 1/4，非 ASCII rune 计 1。
func estTokens(s string) int {
	n := 0
	for _, r := range s {
		if r < 128 {
			n++ // 末尾 /4
		} else {
			n += 4
		}
	}
	return n / 4
}

// capPromptFile 注入硬顶闸（T34 §3.1/V7）：提示文件估算 >8k token→中段截
// （头 75%＋尾 25%，保哨兵契约末行）落 run/inject/trunc-*.md 副本换路径，
// 原文件不动；截断发 inject_truncated 事件（事件行＋emit，双径）。
// 文件读不出→原样返回（可读性归 D-40 空提示闸判，不在本闸重复）。
func (g *Gateway) capPromptFile(ctx context.Context, req spawnReq, path string) string {
	if path == "" {
		return path
	}
	b, err := os.ReadFile(path)
	if err != nil || estTokens(string(b)) <= injectTokenCap {
		return path
	}
	runes := []rune(string(b))
	budget := injectTokenCap * 4 // 估算分母单位（ASCII 字符当量）
	acc := make([]int, len(runes))
	for i, r := range runes {
		if r < 128 {
			acc[i] = 1
		} else {
			acc[i] = 4
		}
	}
	// 头取 75% 预算、尾取 25% 预算（哨兵契约在尾部——截中段不截尾）。
	head, tail := budget*3/4, budget/4
	hi := 0
	for hi < len(runes) && acc[hi] <= head {
		head -= acc[hi]
		hi++
	}
	ti := len(runes)
	for ti > hi && acc[ti-1] <= tail {
		tail -= acc[ti-1]
		ti--
	}
	marker := fmt.Sprintf("\n\n> [T34 注入硬顶：中段 %d rune 已截——原文 %s]\n\n",
		ti-hi, path)
	out := string(runes[:hi]) + marker + string(runes[ti:])
	dir := filepath.Join(g.cfg.RunDir, "inject")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return path
	}
	p := filepath.Join(dir, fmt.Sprintf("trunc-%s-%s-%d.md", req.ticketID, req.role,
		time.Now().UnixNano()))
	if err := os.WriteFile(p, []byte(out), 0o600); err != nil {
		return path
	}
	note := fmt.Sprintf("提示件截断 %d→%d est token（cap=%d）", estTokens(string(b)),
		estTokens(out), injectTokenCap)
	_ = g.database.AppendEvent(ctx, req.ticketID, "inject_truncated", db.Event{
		Actor: "gateway", Note: note})
	g.emit(Event{Kind: "inject_truncated", TicketID: req.ticketID, Role: req.role,
		Note: note})
	return p
}

// extSrcTicket 票级外源判定（T10）：created 事件 provenance——feishu 需求
// 径立票＝外源，terminal/CEO 径＝内源。
func extSrcTicket(ctx context.Context, d *db.DB, tid string) bool {
	evs, err := d.Events(ctx, tid)
	if err != nil || len(evs) == 0 {
		return false
	}
	return extSrcActor(evs[0].Provenance)
}

// extSrcActor 外源判定（T10）：terminal/内部角色＝内源；feishu:<id>/
// 其他系统外戳＝外源——「指示」降权为材料。
func extSrcActor(a string) bool {
	switch a {
	case "", "terminal", "gateway", "CEO", "CTO", "施工", "审计", "巡检", "board":
		return false
	}
	return strings.HasPrefix(a, "feishu") || strings.Contains(a, ":")
}
