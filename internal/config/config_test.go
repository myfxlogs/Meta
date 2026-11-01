package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoad_缺坏拒起（票0004 锚 7 首句）：配置必载（D-9）——缺省路径无文件
// →错带路径与 --config 指引（serve 据此拒起，业主照错文可直做）；
// 坏内容→解析错；好配置→载入。
func TestLoad_缺坏拒起(t *testing.T) {
	d := t.TempDir()
	missing := filepath.Join(d, "gateway.textproto")
	_, err := Load(missing)
	if err == nil {
		t.Fatal("缺文件应拒")
	}
	if !strings.Contains(err.Error(), missing) || !strings.Contains(err.Error(), "--config") {
		t.Fatalf("拒起错应带路径与指引: %v", err)
	}

	bad := filepath.Join(d, "bad.textproto")
	if err := os.WriteFile(bad, []byte("not_a_field: {"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil {
		t.Fatal("坏内容应拒")
	}

	ok := filepath.Join(d, "ok.textproto")
	if err := os.WriteFile(ok, []byte("default_attempt_cap: 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(ok)
	if err != nil || c.GetDefaultAttemptCap() != 1 {
		t.Fatalf("好配置应载: %v %+v", err, c)
	}
}
