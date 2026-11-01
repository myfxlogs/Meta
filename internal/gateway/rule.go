package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// handleRule 裁决回流（§1.4）：席位流唯一说话处的机械闸——
// run.adjunct && role=="CTO" 才受理，否则拒（非 adjunct 席发 rule＝断流）。
// 动作两枚：kill_requeue（杀施工席+failSet 换壳+既有回队径重派）；
// defer（帽内重挂全额闹钟；帽尽转 defer_limit 事件行——§5.2「报 CEO」机械落点）。
// 「该不该杀/该不该延期」的语义裁决在裁决席（智能体），网关只执机械闸。
func (g *Gateway) handleRule(ctx context.Context, st *runState, r *pbv1.Rule) error {
	if !st.run.Adjunct || st.run.Role != "CTO" {
		return status.Error(codes.InvalidArgument,
			fmt.Sprintf("rule 帧仅 CTO adjunct 裁决席可发（adjunct=%v role=%s）", st.run.Adjunct, st.run.Role))
	}
	tid := st.run.TicketID
	switch r.Action {
	case "kill_requeue":
		g.ruleKillRequeue(ctx, tid, st, r.Note)
	case "defer":
		g.ruleDefer(ctx, tid, st, r.Note)
	default:
		return status.Errorf(codes.InvalidArgument, "未知裁决动作 %q", r.Action)
	}
	return nil
}

// ruleKillRequeue：取该票最新活 non-adjunct run→其壳键进本票 failSet（换壳，
// §5.2）→killByGW（不罚健康）→SIGKILL 进程组→既有 onRunDead 径自动回队+重派。
// 无活施工 run→幂等：相位仍 施工中 则补一条 requeue（防相位卡死无行动者）。
func (g *Gateway) ruleKillRequeue(ctx context.Context, tid string, st *runState, note string) {
	run, _ := g.database.ActiveRunForTicket(ctx, tid)
	if run == nil {
		if phase, _ := g.database.PhaseOf(ctx, tid); phase == db.PhaseWorking {
			_ = g.transition(ctx, tid, db.PhaseAssigned, db.EdgeRequeue,
				"gateway", "裁决：无活席补回队")
			g.cancelAlarm(tid)
			g.emit(Event{Kind: "requeue", TicketID: tid})
		}
	} else {
		// 对抗性锚提名行：施工壳键进 failSet——删则同壳重派不换壳。
		key := g.failKeyForRun(run)
		g.mu.Lock()
		if g.failSet[tid] == nil {
			g.failSet[tid] = map[string]bool{}
		}
		g.failSet[tid][key] = true
		g.mu.Unlock()

		st2 := g.bindRun(run)
		st2.killByGW = true // 网关令杀＝非壳之错，不罚健康（§5.2 换壳不判死）
		g.killRun(st2)
	}
	g.ruleLanded(ctx, tid, st, "kill_requeue", run, note)
}

// ruleDefer：defer_count<K → +1、闹钟重挂全额 ProgressTimeout、记 ruling 行；
// 帽尽 → 不杀不挂，转记 defer_limit 事件行（路由表接通知，闭环收敛不空转）。
func (g *Gateway) ruleDefer(ctx context.Context, tid string, st *runState, note string) {
	t, err := g.database.GetTicket(ctx, tid)
	if err != nil || t == nil {
		return
	}
	if t.DeferCount < deferCap {
		if _, err := g.database.IncrementDefer(ctx, tid); err == nil {
			g.armAlarm(tid, time.Now().Add(g.cfg.ProgressTimeout))
		}
		g.ruleLanded(ctx, tid, st, "defer", nil, note)
		return
	}
	_ = g.database.AppendEvent(ctx, tid, "defer_limit", db.Event{
		Actor: "gateway", Note: "延期帽 K 已尽，等你发落",
	})
	g.emit(Event{Kind: "defer_limit", TicketID: tid})
}

// ruleLanded ruling 落账：裁决已执+动作+理由（§1.4——入表可配通知，
// 业主能见到「卡死已裁」）。
func (g *Gateway) ruleLanded(ctx context.Context, tid string, st *runState, action string, run *db.Run, why string) {
	note := "action=" + action
	if run != nil {
		note += " run=" + run.RunID
	}
	if why != "" {
		note += " " + why
	}
	_ = g.database.AppendEvent(ctx, tid, "ruling", db.Event{
		Actor: st.run.RunID, Note: note})
	g.emit(Event{Kind: "ruling", TicketID: tid, RunID: st.run.RunID,
		Role: st.run.Role, Note: note})
}

// failKeyForRun run→本票 failSet 壳键：优先该 run 内存态的尝试行（name+model
// 全）；重启后内存失则解析 run.Shell 的 "name/model" 标签。
func (g *Gateway) failKeyForRun(run *db.Run) string {
	g.mu.Lock()
	st, ok := g.runs[run.RunID]
	g.mu.Unlock()
	if ok && st.attempt.Shell != "" {
		return db.Shell{Name: st.attempt.Shell, Model: st.attempt.Model, Role: run.Role}.ShellKey()
	}
	name := shellNameOf(run.Shell)
	model := strings.TrimPrefix(run.Shell, name+"/")
	if model == run.Shell {
		model = ""
	}
	return db.Shell{Name: name, Model: model, Role: run.Role}.ShellKey()
}
