package gateway

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"meta/internal/adapter"
	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// newAdapter 壳适配器构造缝（包级——测试可换伪壳走通 cheapFilter 全径，
// inject.go 的 assemble 同款先例）。
var newAdapter = adapter.New

// 巡检席三件（票0030）：订阅匹配（emit 末位钩）→批收→便宜壳两级过滤→
// 信号路由（带票→adjunct 巡检席；无票→巡检票归并/立案＋正式派发）。

// patrolFeed emit 末位钩：kind 命中订阅→批收道（非阻塞——满则丢＋
// patrol_drop 事件行；patrol_drop 自身不入种行零递归）。subs 启动装载后
// 只读，直读无锁。
func (g *Gateway) patrolFeed(e Event) {
	if len(g.subs[e.Kind]) == 0 {
		return
	}
	select {
	case g.patrolCh <- e:
	default:
		_ = g.database.AppendEvent(context.Background(), e.TicketID, "patrol_drop", db.Event{
			Actor: "gateway", Note: "巡检批收道满丢: " + e.Kind})
	}
}

// patrolPump 批收环：首事件挂一次性窗闹钟（事件驱动零周期），批满
// BatchMax 提前冲；冲出的批过两级过滤后逐信号路由。
func (g *Gateway) patrolPump() {
	var batch []Event
	armed := false
	flush := func() {
		evs := batch
		batch, armed = nil, false
		if len(evs) > 0 {
			g.patrolHandle(evs)
		}
	}
	for {
		select {
		case e := <-g.patrolCh:
			batch = append(batch, e)
			if len(batch) >= g.cfg.Filter.BatchMax {
				flush()
			} else if !armed {
				armed = true
				time.AfterFunc(g.cfg.Filter.BatchWindow, func() {
					select {
					case g.patrolWake <- struct{}{}:
					default:
					}
				})
			}
		case <-g.patrolWake:
			flush()
		case <-g.done:
			return
		}
	}
}

// patrolHandle 两级过滤＋派发：Filter.Name 空＝全当信号；过滤壳调用败/
// 输出不可判＝放行全开＋filter_degraded 事件（信号可用性优先——省 token
// 是收益不是闸，闸错埋真信号更贵）。
func (g *Gateway) patrolHandle(evs []Event) {
	ctx := context.Background()
	signals := evs
	if g.cfg.Filter.Name != "" {
		keep, err := g.cheapFilter(ctx, evs)
		if err != nil {
			_ = g.database.AppendEvent(ctx, "", "filter_degraded", db.Event{
				Actor: "gateway", Note: err.Error()})
			g.emit(Event{Kind: "filter_degraded", Note: err.Error()})
		} else {
			signals = keep
		}
	}
	for _, e := range signals {
		for _, role := range g.subs[e.Kind] {
			g.routeSignal(ctx, e, role)
		}
	}
}

// cheapFilter 便宜壳一级筛：编号事件清单写提示文件→adapter 造 argv→
// 限时子进程→扫 stdout KEEP/DROP 行。
func (g *Gateway) cheapFilter(ctx context.Context, evs []Event) ([]Event, error) {
	var b strings.Builder
	b.WriteString("你是事件过滤层。逐条判定下列编号事件——真信号＝需要席位介入" +
		"处置的异常/失败/阻塞/驳回；噪声＝常态活性/信息/重复行。" +
		"每条只答一行 `KEEP <编号>` 或 `DROP <编号>`，编号未答按 DROP 计。\n\n")
	for i, e := range evs {
		fmt.Fprintf(&b, "#%d kind=%s ticket=%s role=%s note=%s\n",
			i+1, e.Kind, e.TicketID, e.Role, e.Note)
	}
	pf, err := g.patrolFile(fmt.Sprintf("filter-%d.md", time.Now().UnixNano()), b.String())
	if err != nil {
		return nil, err
	}
	ad, err := newAdapter(g.cfg.Filter.Name)
	if err != nil {
		return nil, err
	}
	c, err := ad.Build(adapter.Spec{
		Shell: g.cfg.Filter.Name, Model: g.cfg.Filter.Model,
		Role: "过滤", PromptFile: pf, WorkDir: g.cfg.RunDir,
	})
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, g.cfg.Filter.Timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, c.Argv[0], c.Argv[1:]...)
	if c.Dir != "" {
		cmd.Dir = c.Dir
	}
	cmd.Env = append(os.Environ(), c.Env...)
	if c.StdinFile != "" {
		f, err := os.Open(c.StdinFile)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		cmd.Stdin = f
	}
	out, err := cmd.Output()
	if err != nil {
		// 死因分类（票0034）：裸 `signal: killed` 盖超时杀/限额退壳/起壳
		// 败一个样＝下次坏照样瞎（11-02 实证事件 2638）——note 即诊断面。
		return nil, fmt.Errorf("过滤壳 %s: %s", g.cfg.Filter.Name,
			classifyFilterExit(cctx, err, out))
	}
	keep, ok := parseKeepAll(string(out), len(evs))
	if !ok {
		// JSONL 壳兜底（实证 opencode --format json）：裸文本判不出→
		// 逐行 JSON 解字符串字段重扫——KEEP/DROP 藏在助手文本载荷里。
		keep, ok = parseKeepAll(adapter.JSONLText(string(out)), len(evs))
	}
	if !ok {
		if quotaLike(string(out)) {
			return nil, fmt.Errorf("过滤壳限额打穿（输出不可判＋限额特征） tail=%s",
				outTail(string(out), 300))
		}
		return nil, fmt.Errorf("过滤壳输出不可判（无 KEEP/DROP 行） tail=%s",
			outTail(string(out), 200))
	}
	var sig []Event
	for i, e := range evs {
		if keep[i+1] {
			sig = append(sig, e)
		}
	}
	return sig, nil
}

// parseKeepAll 过滤壳输出判读（纯函数，测试锚）：`KEEP <n>`/`DROP <n>`
// 行集；编号也可用逗号列（KEEP: 1,3）。未答编号按 DROP（保守放行口径：
// 便宜层的职责是挡噪声）。一行有效判定都没有→ok=false（调用方放行全开）。
func parseKeepAll(out string, n int) (map[int]bool, bool) {
	keep := map[int]bool{}
	seen := false
	for _, ln := range strings.Split(out, "\n") {
		f := strings.Fields(strings.TrimSpace(ln))
		if len(f) < 2 {
			continue
		}
		verb := strings.ToUpper(strings.TrimSuffix(f[0], ":"))
		if verb != "KEEP" && verb != "DROP" {
			continue
		}
		for _, tok := range f[1:] {
			for _, p := range strings.Split(tok, ",") {
				i, err := strconv.Atoi(strings.TrimSpace(p))
				if err != nil || i < 1 || i > n {
					continue
				}
				seen = true
				if verb == "KEEP" {
					keep[i] = true
				}
			}
		}
	}
	return keep, seen
}

// quotaSigs 限额签名集（小写子串匹配——票0034）：壳/上游 API 限额打穿时的
// 高频特征词。启发式（设计§5 诚实边界）：假阳最坏＝分类词偏差（壳败放行
// 全开语义不变，信号一条不丢）；假阴＝落「退码非零」仍强于裸 killed。
var quotaSigs = []string{
	"rate limit", "429", "quota", "insufficient_quota",
	"daily limit", "limit exceeded", "限额",
}

// quotaLike 输出/错误尾带限额特征判（纯函数，测试锚）。
func quotaLike(s string) bool {
	s = strings.ToLower(s)
	for _, sig := range quotaSigs {
		if strings.Contains(s, sig) {
			return true
		}
	}
	return false
}

// outTail 输出尾摘要（logTail 同款形制，票0034）：末 n 字节→clean 净化
// （决策 67 毒字节进事件流瘫 marshal 先例）→换行并竖线→TrimSpace——
// 进 filter_degraded note 即安全。
func outTail(s string, n int) string {
	b := []byte(s)
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return strings.TrimSpace(strings.ReplaceAll(clean(string(b)), "\n", " | "))
}

// classifyFilterExit 过滤壳死因分类（纯函数，测试锚）：超时杀/限额打穿/
// 退码非零/起壳败四桶。cmd.Output 的错误原文（`signal: killed` 等）不可分
// 死因——分类词＋净化证据尾进 filter_degraded note，下次坏可判。
func classifyFilterExit(cctx context.Context, err error, out []byte) string {
	// CommandContext 超时杀的钦定特征：壳进程被杀时 ctx 已截止。
	if cctx.Err() == context.DeadlineExceeded {
		return "超时（ctx 截止被杀——CommandContext 限时杀）"
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		stderr := string(ee.Stderr)
		if quotaLike(stderr) || quotaLike(string(out)) {
			tail := outTail(stderr, 300)
			if tail == "" {
				tail = outTail(string(out), 300)
			}
			return fmt.Sprintf("限额打穿（限额特征命中） exit=%d tail=%s",
				ee.ExitCode(), tail)
		}
		return fmt.Sprintf("退码非零 exit=%d tail=%s", ee.ExitCode(),
			outTail(stderr, 300))
	}
	// Start 败/二进制缺/权限——壳根本没跑起来。
	return "起壳败: " + err.Error()
}

// routeSignal 信号→席位：带票事件→adjunct 巡检席（同票同角色去重闸天然
// 防风暴；票终态/冻票 DispatchSeat 自拒落 patrol_skip 行）；无票事件
// （patrol_tick 等）→巡检票归并/立案＋正式派发（步行边全管线）。
func (g *Gateway) routeSignal(ctx context.Context, e Event, role string) {
	tid := e.TicketID
	if tid == "" {
		tid = g.patrolTicketFor(ctx)
		if tid == "" {
			return
		}
		// 信号原文挂票内留言——巡检席注入前缀经 ticket_msg 带走。
		_, _ = g.database.AppendMsg(ctx, tid, "patrol:"+e.Kind, e.Note)
		if err := g.DispatchSeat(ctx, tid, role, SeatSpec{Provenance: "system_event"}); err != nil {
			_ = g.database.AppendEvent(ctx, tid, "patrol_skip", db.Event{
				Actor: "gateway", Note: err.Error()})
		}
		return
	}
	pf, err := g.patrolPromptFile(tid, e)
	if err != nil {
		_ = g.database.AppendEvent(ctx, tid, "patrol_skip", db.Event{
			Actor: "gateway", Note: "巡检提示件: " + err.Error()})
		return
	}
	if err := g.DispatchSeat(ctx, tid, role, SeatSpec{
		Adjunct: true, PromptFile: pf, Provenance: "system_event"}); err != nil {
		_ = g.database.AppendEvent(ctx, tid, "patrol_skip", db.Event{
			Actor: "gateway", Note: err.Error()})
		g.emit(Event{Kind: "patrol_skip", TicketID: tid, Role: role, Note: err.Error()})
	}
}

// patrolTicketFor 无票信号落点：有非终态 patrol-* 票归并（心跳不积票），
// 无则立 patrol-<unix> 值班票（内置八栏 spec 过缺一不派闸）。
func (g *Gateway) patrolTicketFor(ctx context.Context) string {
	if ts, err := g.database.NonTerminalTickets(ctx); err == nil {
		for _, t := range ts {
			if strings.HasPrefix(t.ID, "patrol-") {
				return t.ID
			}
		}
	}
	spec := &pbv1.TicketSpec{
		Goal:       "巡检事件处置：调查订阅命中的异常信号，真问题用 meta ticket new 自立案，交付诊断报告",
		Why:        "巡检订阅命中无归属票的信号（patrol_tick 心跳/票外异常）——本票是其载体与归并点",
		Scope:      "读信号→meta events/runs/phase 查证→真问题立案→报告；多信号同票归并处理",
		OutOfScope: "不改业务代码、不部署、不并 master、不替 CEO 裁决；诊断外动作一律立案移交",
		Refs:       "票0030 巡检席设计-0030；subscription 表；CLI socket＝$META_GATEWAY_SOCK",
		Anchors:    "每条信号有判定与去向（立案票号/噪声理由）；漏报信号＝瞒报同罪",
		Verify:     "报告含信号清单＋逐条判定＋已立票号列",
		Deliver:    "交付摘要列信号处置表；META_DELIVER:<ref> 哨兵照旧",
	}
	blob, err := proto.Marshal(spec)
	if err != nil {
		return ""
	}
	id := fmt.Sprintf("patrol-%d", time.Now().Unix())
	if err := g.database.CreateTicketProv(ctx, id,
		"巡检席值班票", blob, "system_event", ""); err != nil {
		return ""
	}
	return id
}

// patrolPromptFile adjunct 巡检提示件（含诊断与立案径：CLI 经
// $META_GATEWAY_SOCK——席位 env 已带；META_ADMIN 闸不动）。
func (g *Gateway) patrolPromptFile(ticketID string, e Event) (string, error) {
	body := fmt.Sprintf(`# 巡检信号（adjunct 诊断席——不占相位、不重派）

你是**巡检席**。命中订阅的事件：

- kind: %s
- ticket: %s
- role: %s
- note: %s
- provenance: %s

## 动作规程

1. 诊断：meta events --socket "$META_GATEWAY_SOCK" --ticket %s、
   meta runs --ticket %s、meta phase --ticket %s 查证是否真异常。
2. 真信号→自立案：meta ticket new --socket "$META_GATEWAY_SOCK" 立票
   （id 建议 %s-pN，票面八栏照 docs/实现/ 既有票形制）。
3. 噪声→不动作，报告里如实记判定理由。
4. 收尾单独一行哨兵 META_DELIVER:patrol-<描述>（含 sentinel.log 同行）。
`, e.Kind, e.TicketID, e.Role, e.Note, e.Provenance, ticketID, ticketID, ticketID, ticketID)
	return g.patrolFile(fmt.Sprintf("patrol-%s-%s-%d.md", ticketID, e.Kind,
		time.Now().UnixNano()), body)
}

// patrolFile 巡检提示件落盘共用径（run/inject/ 下，0600）。
func (g *Gateway) patrolFile(name, body string) (string, error) {
	dir := filepath.Join(g.cfg.RunDir, "inject")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	pf := filepath.Join(dir, name)
	if err := os.WriteFile(pf, []byte(body), 0o600); err != nil {
		return "", err
	}
	return pf, nil
}

// patrolDutyPromptFile patrol-* 值班兜底提示件（票0034）：注入机缺
// （InjectRoot 空）或组装败时的最简值班提示——巡检是「机器坏时仍能报信」
// 的兜底径，不能依赖被它监视的设施（11-02 实证：unit 缺 --inject-root 一次，
// 值班票 100% 撞 D-40 空提示全灭）。占相位正经席口径（非 adjunct——文案
// 不写「不占相位」）；比注入前缀薄（无角色卡/配方/两库索引），八栏内嵌＋
// 留言＋轨＋哨兵契约在，本职「判信号→立案→报告」可做。
func (g *Gateway) patrolDutyPromptFile(ctx context.Context, ticketID, role string, t *db.Ticket) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "# 巡检席值班票（票 %s：%s）\n\n", ticketID, t.Title)
	fmt.Fprintf(&b, "你是**巡检席**（role=%s，占相位的正经席位）。本提示为注入机不可用时的"+
		"兜底件，比全量注入前缀薄——按下列票面与轨执行本职。\n\n", role)

	b.WriteString("## 票面八栏\n\n")
	ts := &pbv1.TicketSpec{}
	if len(t.Spec) > 0 && proto.Unmarshal(t.Spec, ts) == nil {
		for _, col := range []struct{ name, val string }{
			{"目标", ts.Goal}, {"为什么", ts.Why}, {"范围内", ts.Scope},
			{"范围外（禁线）", ts.OutOfScope}, {"设计引用", ts.Refs},
			{"验收锚", ts.Anchors}, {"验证命令", ts.Verify}, {"交付形制", ts.Deliver},
		} {
			fmt.Fprintf(&b, "- **%s**：%s\n", col.name, col.val)
		}
	} else {
		b.WriteString("（spec 反序列化败/空——完整票面读 DB：meta ticket）\n")
	}

	if ms, err := g.database.MsgsForTicket(ctx, ticketID, false); err == nil && len(ms) > 0 {
		b.WriteString("\n## 未投递留言（信号原文——必读）\n\n")
		for _, m := range ms {
			fmt.Fprintf(&b, "- [%s] %s（%s）\n", m.Actor, m.Text,
				m.CreatedAt.Format("01-02 15:04"))
		}
	}

	b.WriteString("\n## 动作轨\n\n")
	fmt.Fprintf(&b, "1. 诊断：meta events / runs / phase --socket \"$META_GATEWAY_SOCK\" --ticket %s "+
		"逐条查证留言中的信号。\n", ticketID)
	fmt.Fprintf(&b, "2. 真问题→自立案：meta ticket new --socket \"$META_GATEWAY_SOCK\"（id 建议 %s-pN，"+
		"票面八栏照 docs/实现/ 既有票形制）。\n", ticketID)
	b.WriteString("3. 报告：每条信号有判定与去向（立案票号/噪声理由）——漏报信号＝瞒报同罪。\n")

	b.WriteString("\n## 交付契约\n\n" +
		"完工交付摘要后，stdout 最后一行单独一行哨兵，并把同一行原文追加到\n" +
		"$META_ART_DIR/sentinel.log（双发——缺任一＝半交付）：\n\n" +
		"META_DELIVER:<ref>\n")
	return g.patrolFile(fmt.Sprintf("duty-%s-%s-%d.md", ticketID, role,
		time.Now().UnixNano()), b.String())
}
