package gateway

import (
	"context"
	"encoding/json"
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
	ad, err := adapter.New(g.cfg.Filter.Name)
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
		return nil, fmt.Errorf("过滤壳 %s: %w", g.cfg.Filter.Name, err)
	}
	keep, ok := parseKeepAll(string(out), len(evs))
	if !ok {
		// JSONL 壳兜底（实证 opencode --format json）：裸文本判不出→
		// 逐行 JSON 解字符串字段重扫——KEEP/DROP 藏在助手文本载荷里。
		keep, ok = parseKeepAll(jsonlText(string(out)), len(evs))
	}
	if !ok {
		return nil, fmt.Errorf("过滤壳输出不可判（无 KEEP/DROP 行）")
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

// jsonlText JSONL 行集→文本载荷：每行按 JSON 对象解码、递归抽全部字符串
// 值拼接（opencode --format json / claude stream-json 的助手文本都在
// 字符串字段里——裸扫描判不出 KEEP/DROP 行首）。
func jsonlText(out string) string {
	var b strings.Builder
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			b.WriteString(t)
			b.WriteByte('\n')
		case map[string]any:
			for _, x := range t {
				walk(x)
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "{") {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(ln), &v); err == nil {
			walk(v)
		}
	}
	return b.String()
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
