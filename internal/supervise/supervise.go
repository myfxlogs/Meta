// Package supervise 是网关二进制的守护父进程（票0003，B4 补全）：
// 以子进程前台 exec `<bin> serve <args>`，子死（SIGCHLD 事件）即重起；
// 连崩达 burst 熔断——若 bin.prev 存在且本轮未回滚过，原子晋升 prev→bin
// 再以 prev 续跑（升级坏版自动回滚恢复 serve）；无 prev 或已回滚仍崩→
// 停机交 systemd StartLimitBurst→OnFailure 报警。重启链有界（burst 封顶），
// 无周期巡检——死亡发现全靠 Wait 事件，合 R2。
package supervise

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Config 守护参数。
type Config struct {
	Bin     string        // 被守护二进制（如 /opt/meta/bin/meta）
	Args    []string      // 透传 serve 参数（含 "serve"）
	Prev    string        // 回滚源；缺省 Bin+".prev"
	Burst   int           // 连崩熔断阈值，缺省 5
	Stable  time.Duration // 子进程活过此时长→连崩计数清零，缺省 60s
	Backoff time.Duration // 重起间隔，缺省 3s
	Out     io.Writer     // 守护日志（缺省 os.Stderr）
}

func (c *Config) withDefaults() {
	if c.Prev == "" {
		c.Prev = c.Bin + ".prev"
	}
	if c.Burst <= 0 {
		c.Burst = 5
	}
	if c.Stable <= 0 {
		c.Stable = 60 * time.Second
	}
	if c.Backoff <= 0 {
		c.Backoff = 3 * time.Second
	}
	if c.Out == nil {
		c.Out = os.Stderr
	}
}

// Run 阻塞守护：ctx 取消（信号）→转发 SIGTERM 给子进程后退 0；
// 子进程正常退→退 0；熔断无可退→退 10（OnFailure 报警径）。
func Run(ctx context.Context, cfg Config) int {
	cfg.withDefaults()
	streak, rolledBack := 0, false
	for {
		start := time.Now()
		code, stopped := runChild(ctx, cfg)
		if stopped || code == 0 {
			return 0 // 信号终止或子进程正常退：传播，不计崩溃
		}
		if time.Since(start) >= cfg.Stable {
			streak = 0 // 稳态后死＝新一轮，不算连崩
		}
		streak++
		fmt.Fprintf(cfg.Out, "supervise: 第 %d 次连崩（退码=%d）\n", streak, code)
		if streak >= cfg.Burst {
			// 回滚二进制选择——对抗性锚提名区：删/翻本块，supervise 单测必红。
			if !rolledBack && fileExists(cfg.Prev) {
				if err := os.Rename(cfg.Prev, cfg.Bin); err != nil {
					fmt.Fprintf(cfg.Out, "supervise: 回退失败: %v\n", err)
					return 11
				}
				rolledBack, streak = true, 0
				fmt.Fprintf(cfg.Out, "supervise: 连崩熔断——已回退 %s → %s，以旧版重启\n",
					cfg.Prev, cfg.Bin)
				continue
			}
			fmt.Fprintf(cfg.Out, "supervise: 连崩熔断停机（prev 存在=%v 已回滚=%v）——交外层报警\n",
				fileExists(cfg.Prev), rolledBack)
			return 10
		}
		select {
		case <-time.After(cfg.Backoff):
		case <-ctx.Done():
			return 0
		}
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// runChild 前台 exec 一次（B5 禁 daemonize）：返回 (退码, 被停)。
// ctx 取消→先 SIGTERM 给子进程，3s 内不退再 SIGKILL。
func runChild(ctx context.Context, cfg Config) (int, bool) {
	cmd := exec.Command(cfg.Bin, cfg.Args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr // 直通→journal/演练日志留档
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(cfg.Out, "supervise: exec %s: %v\n", cfg.Bin, err)
		return 127, false
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return exitCode(err), false
	case <-ctx.Done():
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
		return 0, true
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1 // 信号杀死
}
