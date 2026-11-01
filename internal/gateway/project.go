package gateway

import (
	"context"
	"fmt"

	"meta/internal/db"
)

// 票→项目挂接线（票0015，设计 req-9ce5）：挂接只加不换——""→P 或 P→P（幂等）
// 放行，X→Y（X 非空）是换挂一律拒（换挂语义＝作废重立或专门裁决，非机械闸
// 副业）。五判全机械：查票/查登记/相位集比对/等值/非空比对（禁区#7 边界内）。

// attachProject 共用机械闸（五判，全机械）：
//  1. 票不存在 → 拒；
//  2. project 未登记（GetProject nil）→ 拒「项目 X 无配方登记」；
//  3. 相位 ∈ {部署中, 完成, 作废} → 拒——部署中票的项目锁已按旧键取，
//     换键＝锁孤儿；终态票无挂义；
//  4. t.Project == project → 幂等 ok，不写无事件；
//  5. t.Project != "" 且 ≠ → 拒「票已挂 Y——换挂非本径」。
//
// 过闸：SetTicketProject＋project_set 事件行＋emit（actor/prov 随行）；
// 拒亦留痕：project_set 行 note 前缀 denied:（设计 §5——撞闸票停发原位但
// 尝试本身可审）。空项目挂接无锁孤儿：设计锁只在派发时 t.Project!="" 才取
// （空票从不持设计锁），部署锁只在 待部署→CTO 派发取——""→P 于任意非终态
// 非部署中相位安全。
func (g *Gateway) attachProject(ctx context.Context, tid, project, actor, prov string) error {
	t, err := g.database.GetTicket(ctx, tid)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("票 %s 不存在", tid)
	}
	deny := func(why string) error {
		_ = g.database.AppendEvent(ctx, tid, "project_set", db.Event{
			Actor: actor, Note: "denied: " + why, Provenance: prov})
		g.emit(Event{Kind: "project_set", TicketID: tid, RunID: actor,
			Note: "denied: " + why, Provenance: prov})
		return fmt.Errorf("%s", why)
	}
	// 对抗性锚提名行（锚 c）：登记校验——删则未登记项目挂成功。
	proj, err := g.database.GetProject(ctx, project)
	if err != nil {
		return err
	}
	if proj == nil {
		return deny("项目 " + project + " 无配方登记")
	}
	switch t.Phase {
	case db.PhaseDeploying, db.PhaseDone, db.PhaseVoid:
		return deny("票 " + tid + " 相位 " + string(t.Phase) + " 不可挂接（部署中/终态禁换锁键）")
	}
	if t.Project == project {
		return nil // 幂等：不写无事件
	}
	// 对抗性锚提名行（锚 a）：换挂判等——翻成 == 则换挂票挂上。
	if t.Project != "" {
		return deny("票 " + tid + " 已挂 " + t.Project + "——换挂非本径")
	}
	if err := g.database.SetTicketProject(ctx, tid, project); err != nil {
		return err
	}
	g.emitProjectSet(ctx, tid, project, actor, prov)
	return nil
}

// emitProjectSet project_set 留痕：ticket_event 行＋emit（观察者径）——
// 非相位边、非 CEO 事件，管理动作透明（路由表配 notify）。立票径与挂接径共用。
func (g *Gateway) emitProjectSet(ctx context.Context, tid, project, actor, prov string) {
	_ = g.database.AppendEvent(ctx, tid, "project_set", db.Event{
		Actor: actor, Note: "project=" + project, Provenance: prov})
	g.emit(Event{Kind: "project_set", TicketID: tid, RunID: actor,
		Note: "project=" + project, Provenance: prov})
}
