package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
}

type spawnReq struct {
	ticketID string
	role     string
}

// DispatchSeat 派发席位：票面机械走到已派→入 spawn 队列→壳路由。
// 冻票拒派（§5.5：budget_frozen 阻后续 spawn）。
func (g *Gateway) DispatchSeat(ctx context.Context, ticketID, role string, spec SeatSpec) error {
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
		if err := g.database.Transition(ctx, ticketID, to, kind, "gateway", "手工径派发"); err != nil {
			return err
		}
		t.Phase = to
	}
	g.mu.Lock()
	g.specs[ticketID] = spec
	g.mu.Unlock()
	g.enqueueSpawn(ticketID, role)
	return nil
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
	spec, ok := g.specs[req.ticketID]
	frozen := g.frozen[req.ticketID]
	g.mu.Unlock()
	if frozen {
		return
	}
	if !ok {
		g.emit(Event{Kind: "spawn_blocked", TicketID: req.ticketID, Role: req.role,
			Note: "无派发参数（重启后失）；需重新 Dispatch"})
		_ = g.database.AppendEvent(ctx, req.ticketID, "spawn_blocked", db.Event{Actor: "gateway"})
		return
	}

	rows, err := g.shellsFor(ctx, req.role)
	if err != nil {
		return
	}
	g.mu.Lock()
	excl := g.failSet[req.ticketID]
	g.mu.Unlock()
	attempts := route.Candidates(rows, excl, g.cfg.Stagger)
	if spec.PreferShell != "" {
		attempts = preferFirst(attempts, spec.PreferShell)
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
		g.emit(Event{Kind: "shell_recovered", TicketID: st.run.TicketID, Role: st.run.Role,
			Note: shellLabel(at)})
		_ = g.database.AppendEvent(ctx, st.run.TicketID, "shell_recovered", db.Event{
			Actor: "gateway", Note: shellLabel(at)})
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
	g.emit(Event{Kind: kind, TicketID: ticketID, Role: role, Note: shellLabel(at)})
	_ = g.database.AppendEvent(ctx, ticketID, kind, db.Event{Actor: "gateway", Note: shellLabel(at)})
}

// shellsExhausted 全壳耗尽：事件→CEO（§8.1 必报）。
func (g *Gateway) shellsExhausted(ctx context.Context, req spawnReq) {
	g.emit(Event{Kind: "shells_exhausted", TicketID: req.ticketID, Role: req.role})
	_ = g.database.AppendEvent(ctx, req.ticketID, "shells_exhausted", db.Event{
		Actor: "gateway", Note: "角色 " + req.role + " 全部候选用尽"})
}

// shellsFor 角色登记行；空表按 §7 缺省序落种再读。
func (g *Gateway) shellsFor(ctx context.Context, role string) ([]db.Shell, error) {
	rows, err := g.database.ListShells(ctx, role)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		for _, s := range defaultRegistry(role) {
			_ = g.database.UpsertShell(ctx, s)
		}
		rows, err = g.database.ListShells(ctx, role)
	}
	return rows, err
}

// defaultRegistry §7 钦定缺省序（priority 小者优先；model 空＝壳自身缺省）。
func defaultRegistry(role string) []db.Shell {
	devinModel := "swe-2-max" // CTO/审计
	if role == "施工" {
		devinModel = "swe-2-high" // 施工 medium/high 档
	}
	if role == "CEO" {
		return nil // CEO 行业主写死，不落种
	}
	return []db.Shell{
		{Name: "devin", Model: devinModel, Role: role, Priority: 10, Health: route.HealthUnknown},
		{Name: "claude", Model: "", Role: role, Priority: 20, Health: route.HealthUnknown},
		{Name: "opencode", Model: "", Role: role, Priority: 30, Health: route.HealthUnknown},
	}
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
