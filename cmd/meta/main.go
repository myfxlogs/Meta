// meta 是网关单二进制（§3）：serve 起进程；secret put 是最小写径
//（凭证清单：CLI 读 stdin，值只进 secret 表密文）。
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"meta/internal/db"
	"meta/internal/gateway"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(os.Args[2:])
	case "secret":
		err = secret(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "meta:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `用法:
  meta serve  --db run/meta.db --socket run/meta.sock --rundir run
  meta secret put --db run/meta.db --key run/vault.key [--acl 角色1,角色2] <name>   (值读 stdin)`)
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	dbPath := fs.String("db", "run/meta.db", "SQLite 库路径")
	sock := fs.String("socket", "run/meta.sock", "席位面 unix socket")
	runDir := fs.String("rundir", "run", "run 目录（seats/logs/vault.key）")
	progressTimeout := fs.Duration("progress-timeout", 30*time.Minute, "进度帧闹钟")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := gateway.New(gateway.Config{
		DBPath:          *dbPath,
		RunDir:          *runDir,
		SocketPath:      *sock,
		ProgressTimeout: *progressTimeout,
	})
	if err != nil {
		return err
	}
	defer g.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- g.Serve() }()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case s := <-sig:
		fmt.Fprintln(os.Stderr, "meta: 收信号", s)
		return nil
	case err := <-errCh:
		return err
	}
}

func secret(args []string) error {
	if len(args) < 1 || args[0] != "put" {
		usage()
		return fmt.Errorf("secret 子命令只实现 put")
	}
	fs := flag.NewFlagSet("secret put", flag.ContinueOnError)
	dbPath := fs.String("db", "run/meta.db", "SQLite 库路径")
	keyPath := fs.String("key", "run/vault.key", "主密钥路径")
	acl := fs.String("acl", "", "放行角色，逗号分隔（CEO,CTO,审计,施工）")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("secret put <name>")
	}
	name := fs.Arg(0)

	key, err := db.LoadOrCreateVaultKey(*keyPath)
	if err != nil {
		return err
	}
	if err := os.Chmod(*keyPath, 0o600); err != nil {
		return err
	}

	plain, err := io.ReadAll(os.Stdin) // 值走 stdin，不落 argv/日志
	if err != nil {
		return err
	}
	ct, err := db.EncryptSecret(key, plain)
	if err != nil {
		return err
	}

	d, err := db.Open(*dbPath)
	if err != nil {
		return err
	}
	defer d.Close()
	var roles []string
	if *acl != "" {
		roles = strings.Split(*acl, ",")
	}
	if err := d.PutSecret(context.Background(), name, ct, roles); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "meta: secret %s 已写入（%dB 密文，acl=%q）\n", name, len(ct), *acl)
	return nil
}
