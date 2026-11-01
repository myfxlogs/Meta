package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

	out, err := assemble(inject.Input{
		Root:     g.cfg.InjectRoot,
		Role:     role,
		TicketID: ticketID,
		Title:    t.Title,
		Spec:     ts,
		Verdicts: verdicts,
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
