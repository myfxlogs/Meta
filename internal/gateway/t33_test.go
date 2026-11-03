package gateway

// T33 席位包生环境与化身第三断点——三件各锚。

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meta/internal/db"
)

// 件① spawn 基座环境：缺省补 PATH/HOME（沿网关值）＋XDG/DBUS（运行目录
// 存在才拼）；已有键不动。systemd-run --user 连用户管理器靠 XDG/DBUS——
// metaseat 网关 environ 零 XDG（0049 exit=-1 三连实证根因）。
// 对抗锚提名行：seatBaseEnv——删则 scope 席 argv 起跑前死。
func TestT33_SeatBaseEnv(t *testing.T) {
	t.Setenv("PATH", "/probe/bin:/usr/bin")
	t.Setenv("HOME", "/home/probe")
	xdg := t.TempDir()
	get := func(env []string, k string) (string, bool) {
		for _, kv := range env {
			if strings.HasPrefix(kv, k+"=") {
				return kv[len(k)+1:], true
			}
		}
		return "", false
	}

	// 注入件零基座→四键补齐。
	env := seatBaseEnvAt([]string{"META_X=1"}, xdg)
	for _, k := range []string{"PATH", "HOME", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS"} {
		if _, ok := get(env, k); !ok {
			t.Fatalf("基座键 %s 未补: %v", k, env)
		}
	}
	if v, _ := get(env, "XDG_RUNTIME_DIR"); v != xdg {
		t.Fatalf("XDG=%s 预期 %s", v, xdg)
	}
	if v, _ := get(env, "DBUS_SESSION_BUS_ADDRESS"); v != "unix:path="+xdg+"/bus" {
		t.Fatalf("DBUS=%s 未按 XDG 拼", v)
	}
	if v, _ := get(env, "PATH"); v != "/probe/bin:/usr/bin" {
		t.Fatalf("PATH=%s 未沿网关", v)
	}

	// 已有键不动（显式配置优先——注：DBUS 同在场）。
	full := []string{"PATH=/keep", "HOME=/keep", "XDG_RUNTIME_DIR=/keep/x",
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/keep/x/bus"}
	if env = seatBaseEnvAt(full, xdg); len(env) != 4 {
		t.Fatalf("已有键被扰: %v", env)
	}

	// XDG 有 DBUS 缺→按既有 XDG 拼（不假设 uid 布局）。
	env = seatBaseEnvAt([]string{"XDG_RUNTIME_DIR=/rt/u"}, xdg)
	if v, _ := get(env, "DBUS_SESSION_BUS_ADDRESS"); v != "unix:path=/rt/u/bus" {
		t.Fatalf("DBUS 未按既有 XDG 拼: %s", v)
	}

	// 运行目录不存在→不强补坏路径（容器/无登录会话径）。
	env = seatBaseEnvAt(nil, filepath.Join(xdg, "nope"))
	if _, ok := get(env, "XDG_RUNTIME_DIR"); ok {
		t.Fatal("无运行目录仍补 XDG")
	}
}

// 件② 化身唤醒第三断点：transition(EdgeDeployFailed) 真径——phase: 前缀
// 门铃外补语义门铃 "deploy_failed"→泵闸命中 spawn_kinds→ensureCEO→
// CEO run 行现（票面锚：run 行或 last_resort 必现其一）。
// 对抗锚提名行：transition 内 EdgeDeployFailed 补 emit——删则部署失败
// 永远唤不起无头化身（旧测试手注裸 kind 遮了前缀失配）。
func TestT33_DeployFailedWakesCEO(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "ceo")
	t.Setenv("META_ADOPT_GRACE", "30")
	g, _ := newTestGateway(t, func(c *Config) {
		*c = ceoTestCfg(filepath.Dir(c.DBPath), true)
		c.CeoSpawnKinds["deploy_failed"] = true
		c.CeoEvents["deploy_failed"] = true
	})
	ctx := context.Background()
	mkTicketAt(t, g, "t33", db.PhaseDeploying)
	// T35：deploy_failed 语义门铃径改成死席排干 failed 哨兵（失败表
	// blocked 前的失败账事件——化身唤醒锚保持不变）。
	writeSentinel(t, filepath.Dir(g.cfg.DBPath), "r33", "META_DEPLOY:failed\n")
	g.onRunDead(deadSeatUnit("t33", "r33"))
	waitEvent(t, g, "deploy_failed", "t33", 3*time.Second)
	waitFor2(t, 15*time.Second, func() bool {
		r, _ := g.DB().ActiveRunForRole(ctx, "CEO")
		return r != nil && r.Provenance == "system_event"
	})
	// 杀掉化身→D-22 比对不炸（未派发事件零计数早退）。
	if r, _ := g.DB().ActiveRunForRole(ctx, "CEO"); r != nil {
		g.killRun(g.bindRun(r))
	}
}

// 件②补 ensureCEO 静默早退补响：壳登记读败原径零痕迹——改计唤起连败
// （3 连败→ceo_last_resort 既有径兜底）。
func TestT33_EnsureCEOShellsFailLoud(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = ceoTestCfg(filepath.Dir(c.DBPath), true)
	})
	g.DB().Close() // 强制壳登记读败径
	g.ensureCEO(context.Background(), Event{Kind: "deploy_failed", TicketID: "t33"})
	g.mu.Lock()
	n := g.ceoFails
	g.mu.Unlock()
	if n != 1 {
		t.Fatalf("壳登记读败静默早退（ceoFails=%d）——唤起死黑箱回潮", n)
	}
}
