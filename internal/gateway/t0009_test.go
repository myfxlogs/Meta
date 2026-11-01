package gateway

// 票 0009 守卫簇锚 3/4/5：空提示拒起（D-40）、席位级死因禁标壳、壳级回归。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meta/internal/db"
	"meta/internal/inject"
	"meta/internal/route"
	"meta/internal/seat"
)

// exitcodeSpec 强制退码席 spec（SEAT_MODE=exitcode＋SEAT_EXIT_CODE=n）。
func exitcodeSpec(t *testing.T, code int) SeatSpec {
	t.Helper()
	s := dispatchSpec(t)
	s.Env = append(s.Env[:1:1],
		"SEAT_MODE=exitcode",
		fmt.Sprintf("SEAT_EXIT_CODE=%d", code),
		"META_ADOPT_GRACE=30",
		"PATH="+os.Getenv("PATH"))
	return s
}

// assertNoShellBlame 席位级/拒起径公共断言：failSet 空＋施工壳健康零迁移。
func assertNoShellBlame(t *testing.T, g *Gateway, tid string) {
	t.Helper()
	ctx := context.Background()
	g.mu.Lock()
	fs := g.failSet[tid]
	g.mu.Unlock()
	if len(fs) != 0 {
		t.Fatalf("票 %s 不该进 failSet: %v", tid, fs)
	}
	rows, _ := g.DB().ListShells(ctx, "施工")
	for _, r := range rows {
		if r.Health != route.HealthUnknown && r.Health != "" {
			t.Fatalf("票 %s 不该迁壳健康: %+v", tid, r)
		}
	}
}

// TestDispatch_空提示拒起 锚 3（D-40）：最终 PromptFile 空/读不出/全空白/
// 注入产物空→spawn_blocked＋seat_run 零新行＋failSet 空＋壳健康零迁移。
// 对抗性锚 b：删 dispatchOnce 的 promptFileEmpty 闸本表全红。
func TestDispatch_空提示拒起(t *testing.T) {
	ctx := context.Background()

	blocked := func(t *testing.T, g *Gateway, tid string) {
		t.Helper()
		e := waitEvent(t, g, "spawn_blocked", tid, 8*time.Second)
		if !strings.Contains(e.Note, "空提示拒起") {
			t.Fatalf("spawn_blocked 措辞不符: %q", e.Note)
		}
		if ev := hasEvent(t, g, tid, "spawn_blocked"); ev == nil {
			t.Fatal("spawn_blocked 未落事件表")
		}
		if rs, _ := g.DB().RunsForTicket(ctx, tid); len(rs) != 0 {
			t.Fatalf("空提示不该起席: runs=%d", len(rs))
		}
		assertNoShellBlame(t, g, tid)
		for _, e := range collectEvents(g, 300*time.Millisecond) {
			if e.Kind == "seat_up" && e.TicketID == tid {
				t.Fatal("空提示不该 seat_up")
			}
		}
	}

	// ①无覆盖径且注入机未启（生产主径）。
	t.Run("空提示注入未启", func(t *testing.T) {
		g, _ := newTestGateway(t, func(c *Config) {
			*c = dispatchCfg(filepath.Dir(c.DBPath))
		})
		mkTicketAt(t, g, "tg1", db.PhaseAssigned)
		spec := dispatchSpec(t)
		spec.PromptFile = ""
		if err := g.DispatchSeat(ctx, "tg1", "施工", spec); err != nil {
			t.Fatal(err)
		}
		blocked(t, g, "tg1")
	})

	// ②prompt_file 指不存在文件。
	t.Run("文件缺失", func(t *testing.T) {
		g, _ := newTestGateway(t, func(c *Config) {
			*c = dispatchCfg(filepath.Dir(c.DBPath))
		})
		mkTicketAt(t, g, "tg2", db.PhaseAssigned)
		spec := dispatchSpec(t)
		spec.PromptFile = filepath.Join(t.TempDir(), "no-such.md")
		if err := g.DispatchSeat(ctx, "tg2", "施工", spec); err != nil {
			t.Fatal(err)
		}
		blocked(t, g, "tg2")
	})

	// ③prompt_file 零字节。
	t.Run("零字节", func(t *testing.T) {
		g, _ := newTestGateway(t, func(c *Config) {
			*c = dispatchCfg(filepath.Dir(c.DBPath))
		})
		mkTicketAt(t, g, "tg3", db.PhaseAssigned)
		spec := dispatchSpec(t)
		zero := filepath.Join(t.TempDir(), "zero.md")
		if err := os.WriteFile(zero, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		spec.PromptFile = zero
		if err := g.DispatchSeat(ctx, "tg3", "施工", spec); err != nil {
			t.Fatal(err)
		}
		blocked(t, g, "tg3")
	})

	// ④注入机开但组装产物空（mock assemble 返 ""）→落空文件→同径拒起。
	t.Run("注入产物空", func(t *testing.T) {
		root := injectFixture(t)
		g, _ := newTestGateway(t, func(c *Config) {
			*c = dispatchCfg(filepath.Dir(c.DBPath))
			c.InjectRoot = root
		})
		old := assemble
		assemble = func(in inject.Input) (string, error) { return "", nil }
		defer func() { assemble = old }()
		if err := g.DB().CreateTicket(ctx, "tg4", "产物空票", testSpec(t)); err != nil {
			t.Fatal(err)
		}
		spec := dispatchSpec(t)
		spec.PromptFile = ""
		if err := g.DispatchSeat(ctx, "tg4", "施工", spec); err != nil {
			t.Fatal(err)
		}
		blocked(t, g, "tg4")
	})
}

// TestDispatch_席位级死因禁标壳 锚 4：helper 席强制 ExitSeatErr/ExitOrphan/
// ExitAbort/ExitInputErr→seat_err 事件留痕，failSet 不含该壳＋ShellHealth
// 行未动。对抗性锚 c：删 reportShell 的 ExitWrapper 早退分支本表全红。
func TestDispatch_席位级死因禁标壳(t *testing.T) {
	for _, code := range []int{
		seat.ExitSeatErr, seat.ExitOrphan, seat.ExitAbort, seat.ExitInputErr,
	} {
		t.Run(fmt.Sprintf("exit=%d", code), func(t *testing.T) {
			g, _ := newTestGateway(t, func(c *Config) {
				*c = dispatchCfg(filepath.Dir(c.DBPath))
			})
			ctx := context.Background()
			mkTicketAt(t, g, "tx", db.PhaseAssigned)
			if err := g.DispatchSeat(ctx, "tx", "施工", exitcodeSpec(t, code)); err != nil {
				t.Fatal(err)
			}
			// 三候选全部席位级秒退→shells_exhausted 收口（每候选各一 seat_err）。
			waitEvent(t, g, "shells_exhausted", "tx", 15*time.Second)
			nErr := 0
			evs, _ := g.DB().Events(ctx, "tx")
			for _, e := range evs {
				if e.Kind == "seat_err" {
					nErr++
					if !strings.Contains(e.Note, fmt.Sprintf("exit=%d", code)) {
						t.Fatalf("seat_err 缺退码: %q", e.Note)
					}
				}
			}
			if nErr != 3 {
				t.Fatalf("seat_err 行数=%d 预期 3（三候选各一）", nErr)
			}
			assertNoShellBlame(t, g, "tx")
		})
	}
}

// TestDispatch_壳级回归 锚 5：②a→limited＋failSet；②b→dead＋failSet；
// ①机器级进 failSet 不迁健康——本票只剥「席位级」，壳级语义一字不改。
func TestDispatch_壳级回归(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name       string
		code       int
		wantHealth string // ""＝不迁
		wantEvent  string // ""＝无健康事件
	}{
		{"①机器级", seat.ExitSpawnFail, "", ""},
		{"②a限流", seat.ExitTransient, route.HealthLimited, "shell_limited"},
		{"②b死额", seat.ExitFatal, route.HealthDead, "shell_dead"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newTestGateway(t, func(c *Config) {
				*c = dispatchCfg(filepath.Dir(c.DBPath))
			})
			mkTicketAt(t, g, "ts", db.PhaseAssigned)
			if err := g.DispatchSeat(ctx, "ts", "施工", exitcodeSpec(t, tc.code)); err != nil {
				t.Fatal(err)
			}
			waitEvent(t, g, "shells_exhausted", "ts", 15*time.Second)

			g.mu.Lock()
			fs := g.failSet["ts"]
			g.mu.Unlock()
			rows, _ := g.DB().ListShells(ctx, "施工")
			if len(fs) != len(rows) || len(fs) == 0 {
				t.Fatalf("壳级死因应全行进 failSet: %v（rows=%d）", fs, len(rows))
			}
			for _, r := range rows {
				key := r.ShellKey()
				if !fs[key] {
					t.Fatalf("failSet 缺壳键 %s", key)
				}
			}
			nHealthEv := 0
			evs, _ := g.DB().Events(ctx, "ts")
			for _, e := range evs {
				if e.Kind == "shell_limited" || e.Kind == "shell_dead" {
					nHealthEv++
				}
			}
			if tc.wantHealth == "" {
				for _, r := range rows {
					if r.Health != route.HealthUnknown {
						t.Fatalf("①机器级不该迁健康: %+v", r)
					}
				}
				if nHealthEv != 0 {
					t.Fatalf("①机器级不该有健康事件: %d", nHealthEv)
				}
			} else {
				for _, r := range rows {
					if r.Health != tc.wantHealth {
						t.Fatalf("健康=%s 预期 %s: %+v", r.Health, tc.wantHealth, r)
					}
				}
				if ev := hasEvent(t, g, "ts", tc.wantEvent); ev == nil {
					t.Fatalf("缺 %s 事件行", tc.wantEvent)
				}
			}
		})
	}
}
