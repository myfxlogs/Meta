package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"meta/internal/db"
	"meta/internal/inject"
	pbv1 "meta/internal/pb/metav1"
	"meta/internal/route"
)

// ceo.go —— CEO 面五 RPC（总设计 §12）＋连接登记锁＋事件泵＋无头席内部唤起
// ＋provenance 全链盖戳（D9）＋席位流 admin 帧（§1.5）＋req_pending 泊车
// ＋D-22 收口比对。票0011 核心件；飞书入站馈口（0013）复用本泵。
//
// 机械判清单（禁区#7 边界内）：锁登记互斥、kind 集查表、连败计数、
// provenance 盖取、记忆写数比对、角色/相位闸。「该不该裁」永远归 CEO 席。

// ---------- 锁＝连接登记（禁区#10/B8：互斥＋排队，非 DB 旗非阻塞） ----------

// ceoByToken 五 RPC 共用首验：token→活 run→role 必须 CEO。
func (g *Gateway) ceoByToken(ctx context.Context, token string) (*db.Run, error) {
	run, err := g.database.RunByToken(ctx, token)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if run == nil || run.Dead {
		return nil, status.Error(codes.Unauthenticated, "run_token 无效")
	}
	if run.Role != "CEO" {
		return nil, status.Error(codes.PermissionDenied, "token 非 CEO run")
	}
	return run, nil
}

// registerCEO Events 订阅登记：一人订阅在，第二订阅拒（FailedPrecondition）。
// 对抗性锚①提名行：删本分支＝双 CEO 并存。
func (g *Gateway) registerCEO(runID string) (chan Event, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.ceoSub != nil {
		return nil, false
	}
	ch := make(chan Event, 256)
	g.ceoSub = ch
	g.ceoSubRunID = runID
	return ch, true
}

// deregisterCEO EOF 除名＋回放 pending 到该订阅前不得丢——队列保留给下一订阅。
// pid=0 的 token run（meta ceo 交互面，无进程无 attach）随 EOF 判死＋D-22 比对。
func (g *Gateway) deregisterCEO(run *db.Run) {
	g.mu.Lock()
	if g.ceoSubRunID == run.RunID {
		g.ceoSub = nil
		g.ceoSubRunID = ""
	}
	g.mu.Unlock()
	if run.AttachedAt.IsZero() && run.PID == 0 { // token run：流在人在，流散人散
		ctx := context.Background()
		_ = g.database.MarkDead(ctx, run.RunID)
		g.ceoGapCheck(ctx, run.RunID)
	}
}

// ceoActiveSub 取活跃订阅道（nil＝无）。
func (g *Gateway) ceoActiveSub() chan Event {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ceoSub
}

// ceoBusy 无头唤起去重闸：已有订阅或活 CEO run（pid>0 且核活在）即不再起。
func (g *Gateway) ceoBusy(ctx context.Context) bool {
	if g.ceoActiveSub() != nil {
		return true
	}
	run, _ := g.database.ActiveRunForRole(ctx, "CEO")
	return run != nil && run.PID > 0 && processAlive(run.PID)
}

// ---------- 事件泵（订阅口消费同一事件源；spawn_kind→订阅/排队+唤起） ----------

// ceoPump 每事件两判（纯机械）：kind∈spawn_kinds→有订阅随流派发、无→pending
// +ensureCEO；kind∈ceo_events→有订阅随流透传。串行消费故唤起无竞。
// 订阅在 New() 同步建好再交本 goroutine——消「先 emit 后订阅」的丢事件竞。
func (g *Gateway) ceoPump(ch chan Event) {
	defer g.unsubscribe(ch)
	for {
		select {
		case e := <-ch:
			g.mu.Lock()
			spawnKind := g.cfg.CeoSpawnKinds[e.Kind]
			pass := spawnKind || g.cfg.CeoEvents[e.Kind]
			sub := g.ceoSub
			g.mu.Unlock()
			if !pass {
				continue
			}
			if sub != nil {
				select {
				case sub <- e:
					g.mu.Lock()
					g.ceoDispatched[g.ceoSubRunID]++
					g.mu.Unlock()
				default: // 订阅道满不阻塞泵（持久日志在 ticket_event）
				}
				continue
			}
			if spawnKind {
				g.mu.Lock()
				g.ceoPending = append(g.ceoPending, e)
				g.mu.Unlock()
				g.ensureCEO(context.Background(), e)
			}
		case <-g.done:
			return
		}
	}
}

// ---------- 无头席内部唤起（不走 DispatchSeat——Dispatch(role=CEO) 永拒） ----------

// ensureCEO 需 CEO 事件无人接→内部唤起无头化身：渲染 ceo_prompt 模板→
// CEO 壳行候选序试起；连败 3→ceo_last_resort 事件行（D3 呼救径，原文直推
// 飞书属 0013）。provenance=system_event。
func (g *Gateway) ensureCEO(ctx context.Context, e Event) {
	if g.ceoBusy(ctx) {
		return
	}
	rows, err := g.shellsFor(ctx, "CEO")
	if err != nil {
		return
	}
	attempts := route.Candidates(rows, nil, g.cfg.Stagger)
	if len(attempts) == 0 {
		g.ceoSpawnFailed(ctx, e, "无 CEO 壳行")
		return
	}
	pf, err := g.renderCeoPrompt(e)
	if err != nil {
		g.ceoSpawnFailed(ctx, e, "提示模板: "+err.Error())
		return
	}
	for _, at := range attempts {
		if at.Stagger > 0 && !waitOnce(at.Stagger, g.done) {
			return
		}
		if g.spawnCEO(ctx, at, pf) {
			g.mu.Lock()
			g.ceoFails = 0
			g.mu.Unlock()
			return
		}
		g.ceoSpawnFailed(ctx, e, shellLabel(at))
	}
}

// spawnCEO 起一枚 CEO 候选：prompt 文件 + CEO env，attach/秒退窗口同 trySpawn。
func (g *Gateway) spawnCEO(ctx context.Context, at route.Attempt, promptFile string) bool {
	run := g.newRun("", "CEO", at.Shell)
	run.Shell = shellLabel(at)
	run.Provenance = "system_event"
	artDir := filepath.Join(g.cfg.RunDir, "artifacts", run.RunID)
	_ = os.MkdirAll(artDir, 0o700)
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"META_SHELL="+at.Shell,
		"META_MODEL="+at.Model,
		"META_ROLE=CEO",
		"META_PROMPT_FILE="+promptFile,
		"META_WORKDIR="+g.ceoWorkDir(),
		"META_ART_DIR="+artDir,
	)
	argv := g.cfg.SeatArgv
	if len(argv) == 0 {
		argv = []string{g.cfg.SelfPath, "seat"}
	}
	if err := g.startRun(ctx, run, argv, env); err != nil {
		_ = g.database.AppendEvent(ctx, "", "spawn_failed", db.Event{
			Actor: "gateway", Note: "CEO " + shellLabel(at) + ": " + err.Error()})
		return false
	}
	st := g.bindRun(run)
	st.attempt = at
	select {
	case <-st.attachedCh:
	case <-st.exitCh:
		return false
	case <-oneShot(g.cfg.QuickExit, g.done):
	}
	g.emit(Event{Kind: "seat_up", RunID: run.RunID, Role: "CEO"})
	return true
}

func (g *Gateway) ceoWorkDir() string {
	if g.cfg.InjectRoot != "" {
		return g.cfg.InjectRoot
	}
	return g.cfg.RunDir
}

// renderCeoPrompt 模板渲染（{{kind}}/{{ticket_id}}/{{note}}/{{run_dir}}/
// {{gateway_sock}}）→ run/inject/ceo-<rand>.md。模板缺＝fail loud 算唤起失败。
func (g *Gateway) renderCeoPrompt(e Event) (string, error) {
	if g.cfg.CeoPrompt == "" {
		return "", fmt.Errorf("ceo_prompt 未配")
	}
	body, err := os.ReadFile(g.cfg.CeoPrompt)
	if err != nil {
		return "", err
	}
	out := strings.NewReplacer(
		"{{kind}}", e.Kind,
		"{{ticket_id}}", e.TicketID,
		"{{note}}", e.Note,
		"{{run_dir}}", g.cfg.RunDir,
		"{{gateway_sock}}", g.cfg.SocketPath,
	).Replace(string(body))
	dir := filepath.Join(g.cfg.RunDir, "inject")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "ceo-"+randHex(6)+".md")
	return p, os.WriteFile(p, []byte(out), 0o600)
}

// ceoSpawnFailed 连败计数：3→ceo_last_resort 事件行（一次）＋复位计数
// （再 3 败再报——呼救不是一次性沉默）。非壳之错与壳级败同桶：连败=唤起连败。
func (g *Gateway) ceoSpawnFailed(ctx context.Context, e Event, why string) {
	g.mu.Lock()
	g.ceoFails++
	n := g.ceoFails
	if n >= 3 {
		g.ceoFails = 0
	}
	g.mu.Unlock()
	if n < 3 {
		return
	}
	note := "CEO 无头唤起连败 3（最近: " + why + "）原文: " + e.Kind + " " + e.Note
	_ = g.database.AppendEvent(ctx, e.TicketID, "ceo_last_resort", db.Event{
		Actor: "gateway", Note: note})
	g.emit(Event{Kind: "ceo_last_resort", TicketID: e.TicketID, Note: note})
}

// ---------- D-22：CEO run 收口机械比对（事件数 vs 写数） ----------

// ceoGapCheck 判死/退场点机械比对：派发给它的事件数 vs ceo_memory.put
// {event_ref=run_id} 写数；零写入且处理过事件→ceo_gap 行（通知）。
func (g *Gateway) ceoGapCheck(ctx context.Context, runID string) {
	g.mu.Lock()
	dispatched := g.ceoDispatched[runID]
	delete(g.ceoDispatched, runID)
	g.mu.Unlock()
	if dispatched == 0 {
		return
	}
	writes, _ := g.database.CountMemoryByEventRef(ctx, runID)
	if writes > 0 {
		return
	}
	note := fmt.Sprintf("run=%s 处理事件 %d 而 ceo_memory 零写（D-22 收口缺）", runID, dispatched)
	_ = g.database.AppendEvent(ctx, "", "ceo_gap", db.Event{
		Actor: "gateway", Note: note})
	g.emit(Event{Kind: "ceo_gap", RunID: runID, Note: note})
}

// ---------- 五 RPC（经 ceoAPI 薄包装注册——Events 名与 AdminGateway 撞车） ----------

// ceoAPI 把 *Gateway 上的 Ceo* 方法适配成 pbv1.CeoGatewayServer 接口。
type ceoAPI struct {
	pbv1.UnimplementedCeoGatewayServer
	g *Gateway
}

func (a *ceoAPI) Events(s *pbv1.Subscribe, st pbv1.CeoGateway_EventsServer) error {
	return a.g.CeoEvents(s, st)
}
func (a *ceoAPI) SubmitRequirement(ctx context.Context, r *pbv1.Requirement) (*pbv1.TicketRef, error) {
	return a.g.CeoSubmitRequirement(ctx, r)
}
func (a *ceoAPI) Rule(ctx context.Context, r *pbv1.Ruling) (*pbv1.RuleAck, error) {
	return a.g.CeoRule(ctx, r)
}
func (a *ceoAPI) Inquire(ctx context.Context, q *pbv1.Inquiry) (*pbv1.Ack, error) {
	return a.g.CeoInquire(ctx, q)
}
func (a *ceoAPI) Memory(st pbv1.CeoGateway_MemoryServer) error {
	return a.g.CeoMemory(st)
}

// CeoEvents 订阅流＝锁登记点：回放 pending→follow 则随流派发；EOF 除名。
func (g *Gateway) CeoEvents(sub *pbv1.Subscribe, stream pbv1.CeoGateway_EventsServer) error {
	ctx := stream.Context()
	run, err := g.ceoByToken(ctx, sub.RunToken)
	if err != nil {
		return err
	}
	ch, ok := g.registerCEO(run.RunID)
	if !ok {
		return status.Error(codes.FailedPrecondition, "已有活跃 CEO 订阅（锁＝连接登记）")
	}
	defer g.deregisterCEO(run)

	// 回放 pending（切换期间排队不丢，§8）——先 FIFO 回再放行。
	g.mu.Lock()
	pending := append([]Event(nil), g.ceoPending...)
	g.ceoPending = nil
	g.ceoDispatched[run.RunID] += len(pending)
	g.mu.Unlock()
	for _, e := range pending {
		if err := stream.Send(ceoEventItem(e)); err != nil {
			return err
		}
	}
	if !sub.Follow {
		return nil
	}
	for {
		select {
		case e := <-ch:
			if err := stream.Send(ceoEventItem(e)); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-g.done:
			return nil
		}
	}
}

func ceoEventItem(e Event) *pbv1.EventItem {
	return &pbv1.EventItem{
		Kind: e.Kind, TicketId: e.TicketID, Actor: e.RunID, Role: e.Role,
		Note: e.Note, Provenance: e.Provenance, AtUnixNano: time.Now().UnixNano(),
	}
}

// CeoSubmitRequirement 立需求票（§1.4）：spec 部分栏（goal=原文、why=定向+戳）
// ＋requirement 事件（盖 run 行戳——自报字段被盖，对抗性锚②提名行）。
func (g *Gateway) CeoSubmitRequirement(ctx context.Context, r *pbv1.Requirement) (*pbv1.TicketRef, error) {
	run, err := g.ceoByToken(ctx, r.RunToken)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(r.Goal) == "" {
		return nil, status.Error(codes.InvalidArgument, "goal 空")
	}
	prov := run.Provenance // 对抗性锚②：只取 run 行不取请求自报
	title := r.Title
	if title == "" {
		title = preview80(r.Goal)
	}
	// 需求票挂项目（票0015）：落票时校验登记——未登记需求不立。
	if r.Project != "" {
		p, err := g.database.GetProject(ctx, r.Project)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if p == nil {
			return nil, status.Error(codes.FailedPrecondition,
				"项目 "+r.Project+" 无配方登记")
		}
	}
	spec, err := proto.Marshal(&pbv1.TicketSpec{
		Goal: r.Goal,
		Why:  "定向: " + r.Direction + "\nprovenance: " + prov,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	id := "req-" + randHex(6)
	if err := g.database.CreateTicketProv(ctx, id, title, spec, prov, r.Project); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if r.Project != "" {
		g.emitProjectSet(ctx, id, r.Project, run.RunID, prov)
	}
	note := r.Goal
	if r.Direction != "" {
		note += " 定向:" + r.Direction
	}
	_ = g.database.AppendEvent(ctx, id, "requirement", db.Event{
		Actor: run.RunID, Note: note, Provenance: prov})
	g.emit(Event{Kind: "requirement", TicketID: id, RunID: run.RunID,
		Role: "CEO", Note: note, Provenance: prov})
	return &pbv1.TicketRef{Id: id}, nil
}

// CeoRule CEO 裁决三动作（机械闸外全归席）：void（破坏性须 confirm 重述确认
// §3.4）／budget_raise（抬硬顶解冻）／redirect（req_pending 泊车票重派 CTO）。
func (g *Gateway) CeoRule(ctx context.Context, r *pbv1.Ruling) (*pbv1.RuleAck, error) {
	run, err := g.ceoByToken(ctx, r.RunToken)
	if err != nil {
		return nil, err
	}
	ack := func(ok bool, note string) *pbv1.RuleAck {
		return &pbv1.RuleAck{Ok: ok, Note: note}
	}
	fail := func(note string) (*pbv1.RuleAck, error) { return ack(false, note), nil }
	if r.TicketId == "" {
		return fail("ticket_id 空")
	}
	t, err := g.database.GetTicket(ctx, r.TicketId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if t == nil {
		return fail("票不存在")
	}
	note := "action=" + r.Action + " ticket=" + r.TicketId + " " + r.Note
	land := func() (*pbv1.RuleAck, error) {
		_ = g.database.AppendEvent(ctx, r.TicketId, "ruling", db.Event{
			Actor: run.RunID, Note: note, Provenance: run.Provenance})
		g.emit(Event{Kind: "ruling", TicketID: r.TicketId, RunID: run.RunID,
			Role: "CEO", Note: note, Provenance: run.Provenance})
		return ack(true, note), nil
	}
	switch r.Action {
	case "void":
		// 破坏性裁决两步：无 confirm＝回 need_confirm（无 challenge 拒）。
		if !r.Confirm {
			return &pbv1.RuleAck{Ok: false, NeedConfirm: true,
				Note: "作废须重述确认帧（confirm=true）"}, nil
		}
		if err := g.transition(ctx, r.TicketId, db.PhaseVoid, db.EdgeVoid,
			run.RunID, "CEO 令: "+r.Note); err != nil {
			return fail(err.Error())
		}
		return land()
	case "budget_raise":
		if r.HardCap <= 0 {
			return fail("hard_cap 必填正（追加＝抬帽不清零）")
		}
		b, berr := g.database.GetBudget(ctx, r.TicketId)
		if berr != nil {
			return nil, status.Error(codes.Internal, berr.Error())
		}
		if err := g.database.SetBudgetCaps(ctx, r.TicketId, b.Cap, r.HardCap); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if b.UsedTotal < r.HardCap { // 抬帽解冻同 admin.SetBudget 径
			g.mu.Lock()
			delete(g.frozen, r.TicketId)
			g.mu.Unlock()
			_ = g.database.SetBudgetFrozen(ctx, r.TicketId, false)
		}
		return land()
	case "redirect":
		// 泊车专裁：最新事件须 req_pending（对非泊车票重定向＝拒）。
		k, _ := g.database.LatestEventKind(ctx, r.TicketId)
		if t.Phase != db.PhaseCreated || k != string(db.EdgeReqPending) {
			return fail("票 " + r.TicketId + " 非 req_pending 泊车态")
		}
		// 重派的是 **CTO-A**（再设计席）——spec 全栏则走注入机径（全套前缀
		// ＋判决史）；缺栏（需求票未回填）注入机会「缺一不派」，落 CTO-A
		// 再派内联提示。不得用 CEO 无头化身模板（角色指反＝席行为错轨）。
		spec := SeatSpec{WorkDir: g.ceoWorkDir(), Provenance: run.Provenance}
		if !specComplete(t) {
			pf, perr := g.renderRedirectPrompt(r.TicketId, r.Note)
			if perr != nil {
				return nil, status.Error(codes.Internal, perr.Error())
			}
			spec.PromptFile = pf
		}
		if err := g.DispatchSeat(ctx, r.TicketId, "CTO", spec); err != nil {
			return fail(err.Error())
		}
		return land()
	default:
		return fail("未知裁决动作 " + r.Action)
	}
}

// CeoInquire 质询（§8/D7）：起一次 CTO adjunct 诊断实例——只产报告不产票。
func (g *Gateway) CeoInquire(ctx context.Context, q *pbv1.Inquiry) (*pbv1.Ack, error) {
	run, err := g.ceoByToken(ctx, q.RunToken)
	if err != nil {
		return nil, err
	}
	if q.TicketId == "" || strings.TrimSpace(q.Question) == "" {
		return nil, status.Error(codes.InvalidArgument, "ticket_id/question 必填")
	}
	pf, err := g.renderInquirePrompt(q)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := g.DispatchSeat(ctx, q.TicketId, "CTO", SeatSpec{
		PromptFile: pf, WorkDir: g.ceoWorkDir(), Adjunct: true,
		Provenance: run.Provenance}); err != nil {
		return &pbv1.Ack{Ok: false, Note: err.Error()}, nil
	}
	_ = g.database.AppendEvent(ctx, q.TicketId, "inquire", db.Event{
		Actor: run.RunID, Note: q.Question, Provenance: run.Provenance})
	return &pbv1.Ack{Ok: true}, nil
}

func (g *Gateway) renderInquirePrompt(q *pbv1.Inquiry) (string, error) {
	body := "# CTO 诊断席（adjunct · Inquire 唤起 · 票 " + q.TicketId + "）\n\n" +
		"你是 CTO 诊断实例：**只产诊断报告，不产票不派工**。\n\n" +
		"## 质询\n\n" + q.Question + "\n\n" +
		"## 输入\n\n" +
		"- 事件史：`meta events --socket " + g.cfg.SocketPath + " --ticket " + q.TicketId + "`\n" +
		"- run 取证：`meta runs --socket " + g.cfg.SocketPath + " --ticket " + q.TicketId + "`\n" +
		"- 席位日志目录：`" + filepath.Join(g.cfg.RunDir, "logs") + "`\n\n" +
		"## 出口（必须）\n\n写完诊断报告后正常退出即可（adjunct_deliver 回流）。\n"
	return g.writeCeoPromptFile("inquire-", body)
}

// specComplete 票 spec 八栏全否（机械判——缺一不派闸同一把尺）：
// 全则派发走注入机径，缺则调用方须给覆盖提示文件。
func specComplete(t *db.Ticket) bool {
	if len(t.Spec) == 0 {
		return false
	}
	ts := &pbv1.TicketSpec{}
	if err := proto.Unmarshal(t.Spec, ts); err != nil {
		return false
	}
	return inject.CheckSpec(ts) == nil
}

// renderRedirectPrompt CEO redirect 后的 CTO-A 再派提示（需求票缺栏时用——
// 注入机「缺一不派」会挡，故此径自含轨与哨兵约定，防空提示误派）。
func (g *Gateway) renderRedirectPrompt(ticketID, note string) (string, error) {
	body := "# CTO-A 再派（CEO redirect · 需求票 " + ticketID + "）\n\n" +
		"你是 **CTO-A**：需求→设计文档→拆票→派发（角色卡：roles/CTO.md）。\n" +
		"本票先前因「需求待裁」泊车（req_pending），CEO 已重定向——按新定向续作。\n\n" +
		"## CEO 新定向\n\n" + note + "\n\n" +
		"## 轨（设计-0007 §1.4 钦定）\n\n" +
		"1. 读票：`meta events --socket " + g.cfg.SocketPath + " --ticket " + ticketID + "`\n" +
		"   取 created/requirement/req_pending/ruling 行——goal=需求原文、定向与新定向在行内。\n" +
		"2. 产出设计文档并 commit 到仓。\n" +
		"3. 回填本票八栏：`META_ADMIN:fill_spec|<本票八栏 .md 路径>`（只许本票）。\n" +
		"4. 立子票：`META_ADMIN:create_ticket|<id>|<title>|<子票八栏 .md 路径>[|<project>]`（第 5 段可选；空＝继承本票 project）。\n" +
		"5. 派子票（role∈{施工,审计,CTO}，CEO 禁派闸挡）：`META_ADMIN:dispatch|<id>|<role>|[promptfile~workdir]`\n" +
		"   本票漏挂项目时自补：`META_ADMIN:set_project|<project>`（只许本票施工中；项目须已登记）。\n" +
		"6. 完工交付：末行单独输出 `META_DELIVER:<交付 ref>`。\n\n" +
		"## 出口\n\n仍属「需求待裁」（动钱/范围外/票面歧义）→ `Yield` 再泊车等 CEO；\n" +
		"**不要选一种解释硬做**。\n"
	return g.writeCeoPromptFile("redirect-", body)
}

// writeCeoPromptFile CEO 面内联提示落盘共用径（run/inject/<前缀><rand>.md）。
func (g *Gateway) writeCeoPromptFile(prefix, body string) (string, error) {
	dir := filepath.Join(g.cfg.RunDir, "inject")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, prefix+randHex(6)+".md")
	return p, os.WriteFile(p, []byte(body), 0o600)
}

// CeoMemory ceo_memory 流读写：每帧带 run_token（流内复验）；put 落
// event_ref=run_id＋run 行戳；kind=say→ceo_say 事件（不增第六 RPC，§1.9）。
func (g *Gateway) CeoMemory(stream pbv1.CeoGateway_MemoryServer) error {
	ctx := stream.Context()
	for {
		op, err := stream.Recv()
		if err != nil {
			return err // EOF 亦同径退
		}
		run, err := g.ceoByToken(ctx, op.RunToken)
		if err != nil {
			return err
		}
		switch b := op.GetOp().(type) {
		case *pbv1.MemoryOp_Put:
			if b.Put.Kind == "" {
				return status.Error(codes.InvalidArgument, "put.kind 空")
			}
			id, err := g.database.PutMemory(ctx, b.Put.Kind, b.Put.Title, b.Put.Content,
				run.RunID, run.Provenance)
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}
			if b.Put.Kind == "say" {
				_ = g.database.AppendEvent(ctx, "", "ceo_say", db.Event{
					Actor: run.RunID, Note: b.Put.Title, Provenance: run.Provenance})
				g.emit(Event{Kind: "ceo_say", RunID: run.RunID, Role: "CEO",
					Note: string(b.Put.Content), Provenance: run.Provenance})
			}
			if err := stream.Send(&pbv1.MemoryOp{Op: &pbv1.MemoryOp_Row{
				Row: &pbv1.MemoryRow{Id: id, Kind: b.Put.Kind, Title: b.Put.Title,
					Content: b.Put.Content, EventRef: run.RunID, Provenance: run.Provenance,
					AtUnixNano: time.Now().UnixNano()}}}); err != nil {
				return err
			}
		case *pbv1.MemoryOp_Get:
			rows, err := g.database.ListMemory(ctx, b.Get.Kind, int(b.Get.Limit))
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}
			for _, m := range rows {
				if err := stream.Send(&pbv1.MemoryOp{Op: &pbv1.MemoryOp_Row{
					Row: &pbv1.MemoryRow{Id: m.ID, Kind: m.Kind, Title: m.Title,
						Content: m.Content, EventRef: m.EventRef, Provenance: m.Provenance,
						AtUnixNano: m.CreatedAt.UnixNano()}}}); err != nil {
					return err
				}
			}
		default:
			return status.Error(codes.InvalidArgument, "空 MemoryOp")
		}
	}
}

// ---------- 席位流 admin 帧（§1.5：写径随席位流，D-24 不阻碍条款落点） ----------

// handleAdmin admin 帧机械闸：**role==CTO 且非 adjunct**（对抗性锚④提名行——
// 删本闸施工席即可立票）。动作三件：fill_spec（限本票施工中）/create_ticket
// /dispatch；子票与 spawn 沿用本 run 的 provenance 戳。
func (g *Gateway) handleAdmin(ctx context.Context, st *runState, stream pbv1.SeatGateway_AttachServer, a *pbv1.SeatAdmin) error {
	if st.run.Role != "CTO" || st.run.Adjunct {
		return status.Error(codes.PermissionDenied,
			fmt.Sprintf("admin 帧仅非 adjunct CTO 席可发（role=%s adjunct=%v）", st.run.Role, st.run.Adjunct))
	}
	resp := func(ok bool, note string) error {
		return stream.Send(&pbv1.GatewayMsg{Body: &pbv1.GatewayMsg_AdminResp{
			AdminResp: &pbv1.Ack{Ok: ok, Note: note}}})
	}
	switch act := a.GetAction().(type) {
	case *pbv1.SeatAdmin_FillSpec:
		if act.FillSpec.TicketId != st.run.TicketID {
			return resp(false, "fill_spec 越票拒：只允许回填派发票 "+st.run.TicketID)
		}
		phase, err := g.database.PhaseOf(ctx, st.run.TicketID)
		if err != nil || phase != db.PhaseWorking {
			return resp(false, "fill_spec 限本票施工中（相位 "+string(phase)+"）")
		}
		ts, err := inject.ParseTicketMD(act.FillSpec.SpecMd)
		if err != nil {
			return resp(false, "spec 解析/缺栏: "+err.Error())
		}
		blob, err := proto.Marshal(ts)
		if err != nil {
			return resp(false, err.Error())
		}
		if err := g.database.UpdateSpec(ctx, st.run.TicketID, blob); err != nil {
			return resp(false, err.Error())
		}
		_ = g.database.AppendEvent(ctx, st.run.TicketID, "fill_spec", db.Event{
			Actor: st.run.RunID, Note: "八栏回填", Provenance: st.run.Provenance})
		return resp(true, "spec 已回填")
	case *pbv1.SeatAdmin_CreateTicket:
		nt := act.CreateTicket
		if nt.Id == "" {
			return resp(false, "create_ticket id 空")
		}
		// 子票挂项目（票0015）：空则继承本票 project（派发票已挂的子票天然
		// 同项目，登记在挂接时已验不重复查）；非空→校验登记，未登记票不立。
		project := nt.Project
		if project == "" {
			if pt, err := g.database.GetTicket(ctx, st.run.TicketID); err == nil && pt != nil {
				project = pt.Project
			}
		} else if p, err := g.database.GetProject(ctx, project); err != nil {
			return resp(false, err.Error())
		} else if p == nil {
			return resp(false, "项目 "+project+" 无配方登记——票不立")
		}
		// 子票 created 行同戳（§1.3：戳沿链不沿自报）。
		if err := g.database.CreateTicketProv(ctx, nt.Id, nt.Title, nt.Spec, st.run.Provenance, project); err != nil {
			return resp(false, err.Error())
		}
		if project != "" {
			g.emitProjectSet(ctx, nt.Id, project, st.run.RunID, st.run.Provenance)
		}
		return resp(true, "ticket "+nt.Id+" created")
	case *pbv1.SeatAdmin_SetProject:
		sp := act.SetProject
		// 对抗性锚提名行（锚 d）：越票判——删则席位改写他票 project。
		if sp.TicketId != st.run.TicketID {
			return resp(false, "set_project 越票拒：只允许挂接本票 "+st.run.TicketID)
		}
		phase, err := g.database.PhaseOf(ctx, st.run.TicketID)
		if err != nil || phase != db.PhaseWorking {
			return resp(false, "set_project 限本票施工中（相位 "+string(phase)+"）")
		}
		if err := g.attachProject(ctx, st.run.TicketID, sp.Project, st.run.RunID, st.run.Provenance); err != nil {
			return resp(false, err.Error())
		}
		return resp(true, "project "+sp.Project+" set")
	case *pbv1.SeatAdmin_Dispatch:
		d := act.Dispatch
		err := g.DispatchSeat(ctx, d.TicketId, d.Role, SeatSpec{
			PromptFile: d.PromptFile, WorkDir: d.WorkDir,
			PreferShell: d.PreferShell, Adjunct: d.Adjunct,
			Provenance: st.run.Provenance, // spawn 沿用本 run 戳
			Project:    d.Project,         // 派发径挂接（哨兵不出此段，留口）
		})
		if err != nil {
			return resp(false, err.Error())
		}
		return resp(true, "dispatched "+d.TicketId+" "+d.Role)
	default:
		return resp(false, "空 admin 帧")
	}
}

// ---------- 终态清理（D-15）与设计锁释放（B1） ----------

// onTerminal 票入终态（完成/作废）即删进程级内存键——specs/failSet/warned/
// frozen 四表 + 未决闹钟；终态票不该再占内存也不该有幽灵事件。
func (g *Gateway) onTerminal(ticketID string) {
	g.mu.Lock()
	for k := range g.specs {
		if k.ticket == ticketID {
			delete(g.specs, k)
		}
	}
	delete(g.failSet, ticketID)
	delete(g.warned, ticketID)
	delete(g.frozen, ticketID)
	g.mu.Unlock()
	g.cancelAlarm(ticketID)
}

// releaseDesignLock 持锁票离开施工中/入终态→放项目设计锁；锁释放事件→查
// 同项目已立票→补发 spawn_needed:CTO（事件驱动解锁，零定时，§1.8）。
func (g *Gateway) releaseDesignLock(ctx context.Context, ticketID string) {
	t, err := g.database.GetTicket(ctx, ticketID)
	if err != nil || t == nil || t.Project == "" {
		return
	}
	g.mu.Lock()
	if g.designLocks[t.Project] != ticketID {
		g.mu.Unlock()
		return
	}
	delete(g.designLocks, t.Project)
	g.mu.Unlock()
	ids, err := g.database.TicketsByProjectPhaseIDs(ctx, t.Project, db.PhaseCreated)
	if err != nil {
		return
	}
	for _, id := range ids {
		if k, _ := g.database.LatestEventKind(ctx, id); k == string(db.EdgeReqPending) {
			continue // 泊车票同抑制
		}
		_ = g.database.AppendEvent(ctx, id, "spawn_needed", db.Event{
			Actor: "gateway", Note: "对账补发: CTO（设计锁释放）"})
		g.emit(Event{Kind: "spawn_needed", TicketID: id, Role: "CTO"})
	}
}

func preview80(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		return s[:80]
	}
	return s
}

// onRunDead 的 CEO 支在 attach.go 调用本函数（D-22 判死点）。
func (g *Gateway) ceoDeadHook(ctx context.Context, st *runState) {
	g.ceoGapCheck(ctx, st.run.RunID)
	// CEO run 死亡后若仍有 pending，下个 spawn_kind 事件自然再唤起——
	// 此处不主动重唤起（无新需求，旧事件已在队列）。
}
