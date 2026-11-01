// Package caster 播报员引擎（票0004 §3）：AdminGateway 客户端——
// 以 ticket_event 为唯一真源做水位游标回读（断线补报＝同码径），
// 按 textproto 路由表把事件机械映射到「通知／Dispatch」两类动作。
// 引擎无定时器：wake＝Observe 流/进程启动；流断续订退避＝传输层
// 重试（R2 边界条款合法）。判断永不进播报员。
package caster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/prototext"

	pbv1 "meta/internal/pb/metav1"
)

// AdminClient 引擎对管理面的最小依赖（伪实现注入测试径）。
type AdminClient interface {
	EventsAfter(ctx context.Context, sinceID int64, limit int32) (*pbv1.EventList, error)
	Dispatch(ctx context.Context, req *pbv1.DispatchReq) (*pbv1.Ack, error)
	Observe(ctx context.Context) (pbv1.AdminGateway_ObserveClient, error)
}

// Config 引擎参数（meta caster 子命令旗标面）。
type Config struct {
	RoutesPath    string // 路由表 textproto（配置根＝其父目录，模板相对此解）
	StateDir      string // 水位线与渲染产物落点（watermark/prompt-*.md）
	RunDir        string // {{run_dir}} 占位符（席位日志根）
	SocketPath    string // {{gateway_sock}} 占位符
	NotifyCmd     string // 通知脚本（exec 单参数＝渲染后文本）
	ReplayHistory bool   // 水位文件缺失时追补历史（缺省＝快进只推位）
	Dial          func(ctx context.Context) (AdminClient, error)
	// Notify 可注入（测试捕获）；缺省 exec NotifyCmd。
	Notify func(ctx context.Context, text string) error
	// Logf 引擎自报（stderr/journal）。
	Logf func(format string, args ...any)
}

// Engine 引擎本体。
type Engine struct {
	cfg         Config
	table       *pbv1.RouteTable
	configRoot  string
	wmPath      string
	wm          int64
	fastForward bool // 水位文件缺失首跑：只推位不动作（§4 快进模式）
}

// LoadTable 表加载校验：prototext 解析＋机械判——
// 空 kind / 空动作体 / dispatch 空 role 一律拒载（宁缺勿滥）。
func LoadTable(path string) (*pbv1.RouteTable, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("路由表 %s: %w", path, err)
	}
	t := &pbv1.RouteTable{}
	if err := prototext.Unmarshal(b, t); err != nil {
		return nil, fmt.Errorf("路由表解析 %s: %w", path, err)
	}
	for i, r := range t.Routes {
		if r.Kind == "" {
			return nil, fmt.Errorf("路由表 %s 第%d行：kind 空", path, i)
		}
		for j, a := range r.Actions {
			switch b := a.GetBody().(type) {
			case *pbv1.Action_Notify:
			case *pbv1.Action_Dispatch:
				if b.Dispatch.Role == "" {
					return nil, fmt.Errorf("路由表 %s 第%d行第%d动作：dispatch.role 空", path, i, j)
				}
			default:
				return nil, fmt.Errorf("路由表 %s 第%d行第%d动作：未知动作体", path, i, j)
			}
		}
	}
	return t, nil
}

// New 建引擎：载表＋定位水位（缺失＝快进模式，除非 --replay-history）。
func New(cfg Config) (*Engine, error) {
	if cfg.Dial == nil {
		return nil, fmt.Errorf("caster: Dial 空")
	}
	table, err := LoadTable(cfg.RoutesPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.StateDir, 0o700); err != nil {
		return nil, err
	}
	e := &Engine{cfg: cfg, table: table,
		configRoot: filepath.Dir(cfg.RoutesPath),
		wmPath:     filepath.Join(cfg.StateDir, "watermark"),
	}
	if cfg.Logf == nil {
		e.cfg.Logf = func(f string, a ...any) { fmt.Fprintf(os.Stderr, "caster: "+f+"\n", a...) }
	}
	if cfg.Notify == nil {
		e.cfg.Notify = func(ctx context.Context, text string) error {
			return exec.CommandContext(ctx, cfg.NotifyCmd, text).Run()
		}
	}
	if b, err := os.ReadFile(e.wmPath); err == nil {
		_, _ = fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &e.wm)
	} else if !cfg.ReplayHistory {
		e.fastForward = true
	}
	return e, nil
}

// writeWM 水位原子写（tmp+rename；处理完才推——at-least-once，§4）。
// 对抗性锚提名行：删本调用＝事件重复处理。
func (e *Engine) writeWM() error {
	tmp := e.wmPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(fmt.Sprintf("%d\n", e.wm)), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, e.wmPath)
}

// Once 连接＋resync 一次即退（--once 测试与手动追平径）。
func (e *Engine) Once(ctx context.Context) error {
	cli, err := e.cfg.Dial(ctx)
	if err != nil {
		return err
	}
	return e.resync(ctx, cli)
}

// Run 常驻主循环：connect→Observe（先订阅再对账，缝闭两侧）→resync→
// 每收 ObsEvent 再 resync；流断退避重连（1s→60s cap，无界＝传输层续订
// 语义），重连成功即 resync（断线补报同码径，§3/§4）。
func (e *Engine) Run(ctx context.Context) error {
	backoff := time.Second
	for {
		err := e.cycle(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		e.cfg.Logf("流断 %v——%v 后续订重连", err, backoff)
		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
		if backoff < 60*time.Second {
			backoff *= 2
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
		}
	}
}

// cycle 一轮连接生命周期；返回流/连接错误由外层退避续订。
func (e *Engine) cycle(ctx context.Context) error {
	cli, err := e.cfg.Dial(ctx)
	if err != nil {
		return err
	}
	obs, err := cli.Observe(ctx)
	if err != nil {
		return err
	}
	if err := e.resync(ctx, cli); err != nil {
		return err
	}
	for {
		if _, err := obs.Recv(); err != nil {
			return err
		}
		if err := e.resync(ctx, cli); err != nil {
			return err
		}
	}
}

// resync 分页回读至短页：逐行 process(row)→每行处理完原子推 wm。
// 快进模式（水位文件缺失首跑）只推位不动作——不吃历史剩饭（§4）。
func (e *Engine) resync(ctx context.Context, cli AdminClient) error {
	for {
		list, err := cli.EventsAfter(ctx, e.wm, 500)
		if err != nil {
			return err
		}
		for _, row := range list.Items {
			if !e.fastForward {
				e.process(ctx, cli, row)
			}
			if row.Id > e.wm {
				e.wm = row.Id
				if err := e.writeWM(); err != nil {
					e.cfg.Logf("水位写失败: %v", err)
				}
			}
		}
		if len(list.Items) < 500 {
			e.fastForward = false
			return nil
		}
	}
}

// process 一行事件→路由匹配→串行动作。精确等值匹配——
// 对抗性锚提名行：route.Kind==row.Kind 翻成 != 则表外 kind 也派单。
func (e *Engine) process(ctx context.Context, cli AdminClient, row *pbv1.EventItem) {
	for _, rt := range e.table.Routes {
		if rt.Kind != row.Kind {
			continue
		}
		if rt.NoteContains != "" && !strings.Contains(row.Note, rt.NoteContains) {
			continue
		}
		for _, act := range rt.Actions {
			switch b := act.GetBody().(type) {
			case *pbv1.Action_Notify:
				e.execNotify(ctx, row, b.Notify)
			case *pbv1.Action_Dispatch:
				e.execDispatch(ctx, cli, row, b.Dispatch)
			}
		}
	}
}

func (e *Engine) execNotify(ctx context.Context, row *pbv1.EventItem, n *pbv1.NotifyAction) {
	if err := e.cfg.Notify(ctx, e.render(n.GetTemplate(), row, true)); err != nil {
		e.cfg.Logf("notify 失败（仅记，不阻塞水位）: %v", err)
	}
}

// execDispatch 渲染模板→prompt 文件→AdminGateway.Dispatch（它不直接生
// 孩子——铁律①③）。dispatch 失败重发一次（有界重试 1 次，§3）。
func (e *Engine) execDispatch(ctx context.Context, cli AdminClient, row *pbv1.EventItem, d *pbv1.DispatchAction) {
	req := &pbv1.DispatchReq{
		TicketId:    row.TicketId,
		Role:        d.Role,
		WorkDir:     d.WorkDir,
		PreferShell: d.PreferShell,
		Adjunct:     d.Adjunct,
	}
	if d.PromptTemplate != "" {
		body, err := os.ReadFile(filepath.Join(e.configRoot, d.PromptTemplate))
		if err != nil {
			e.cfg.Logf("dispatch 模板读失败 %s: %v", d.PromptTemplate, err)
			return
		}
		pf := filepath.Join(e.cfg.StateDir,
			fmt.Sprintf("prompt-%s-%d.md", row.TicketId, row.Id))
		if err := os.WriteFile(pf, []byte(e.render(string(body), row, false)), 0o600); err != nil {
			e.cfg.Logf("dispatch 提示落盘失败: %v", err)
			return
		}
		req.PromptFile = pf
	}
	for i := 0; i < 2; i++ { // 有界重试 1 次
		ack, err := cli.Dispatch(ctx, req)
		if err == nil && ack.GetOk() {
			return
		}
		if err != nil {
			e.cfg.Logf("dispatch 调用错（第%d次）: %v", i+1, err)
		} else {
			e.cfg.Logf("dispatch 被拒（第%d次）: %s", i+1, ack.GetNote())
		}
	}
}

// render 占位符替换：{{ticket_id}} {{kind}} {{note}} {{role}} {{run_dir}}
// {{gateway_sock}} {{action}}（动作中文化映射，决策41）。
func (e *Engine) render(tmpl string, row *pbv1.EventItem, isNotify bool) string {
	if tmpl == "" && isNotify {
		tmpl = "【Meta】{{role}}:{{action}}[{{ticket_id}}]" // 缺省模板（决策41）
	}
	r := strings.NewReplacer(
		"{{ticket_id}}", row.TicketId,
		"{{kind}}", row.Kind,
		"{{note}}", row.Note,
		"{{role}}", row.Role,
		"{{action}}", actionZh(row.Kind),
		"{{run_dir}}", e.cfg.RunDir,
		"{{gateway_sock}}", e.cfg.SocketPath,
	)
	return r.Replace(tmpl)
}

// actionZh 事件 kind→中文定性（决策41：缺省 notify 模板的动作映射表；
// 表外 kind 回原文——路由只走表内行，此处只作缺省渲染兜底）。
func actionZh(kind string) string {
	switch kind {
	case "claim":
		return "开工"
	case "deliver":
		return "完工交付"
	case "dispatch":
		return "已派单"
	case "stall_suspect":
		return "疑似卡死"
	case "budget_frozen":
		return "预算冻结止血"
	case "reject_limit":
		return "连驳三次"
	case "defer_limit":
		return "延期帽尽"
	case "audit_start":
		return "审计开工"
	case "audit_pass":
		return "查验通过"
	case "reject":
		return "驳回返工"
	case "ruling":
		return "卡死已裁"
	case "shells_exhausted":
		return "换壳链尽"
	case "spawn_needed":
		return "待派单"
	}
	return kind
}
