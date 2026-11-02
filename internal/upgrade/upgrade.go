// Package upgrade 是自升级径（票0003）：换二进制＋restart＋健康自检，
// 自检失败自动回退 prev。与 supervise 的熔断回滚互补：本径管「升级时刻
// 的主动替换」，supervise 管「运行期崩溃循环的被动回退」（B4）。
// 二进制路径全部参数化——为 D-24 降权迁移留缝（决策33定向③）。
package upgrade

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pbv1 "meta/internal/pb/metav1"
)

// Config 升级参数。
type Config struct {
	NewBin       string        // 新二进制路径（必）
	Install      string        // 安装目标（如 /opt/meta/bin/meta；可配＝D-24 缝）
	Prev         string        // 回滚快照落点；缺省 Install+".prev"
	RestartCmd   string        // 重启命令（如 systemctl --user restart meta-gateway）；空=不重启
	CheckSock    string        // 健康自检 socket；空=不自检
	CheckTimeout time.Duration // 自检上限，缺省 15s
	Out          io.Writer     // 日志（缺省 io.Discard）
}

// Do 升级全流程：预检新版→快照旧版为 prev→原子换装→重启→健康自检；
// 自检红且有 prev→回退 prev 再重启，报错上交（不退回成功假象）。
func Do(cfg Config) error {
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.CheckTimeout <= 0 {
		cfg.CheckTimeout = 15 * time.Second
	}
	// 预检：新二进制能 exec 且 version 子命令活——坏文件/错架构在此拦下。
	if out, err := exec.Command(cfg.NewBin, "version").CombinedOutput(); err != nil || len(out) == 0 {
		return fmt.Errorf("upgrade: 新版预检失败（version 不可跑）: %v %s", err, out)
	}
	prev := cfg.Prev
	if prev == "" {
		prev = cfg.Install + ".prev"
	}
	wrotePrev := false
	if _, err := os.Stat(cfg.Install); err == nil {
		if err := copyAtomic(cfg.Install, prev); err != nil {
			return fmt.Errorf("upgrade: 快照 prev: %w", err)
		}
		wrotePrev = true
	}
	if err := copyAtomic(cfg.NewBin, cfg.Install); err != nil {
		return fmt.Errorf("upgrade: 换装: %w", err)
	}
	fmt.Fprintf(cfg.Out, "upgrade: 已装 %s → %s（prev=%v）\n", cfg.NewBin, cfg.Install, wrotePrev)

	restart := func() error {
		if cfg.RestartCmd == "" {
			return nil
		}
		c := exec.Command("sh", "-c", cfg.RestartCmd)
		c.Stdout, c.Stderr = cfg.Out, cfg.Out
		return c.Run()
	}
	// 0034-p3：自检执行体移位——本进程必死于自己发的 restart（同
	// cgroup），先 detached 起存活 checker（systemd-run scope，无则
	// setsid），restart 后由它探活/回退，自检径不再是死码。
	if cfg.CheckSock != "" {
		spawnChecker(cfg, prev, wrotePrev)
	}
	if err := restart(); err != nil {
		return fmt.Errorf("upgrade: 重启命令失败: %w", err)
	}
	if cfg.CheckSock == "" {
		return nil
	}
	fmt.Fprintf(cfg.Out, "upgrade: 自检已移交 detached checker（%s）\n", cfg.CheckSock)
	return nil
}

// spawnChecker detached 自检执行体（0034-p3）：upgrade 自身死于 restart
// ——checker 走 systemd-run scope（无则 setsid）逃 cgroup，restart 后探活，
// 红则回退 prev 再 restart 一次。CheckCmd 是 checker 进程入口（`meta
// upgrade-check`）。
func spawnChecker(cfg Config, prev string, wrotePrev bool) {
	self, err := os.Executable()
	if err != nil {
		return
	}
	args := []string{"upgrade-check",
		"--sock", cfg.CheckSock,
		"--prev", prev,
		"--install", cfg.Install,
		"--restart-cmd", cfg.RestartCmd,
		"--have-prev", fmt.Sprintf("%v", wrotePrev)}
	if p, err := exec.LookPath("systemd-run"); err == nil {
		c := exec.Command(p, "--user", "--scope", "--quiet", "--collect",
			"--unit=meta-upg-check", "--", self)
		c.Args = append(c.Args[:len(c.Args)-1], append([]string{self}, args...)...)
		if c.Run() == nil {
			return
		}
	}
	if p, err := exec.LookPath("setsid"); err == nil {
		c := exec.Command(p, append([]string{self}, args...)...)
		c.Stdout, c.Stderr = io.Discard, io.Discard
		_ = c.Start()
	}
}

// CheckCmd upgrade-check 入口：等待新网关起→Probe socket→红则回退 prev
// ＋再 restart。detached 径被杀/无 prev 时如实退错。
func CheckCmd(out io.Writer, sock, prev, install, restartCmd string, havePrev bool) error {
	if err := Probe(sock, 15*time.Second); err != nil {
		if havePrev {
			fmt.Fprintf(out, "upgrade-check: 自检红——回退 prev\n")
			if rerr := copyAtomic(prev, install); rerr == nil {
				_ = exec.Command("sh", "-c", restartCmd).Run()
			}
			return fmt.Errorf("upgrade-check: 自检红已回退: %w", err)
		}
		return fmt.Errorf("upgrade-check: 自检红（无 prev）: %w", err)
	}
	fmt.Fprintf(out, "upgrade-check: 健康自检绿 %s\n", sock)
	return nil
}

// copyAtomic 同目录 tmp＋rename 原子换装（防半截文件被 exec 拾到）。
func copyAtomic(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, b, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// Probe 健康自检：拨 socket 发一次 Phase RPC——NotFound（服务活着、票没有）
// 即绿；连接层错有界重试到 deadline（R2 边界：有界重试链非轮询）。
func Probe(sock string, timeout time.Duration) error {
	abs, err := filepath.Abs(sock) // D-23 同径：相对→绝对化
	if err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	var last error
	for {
		conn, err := grpc.NewClient("unix://"+abs,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err = pbv1.NewAdminGatewayClient(conn).Phase(ctx, &pbv1.TicketRef{Id: "__healthz__"})
		cancel()
		conn.Close()
		if err == nil {
			return nil // 票恰存在也算活着
		}
		// 服务应答出的任何 gRPC 状态错误（NotFound/Unimplemented/…）＝活着；
		// 只有连接层错（Unavailable/非状态错）才判红。
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unavailable &&
			st.Code() != codes.DeadlineExceeded {
			return nil
		}
		last = err
		if time.Now().After(deadline) {
			return fmt.Errorf("probe %s: %w", abs, last)
		}
		<-time.After(200 * time.Millisecond)
	}
}
