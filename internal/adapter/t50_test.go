package adapter

import "testing"

// T50②：claude 中文限额签名入瞬态特征（429 失明实证）。
func TestT50_CnLimitSignature(t *testing.T) {
	for _, s := range []string{
		"[1310][您已达到每周使用上限，您的限额将在 11-09 重置]",
		"[1310][您已达到每月使用上限]",
		"rate limit exceeded",
	} {
		if !reTransient.MatchString(s) {
			t.Fatalf("签名未中: %s", s)
		}
	}
}
