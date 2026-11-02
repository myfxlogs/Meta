package gateway

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// TestT7_ScopedSecrets 三锚：放行→run/secrets/<runid>/<name> 0700/0600 落盘
// 可读；越权读→Denied+secret_denied 事件；死席→凭据目录回收。
func TestT7_ScopedSecrets(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "ts7", db.PhaseWorking)
	if err := g.DB().CreateRun(ctx, &db.Run{
		RunID: "rs7", TicketID: "ts7", Role: "施工", Shell: "devin",
		RunToken: "tok7"}); err != nil {
		t.Fatal(err)
	}
	// 种子凭据：secret.k1 只放行 施工。
	key, err := db.LoadOrCreateVaultKey(filepath.Join(t.TempDir(), "v.key"))
	if err != nil {
		t.Fatal(err)
	}
	g.vaultKey = key
	ct, _ := db.EncryptSecret(key, []byte("v4lue"))
	if err := g.DB().PutSecret(ctx, "k1", ct, []string{"施工"}); err != nil {
		t.Fatal(err)
	}

	st := &runState{run: &db.Run{RunID: "rs7", TicketID: "ts7", Role: "施工"}}
	g.runs["rs7"] = st

	// 锚1：放行→落盘 0700/0600 可读。
	_ = g.handleSecret(ctx, st, nil, &pbv1.SecretReq{Name: "k1"})
	secDir := filepath.Join(g.cfg.RunDir, "secrets", "rs7")
	b, err := os.ReadFile(filepath.Join(secDir, "k1"))
	if err != nil || string(b) != "v4lue" {
		t.Fatalf("凭据未落盘: %v %q", err, b)
	}
	fi, _ := os.Stat(secDir)
	if fi.Mode().Perm() != 0o555 {
		t.Fatalf("凭据目录权限 %v ≠ 0555（T18 真只读收紧）", fi.Mode().Perm())
	}
	fif, _ := os.Stat(filepath.Join(secDir, "k1"))
	if fif.Mode().Perm() != 0o444 {
		t.Fatalf("凭据文件权限 %v ≠ 0444（T18 真只读收紧）", fif.Mode().Perm())
	}

	// 锚2：越权读（角色外）→Denied＋secret_denied 事件。
	if err := g.DB().CreateRun(ctx, &db.Run{
		RunID: "rs7b", TicketID: "ts7", Role: "巡检", Shell: "devin",
		RunToken: "tok7b", Adjunct: true}); err != nil {
		t.Fatal(err)
	}
	st2 := &runState{run: &db.Run{RunID: "rs7b", TicketID: "ts7", Role: "巡检"}}
	_ = g.handleSecret(ctx, st2, nil, &pbv1.SecretReq{Name: "k1"})
	evs, _ := g.DB().Events(ctx, "ts7")
	var saw bool
	for _, e := range evs {
		saw = saw || e.Kind == "secret_denied"
	}
	if !saw {
		t.Fatal("越权读无 secret_denied 留痕")
	}
	if _, err := os.Stat(filepath.Join(g.cfg.RunDir, "secrets", "rs7b", "k1")); !os.IsNotExist(err) {
		t.Fatal("越权凭据竟落盘")
	}

	// 锚3：死席→目录回收。
	st.exited, st.streamClosed = true, true
	g.onRunDead(st)
	if _, err := os.Stat(secDir); !os.IsNotExist(err) {
		t.Fatal("死席凭据目录未回收")
	}
}
