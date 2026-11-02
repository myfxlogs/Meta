package gateway

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meta/internal/adapter"
	"meta/internal/db"
)

// 票0034 巡检链路两缺回归锚：过滤壳死因分类＋patrol-* 派发兜底。

// TestFilter_ClassifyExit 验收锚2（表驱六 fixture）：超时 ctx＋被杀→「超时」；
// 限额 stderr→「限额」；限额 JSONL stdout→「限额」；非 ExitError→「起壳败」；
// 无特征 ExitError→「退码非零」；quotaLike 正反例。对抗锚：删超时分支→
// 首行 fixture 红。
func TestFilter_ClassifyExit(t *testing.T) {
	run := func(cctx context.Context, script string) ([]byte, error) {
		return exec.CommandContext(cctx, "sh", "-c", script).Output()
	}
	cases := []struct {
		name, script, want, ban string
		noBin                   bool // 起壳败径：二进制缺，非 ExitError
		timeout                 bool // 超时径：cctx 带截止
	}{
		{name: "超时被杀", script: "sleep 2", want: "超时", timeout: true},
		{name: "限额stderr", script: `echo "rate limit exceeded (429)" >&2; exit 3`,
			want: "限额", ban: "退码非零"},
		{name: "限额JSONL stdout",
			script: `echo '{"type":"error","message":"insufficient_quota: daily limit"}'; exit 2`,
			want:   "限额"},
		{name: "起壳败", noBin: true, want: "起壳败"},
		{name: "退码非零无特征", script: `echo boom >&2; exit 5`, want: "退码非零", ban: "限额"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				cctx   context.Context
				cancel context.CancelFunc
				out    []byte
				err    error
			)
			if tc.timeout {
				cctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
			} else {
				cctx = context.Background()
			}
			if tc.noBin {
				_, err = exec.CommandContext(cctx, "/no/such/filter-bin").Output()
			} else {
				out, err = run(cctx, tc.script)
			}
			if err == nil {
				t.Fatalf("fixture %s 应出错", tc.name)
			}
			got := classifyFilterExit(cctx, err, out)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("分类错: got %q want 含 %q", got, tc.want)
			}
			if tc.ban != "" && strings.Contains(got, tc.ban) {
				t.Fatalf("误分类: got %q 不应含 %q", got, tc.ban)
			}
		})
	}

	// 第六 fixture：quotaLike 正反例（对抗锚：判定行翻→本段红）。
	for _, s := range []string{
		"rate limit exceeded", "HTTP 429 Too Many Requests", "insufficient_quota",
		"API daily limit reached", "Rate Limit Exceeded", "usage limit exceeded",
		"账户限额打穿",
	} {
		if !quotaLike(s) {
			t.Fatalf("quotaLike(%q) 应命中", s)
		}
	}
	for _, s := range []string{"everything ok", "timeout after 30s", "connection refused", ""} {
		if quotaLike(s) {
			t.Fatalf("quotaLike(%q) 不应命中", s)
		}
	}
}

// stubFilterAdapter 伪过滤壳：Build 产 sh -c 固定脚本——经 newAdapter 包级
// 缝换入，走通 cheapFilter 落盘/装配/限时子进程/判读全径。
type stubFilterAdapter struct{ script string }

func (a stubFilterAdapter) Name() string { return "stub" }
func (a stubFilterAdapter) Build(adapter.Spec) (adapter.Command, error) {
	return adapter.Command{Argv: []string{"sh", "-c", a.script}}, nil
}
func (a stubFilterAdapter) Parse([]byte) []adapter.Frame           { return nil }
func (a stubFilterAdapter) Tokens() int64                          { return 0 }
func (a stubFilterAdapter) FinalTokens(adapter.Spec) (int64, bool) { return 0, false }
func (a stubFilterAdapter) Classify(int, string) adapter.ExitClass { return adapter.ExitOK }

// TestFilter_QuotaUnjudgeable 验收锚3：无 KEEP/DROP 且含限额特征的输出→
// 错误含「限额」非裸「输出不可判」，且带净化证据尾。经 cheapFilter 全径
// （quotaLike 判定行删/翻→本锚红）。
func TestFilter_QuotaUnjudgeable(t *testing.T) {
	restore := newAdapter
	newAdapter = func(string) (adapter.Adapter, error) {
		return stubFilterAdapter{
			script: `echo '{"type":"error","message":"insufficient_quota: daily limit"}'`}, nil
	}
	defer func() { newAdapter = restore }()
	g, _ := newTestGateway(t, func(c *Config) {
		c.Filter.Name = "stub"
		c.Filter.Timeout = 5 * time.Second // 缺省 0＝ctx 即刻截止，会误入超时桶
	})
	_, err := g.cheapFilter(context.Background(), []Event{{Kind: "sig_hit", Note: "探针"}})
	if err == nil {
		t.Fatal("限额输出不应判成有效过滤")
	}
	if !strings.Contains(err.Error(), "限额") {
		t.Fatalf("限额特征未分类: %v", err)
	}
	if !strings.Contains(err.Error(), "输出不可判") {
		t.Fatalf("缺不可判语境: %v", err)
	}
	if !strings.Contains(err.Error(), "tail=") {
		t.Fatalf("缺证据尾: %v", err)
	}
}

// TestT34_PatrolDutyFallback 验收锚4：InjectRoot 空 fixture 下 patrol_tick
// →立票→派发→兜底件→过 D-40 抵壳路由（shells_exhausted，非 spawn_blocked
// 空提示）＋兜底件非空含票ID＋META_DELIVER（对抗锚：删 dispatchOnce 兜底
// 支→回 spawn_blocked 红）。次段：注入机败（inject_failed）对 patrol-*
// 续走兜底非 return。
func TestT34_PatrolDutyFallback(t *testing.T) {
	// 段一：注入机缺（InjectRoot 空）——11-02 实证场景复刻。
	g, dir := newTestGateway(t, func(c *Config) {
		c.Subs = []SubSeed{{Kind: "patrol_tick", Role: "巡检"}}
		c.Filter.BatchWindow = 50 * time.Millisecond
	})
	ctx := context.Background()
	d := g.DB()

	g.emit(Event{Kind: "patrol_tick", Note: "心跳T34"})
	var pid string
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && pid == "" {
		if ts, err := d.NonTerminalTickets(ctx); err == nil {
			for _, tk := range ts {
				if strings.HasPrefix(tk.ID, "patrol-") {
					pid = tk.ID
				}
			}
		}
		if pid == "" {
			<-time.After(20 * time.Millisecond)
		}
	}
	if pid == "" {
		t.Fatal("patrol_tick 未立值班票")
	}
	waitEvent(t, g, "shells_degraded", pid, 15*time.Second)
	evs, _ := d.Events(ctx, pid)
	for _, e := range evs {
		if e.Kind == "spawn_blocked" && strings.Contains(e.Note, "空提示") {
			t.Fatalf("兜底缺失，撞 D-40 空提示闸: %+v", e)
		}
	}
	pfs, err := filepath.Glob(filepath.Join(dir, "run", "inject", "duty-*.md"))
	if err != nil || len(pfs) == 0 {
		t.Fatalf("兜底提示件未落盘: %v %v", pfs, err)
	}
	body, err := os.ReadFile(pfs[len(pfs)-1])
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{pid, "META_DELIVER", "巡检事件处置", "心跳T34"} {
		if !strings.Contains(string(body), s) {
			t.Fatalf("兜底提示件缺 %q:\n%s", s, body)
		}
	}

	// 段二：注入机败（无 spec→inject_failed）→patrol-* 续走兜底抵壳路由。
	root := injectFixture(t)
	g2, dir2 := newTestGateway(t, func(c *Config) { c.InjectRoot = root })
	mkTicketAt(t, g2, "patrol-t34b", db.PhaseAssigned)
	if err := g2.DispatchSeat(ctx, "patrol-t34b", "巡检", SeatSpec{Provenance: "test"}); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g2, "inject_failed", "patrol-t34b", 8*time.Second)
	waitEvent(t, g2, "shells_degraded", "patrol-t34b", 15*time.Second)
	pfs2, err := filepath.Glob(filepath.Join(dir2, "run", "inject", "duty-*.md"))
	if err != nil || len(pfs2) == 0 {
		t.Fatalf("注入败后兜底件未落盘: %v %v", pfs2, err)
	}
}
