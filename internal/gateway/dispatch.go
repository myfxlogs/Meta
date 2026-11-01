package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"meta/internal/adapter"
	"meta/internal/db"
	"meta/internal/route"
	"meta/internal/seat"
)

// SeatSpec 一次派发的参数（DispatchSeat 入，随票留存至本进程终——
// 重启后缺则 spawn_blocked 事件，不重建假参数）。
type SeatSpec struct {
	PromptFile  string
	WorkDir     string
	Env         []string
	PreferShell string // 非空＝该壳尝试提首（smoke 指定径；不换优先序语义）
	Adjunct     bool   // 辅助席（裁决/诊断，§1.3）：不占相位、必给 prompt_file
	Provenance  string // D9 盖戳源：派发方透传触发事件行/父 run 的戳
}

// specKey specs 表键（§15.1）：(票,角色) 共键——审计派发不覆写施工派发参数，
// 驳回重派施工时原 spec（含 prefer_shell）仍在内存。
type specKey struct {
	ticket string
	role   string
}

type spawnReq struct {
	ticketID string
	role     string
}

// DispatchSeat 派发席位（三支路，§15.5 码位表）：
//   - adjunct 支：闸＝票非终态非冻＋必给 prompt_file＋同票同角色活 adjunct 去重；
//     相位一步不行（裁决席不占相位）。
//   - 审计支（role=="审计"）：闸＝相位==待审；不步行已派——相位推进让位给
//     claim→audit_start（无连接即无审计中）。
//   - 其余 role：机械走边到已派→入 spawn 队列→壳路由。
// 共同：冻票拒派（§5.5）；同角色活席去重（§15.1——焊死 已派 重派双席并存洞）。
func (g *Gateway) DispatchSeat(ctx context.Context, ticketID, role string, spec SeatSpec) error {
	// 禁区#10 机械码位：CEO 只经内部唤起（ensureCEO），一切派发径硬拒——
	// admin 帧/caster/CLI 三路共用此闸。
	if role == "CEO" {
		return fmt.Errorf("dispatch: role=CEO 永拒（禁区#10：CEO 只经内部唤起）")
	}
	t, err := g.database.GetTicket(ctx, ticketID)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("dispatch: 票 %s 不存在", ticketID)
	}
	if t.BudgetFrozen {
		return fmt.Errorf("dispatch: 票 %s 已 budget_frozen", ticketID)
	}
	if spec.Adjunct {
		if spec.PromptFile == "" {
			return fmt.Errorf("dispatch: adjunct 派发必给 prompt_file")
		}
		for _, p := range db.Terminal {
			if t.Phase == p {
				return fmt.Errorf("dispatch: adjunct 相位 %s 不可派", t.Phase)
			}
		}
		if r, err := g.database.ActiveAdjunctForTicketRole(ctx, ticketID, role); err != nil {
			return err
		} else if r != nil {
			return fmt.Errorf("dispatch: 票 %s 已有活 adjunct 席 role=%s", ticketID, role)
		}
	} else {
		if role == "审计" && t.Phase != db.PhaseReview {
			return fmt.Errorf("dispatch: 相位 %s 不可派审计（必须 待审）", t.Phase)
		}
		if r, err := g.database.ActiveRunForTicketRole(ctx, ticketID, role); err != nil {
			return err
		} else if r != nil {
			return fmt.Errorf("dispatch: 票 %s 该角色 %s 已有活席 %s", ticketID, role, r.RunID)
		}
		// B1 项目设计锁（§1.8）：role=CTO && 已立 派发起设计——同项目一票
		// 在 设计中 即拒第二票起设计（锁释放事件补发 spawn_needed 解排队）。
		if role == "CTO" && t.Phase == db.PhaseCreated && t.Project != "" {
			g.mu.Lock()
			holder := g.designLocks[t.Project]
			g.mu.Unlock()
			if holder != "" && holder != ticketID {
				return fmt.Errorf("dispatch: 项目 %s 设计锁由票 %s 持有", t.Project, holder)
			}
		}
		if role != "审计" {
			// 机械走边到已派：已立→设计中→已派（阶段 1 手工径无 CTO，网关直行）。
			for t.Phase != db.PhaseAssigned {
				var to db.Phase
				var kind db.EdgeKind
				switch t.Phase {
				case db.PhaseCreated:
					to, kind = db.PhaseDesigning, db.EdgeDesignStart
				case db.PhaseDesigning:
					to, kind = db.PhaseAssigned, db.EdgeDispatch
				default:
					return fmt.Errorf("dispatch: 相位 %s 不可派", t.Phase)
				}
				if err := g.transition(ctx, ticketID, to, kind, "gateway", "派发"); err != nil {
					return err
				}
				t.Phase = to
				// 进设计中即挂项目设计锁（transition 内离开 设计中 时释放）。
				if to == db.PhaseDesigning && t.Project != "" {
					g.mu.Lock()
					g.designLocks[t.Project] = ticketID
					g.mu.Unlock()
				}
			}
		}
	}
	if err := g.database.EnsureBudgetDefaults(ctx, ticketID,
		g.attemptCapFor(role), g.cfg.TicketHardCap); err != nil {
		return err
	}
	g.mu.Lock()
	g.specs[specKey{ticketID, role}] = spec
	g.mu.Unlock()
	g.enqueueSpawn(ticketID, role)
	return nil
}

// attemptCapFor 角色尝试帽缺省：角色行未配回 default_attempt_cap（§7 毛口径）。
func (g *Gateway) attemptCapFor(role string) int64 {
	if c, ok := g.cfg.RoleCaps[role]; ok {
		return c
	}
	return g.cfg.DefaultAttemptCap
}

// enqueueSpawn 入 spawn 队列（不丢——冻票在消费端判弃，即「取消排队事件」）。
func (g *Gateway) enqueueSpawn(ticketID, role string) {
	select {
	case g.spawnQ <- spawnReq{ticketID: ticketID, role: role}:
	default:
		g.emit(Event{Kind: "spawnq_full", TicketID: ticketID, Role: role})
	}
}

// dispatcher spawn 消费环：串行处理派发请求。
func (g *Gateway) dispatcher() {
	for {
		select {
		case req := <-g.spawnQ:
			g.dispatchOnce(req)
		case <-g.done:
			return
		}
	}
}

// dispatchOnce 一轮壳路由：候选序 →（错峰/并发闸）→ spawn → attach/秒退窗口。
func (g *Gateway) dispatchOnce(req spawnReq) {
	ctx := context.Background()
	t, err := g.database.GetTicket(ctx, req.ticketID)
	if err != nil || t == nil || t.BudgetFrozen {
		return // 冻票：排队事件即此取消
	}
	g.mu.Lock()
	spec, ok := g.specs[specKey{req.ticketID, req.role}]
	frozen := g.frozen[req.ticketID]
	g.mu.Unlock()
	if frozen {
		return
	}
	if !ok {
		_ = g.database.AppendEvent(ctx, req.ticketID, "spawn_blocked", db.Event{
			Actor: "gateway", Note: "无派发参数（重启后失）；需重新 Dispatch role=" + req.role})
		g.emit(Event{Kind: "spawn_blocked", TicketID: req.ticketID, Role: req.role,
			Note: "无派发参数（重启后失）；需重新 Dispatch"})
		return
	}
	// 活席去重消费端闸（§15.1/§15.2）：后到派发径在此幂等相会——同角色
	// 活席已立（含排队中先到者已 spawn 未 claim 的窗）即弃，留痕。
	if spec.Adjunct {
		if r, _ := g.database.ActiveAdjunctForTicketRole(ctx, req.ticketID, req.role); r != nil {
			g.emit(Event{Kind: "spawn_dup", TicketID: req.ticketID, Role: req.role,
				Note: "活 adjunct 席在 " + r.RunID})
			return
		}
	} else {
		if r, _ := g.database.ActiveRunForTicketRole(ctx, req.ticketID, req.role); r != nil {
			g.emit(Event{Kind: "spawn_dup", TicketID: req.ticketID, Role: req.role,
				Note: "该角色已有活席 " + r.RunID})
			return
		}
	}

	// 注入机径：无 --prompt-file 覆盖且注入机开 → 组装提示前缀文件。
	// 组装失败＝缺一不派（§6.1），出事件即返——非壳之错，不耗壳尝试。
	if spec.PromptFile == "" && g.cfg.InjectRoot != "" {
		pf, err := g.injectPrompt(ctx, req.ticketID, req.role, t)
		if err != nil {
			_ = g.database.AppendEvent(ctx, req.ticketID, "inject_failed", db.Event{
				Actor: "gateway", Note: err.Error()})
			g.emit(Event{Kind: "inject_failed", TicketID: req.ticketID, Role: req.role,
				Note: err.Error()})
			return
		}
		spec.PromptFile = pf
	}

	rows, err := g.shellsFor(ctx, req.role)
	if err != nil {
		return
	}
	g.mu.Lock()
	excl := g.failSet[req.ticketID]
	g.mu.Unlock()
	attempts := route.Candidates(rows, excl, g.cfg.Stagger)
	prefer := spec.PreferShell
	// 同壳重做落点（§15.2）：施工驳回后重派若无 prefer，机械补＝票上最近
	// non-adjunct 施工 run 的壳名（④级同壳＝模型一致性偏好；壳已在 failSet
	// 排除集则落下一行，不算破例）。
	if prefer == "" && req.role == "施工" && t.RejectCount > 0 {
		if rs, err := g.database.RunsForTicket(ctx, req.ticketID); err == nil {
			for i := len(rs) - 1; i >= 0; i-- {
				if rs[i].Role == "施工" && !rs[i].Adjunct {
					prefer = shellNameOf(rs[i].Shell)
					break
				}
			}
		}
	}
	if prefer != "" {
		attempts = preferFirst(attempts, prefer)
	}
	if len(attempts) == 0 {
		g.shellsExhausted(ctx, req)
		return
	}

	deferred := false
	for _, at := range attempts {
		if at.Stagger > 0 { // ②a 错峰：一次性闹钟等待
			if !waitOnce(at.Stagger, g.done) {
				return
			}
		}
		if g.cfg.MaxPerShell > 0 && g.activeCount(at.Shell) >= g.cfg.MaxPerShell {
			deferred = true // provider 并发闸：本轮跳过，错峰后重排
			continue
		}
		// 新尝试起席：尝试内用量清零＋80% 警复位（§5.5 闸是尝试内闸——
		// 换壳重做不得背着上一尝试的 used，首帧即被 100% 硬停秒杀）。
		// 在 spawn 前清而非起席成功点：快壳 attach 后首帧用量可能先于清零落库。
		// adjunct 席不是尝试（§1.3）：跳过清零/复位，不清施工的尝试账。
		if !spec.Adjunct {
			// 该行 attempt_cap>0＝本行尝试帽覆盖角色缺省（§7 opencode 低帽落点）。
			if cap, ok := g.attemptCaps[db.Shell{Name: at.Shell, Model: at.Model, Role: req.role}.ShellKey()]; ok {
				// D-36：帽取不到不静默落 0——库瞬时错时硬顶会被盖掉。
				b, err := g.budgetOf(ctx, req.ticketID)
				if err != nil {
					return
				}
				_ = g.database.SetBudgetCaps(ctx, req.ticketID, cap, b.HardCap)
			}
			if err := g.database.ResetAttemptUsage(ctx, req.ticketID); err != nil {
				_ = g.database.AppendEvent(ctx, req.ticketID, "budget_reset_failed", db.Event{
					Actor: "gateway", Note: "尝试内用量清零失败：" + err.Error()})
				g.emit(Event{Kind: "budget_reset_failed", TicketID: req.ticketID, Role: req.role,
					Note: err.Error()})
				return // 闸不可保即摆位（非壳之错不进 failSet）——库恢复后 CTO 重派
			}
			g.mu.Lock()
			delete(g.warned, req.ticketID)
			g.mu.Unlock()
		}
		st, code := g.trySpawn(ctx, req, spec, at)
		if st != nil { // 席位立住：本票派发完成
			g.onSeatAlive(ctx, st, at)
			return
		}
		// 起不来/秒退：分级记 failSet＋健康迁移，换下一行（①②）。
		class := seat.ClassOfExit(code)
		g.reportShell(ctx, req.ticketID, req.role, at, class)
	}
	if deferred {
		// 全部候选被并发闸挡住：错峰后重排一次（一次性闹钟链，有界语义——
		// 重排进来时若仍全挡将再次错峰；不是周期巡检，是事件驱动的延后）。
		time.AfterFunc(g.cfg.Stagger, func() { g.enqueueSpawn(req.ticketID, req.role) })
		return
	}
	g.shellsExhausted(ctx, req)
}

// trySpawn 起一枚候选：spawn 外壳→等 attach 或秒退窗口。返回 (nil, code)=失败。
func (g *Gateway) trySpawn(ctx context.Context, req spawnReq, spec SeatSpec, at route.Attempt) (*runState, int) {
	run := g.newRun(req.ticketID, req.role, at.Shell)
	run.Shell = shellLabel(at)
	run.Adjunct = spec.Adjunct
	run.Provenance = spec.Provenance // D9：spawn 方盖戳——触发事件行的 provenance 透传
	artDir := filepath.Join(g.cfg.RunDir, "artifacts")
	_ = os.MkdirAll(artDir, 0o700)
	artDir = filepath.Join(artDir, run.RunID)
	_ = os.MkdirAll(artDir, 0o700)

	env := append([]string{}, os.Environ()...) // 基线全量（PATH/HOME/壳登录态）
	env = append(env, spec.Env...)
	env = append(env,
		"META_SHELL="+at.Shell,
		"META_MODEL="+at.Model,
		"META_ROLE="+req.role,
		"META_PROMPT_FILE="+spec.PromptFile,
		"META_WORKDIR="+spec.WorkDir,
		"META_ART_DIR="+artDir,
	)
	argv := g.cfg.SeatArgv
	if len(argv) == 0 {
		argv = []string{g.cfg.SelfPath, "seat"}
	}
	if err := g.startRun(ctx, run, argv, env); err != nil {
		_ = g.database.AppendEvent(ctx, req.ticketID, "spawn_failed", db.Event{
			Actor: "gateway", Note: shellLabel(at) + ": " + err.Error()})
		return nil, seat.ExitSpawnFail
	}
	st := g.bindRun(run)
	st.attempt = at

	// ①秒退窗口：attach 即立住；窗口内非零退＝①/② 换行。窗口到未死未连
	// （壳子进程启动慢）＝视立住放行——进度闹钟兜底。
	select {
	case <-st.attachedCh:
		return st, 0
	case <-st.exitCh:
		return nil, st.exitCode
	case <-oneShot(g.cfg.QuickExit, g.done):
		return st, 0
	}
}

// onSeatAlive 席位立住：计并发＋懒探恢复回报。
func (g *Gateway) onSeatAlive(ctx context.Context, st *runState, at route.Attempt) {
	g.mu.Lock()
	g.active[at.Shell]++
	g.mu.Unlock()
	if at.Probe {
		row := db.Shell{Name: at.Shell, Model: at.Model, Role: st.run.Role}
		_ = g.database.SetShellHealth(ctx, row, route.HealthOK)
		_ = g.database.AppendEvent(ctx, st.run.TicketID, "shell_recovered", db.Event{
			Actor: "gateway", Note: shellLabel(at)})
		g.emit(Event{Kind: "shell_recovered", TicketID: st.run.TicketID, Role: st.run.Role,
			Note: shellLabel(at)})
	}
	g.emit(Event{Kind: "seat_up", TicketID: st.run.TicketID, RunID: st.run.RunID, Role: st.run.Role})
}

// reportShell 失效分级落账：任何壳级失败都进本票 failSet（换壳＝排除，
// 票级重做）；②a/②b 按表迁健康态并发必报事件（①机器级不标壳）。
func (g *Gateway) reportShell(ctx context.Context, ticketID, role string, at route.Attempt, class adapter.ExitClass) {
	row := db.Shell{Name: at.Shell, Model: at.Model, Role: role}
	g.mu.Lock()
	if g.failSet[ticketID] == nil {
		g.failSet[ticketID] = map[string]bool{}
	}
	g.failSet[ticketID][row.ShellKey()] = true
	g.mu.Unlock()

	cur, _ := g.database.ShellHealth(ctx, row)
	next, changed := route.NextHealth(cur, class)
	if !changed {
		return
	}
	_ = g.database.SetShellHealth(ctx, row, next)
	kind := map[string]string{
		route.HealthLimited: "shell_limited",
		route.HealthDead:    "shell_dead",
	}[next]
	_ = g.database.AppendEvent(ctx, ticketID, kind, db.Event{Actor: "gateway", Note: shellLabel(at)})
	g.emit(Event{Kind: kind, TicketID: ticketID, Role: role, Note: shellLabel(at)})
}

// shellsExhausted 全壳耗尽：事件→CEO（§8.1 必报）。
func (g *Gateway) shellsExhausted(ctx context.Context, req spawnReq) {
	_ = g.database.AppendEvent(ctx, req.ticketID, "shells_exhausted", db.Event{
		Actor: "gateway", Note: "角色 " + req.role + " 全部候选用尽"})
	g.emit(Event{Kind: "shells_exhausted", TicketID: req.ticketID, Role: req.role})
}

// shellsFor 角色登记行；空表按配置行落种再读（D-9：登记表种子出码外——
// defaultRegistry 码内字面量已废，种子＝config/gateway.textproto 的 shell 行；
// 角色行空才落，DB 行是活态，健康迁移仍走 SetShellHealth）。
func (g *Gateway) shellsFor(ctx context.Context, role string) ([]db.Shell, error) {
	rows, err := g.database.ListShells(ctx, role)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		for _, s := range g.cfg.ShellRows {
			if s.Role != role {
				continue
			}
			_ = g.database.UpsertShell(ctx, db.Shell{
				Name: s.Name, Model: s.Model, Role: s.Role,
				Priority: s.Priority, Health: route.HealthUnknown})
		}
		rows, err = g.database.ListShells(ctx, role)
	}
	return rows, err
}

// shellNameOf 从 "name/model" 标签取壳名（登记表 key 第一段）。
func shellNameOf(label string) string {
	if i := strings.IndexByte(label, '/'); i >= 0 {
		return label[:i]
	}
	return label
}

// preferFirst 把指定壳的尝试提到队首（不改序内相对关系）。
func preferFirst(attempts []route.Attempt, shell string) []route.Attempt {
	var head, tail []route.Attempt
	for _, a := range attempts {
		if a.Shell == shell {
			head = append(head, a)
		} else {
			tail = append(tail, a)
		}
	}
	return append(head, tail...)
}

func shellLabel(at route.Attempt) string {
	if at.Model == "" {
		return at.Shell
	}
	return at.Shell + "/" + at.Model
}

func (g *Gateway) activeCount(shell string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.active[shell]
}

// killRun 杀整进程组（外壳+壳子进程同组，spawn 时 Setpgid）。
func (g *Gateway) killRun(st *runState) {
	if st.run.PID > 0 {
		_ = syscall.Kill(-st.run.PID, syscall.SIGKILL) // 负 pid＝进程组
	}
}

// oneShot 一次性等待：d 到或 done 闭。返回 true=等到点。
func oneShot(d time.Duration, done <-chan struct{}) <-chan struct{} {
	ch := make(chan struct{})
	t := time.AfterFunc(d, func() { close(ch) })
	go func() {
		select {
		case <-done:
			t.Stop()
			close(ch)
		case <-ch:
		}
	}()
	return ch
}

func waitOnce(d time.Duration, done <-chan struct{}) bool {
	fired := oneShot(d, done)
	select {
	case <-fired:
		return true
	case <-done:
		return false
	}
}
