package gateway

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// TestT18_凭据真只读 锚①：同 uid 席对 0444 凭据文件写必败（open O_WRONLY
// 即 EACCES——chmod 回改是二步升级属残面申报）；0555 目录新建文件亦败。
func TestT18_凭据真只读(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	t.Cleanup(func() { // 0555/0444 件清理前回权限位（TempDir unlinkat 闸）
		_ = filepath.Walk(g.cfg.RunDir, func(p string, _ os.FileInfo, e error) error {
			if e == nil {
				_ = os.Chmod(p, 0o700)
			}
			return nil
		})
	})
	ctx := context.Background()
	mkTicketAt(t, g, "ts18", db.PhaseWorking)
	if err := g.DB().CreateRun(ctx, &db.Run{
		RunID: "rs18", TicketID: "ts18", Role: "施工", Shell: "devin",
		RunToken: "tok18"}); err != nil {
		t.Fatal(err)
	}
	key, err := db.LoadOrCreateVaultKey(filepath.Join(t.TempDir(), "v.key"))
	if err != nil {
		t.Fatal(err)
	}
	g.vaultKey = key
	ct, _ := db.EncryptSecret(key, []byte("v4lue"))
	_ = g.DB().PutSecret(ctx, "k9", ct, []string{"施工"})
	st := &runState{run: &db.Run{RunID: "rs18", TicketID: "ts18", Role: "施工"}}
	g.runs["rs18"] = st
	_ = g.handleSecret(ctx, st, nil, &pbv1.SecretReq{Name: "k9"})

	secDir := filepath.Join(g.cfg.RunDir, "secrets", "rs18")
	if err := os.WriteFile(filepath.Join(secDir, "k9"), []byte("x"), 0o644); err == nil {
		t.Fatal("0444 凭据文件竟可写")
	}
	if err := os.WriteFile(filepath.Join(secDir, "new"), []byte("x"), 0o644); err == nil {
		t.Fatal("0555 凭据目录竟可新建")
	}
}

// TestT18_孤儿收尸secrets 锚②：孤儿席（绕过 onRunDead 的对账收尸径）
// secrets 目录连收——目录清空即判。
func TestT18_孤儿收尸secrets(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	t.Cleanup(func() {
		_ = filepath.Walk(g.cfg.RunDir, func(p string, _ os.FileInfo, e error) error {
			if e == nil {
				_ = os.Chmod(p, 0o700)
			}
			return nil
		})
	})
	ctx := context.Background()
	mkTicketAt(t, g, "to18", db.PhaseWorking)
	if err := g.DB().CreateRun(ctx, &db.Run{
		RunID: "ro18", TicketID: "to18", Role: "施工", Shell: "devin",
		RunToken: "tok18b", PID: 1 << 30}); err != nil { // 假死 pid（kill -0 拒）
		t.Fatal(err)
	}
	secDir := filepath.Join(g.cfg.RunDir, "secrets", "ro18")
	_ = os.MkdirAll(secDir, 0o555)
	_ = os.WriteFile(filepath.Join(secDir, "k1"), []byte("x"), 0o444)
	g.reconcile(ctx)
	if _, err := os.Stat(secDir); !os.IsNotExist(err) {
		t.Fatal("孤儿席 secrets 目录未收")
	}
}
