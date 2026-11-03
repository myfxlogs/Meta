package buildinfo

import (
	"regexp"
	"runtime/debug"
	"testing"
)

// TestVersion §6锚④ 表测三例：拼接/复合戳拼接/降级无 +。
func TestVersion(t *testing.T) {
	for _, c := range []struct{ stamp, short, want string }{
		{"dev", "cf357b1", "dev+cf357b1"},
		{"a10dafe-deploy", "x1y2z3", "a10dafe-deploy+x1y2z3"},
		{"dev", "", "dev"},
	} {
		if got := Version(c.stamp, c.short); got != c.want {
			t.Errorf("Version(%q,%q)=%q want %q", c.stamp, c.short, got, c.want)
		}
	}
}

// TestShortRev §6锚④ 四例：40hex→前7/键缺→""/空值→""/短值原文。
func TestShortRev(t *testing.T) {
	rev40 := "cf357b1" + "0123456789abcdef0123456789abcdef0" // 40hex，前7=cf357b1
	for _, c := range []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{"40hex取前7", []debug.BuildSetting{{Key: "vcs.revision", Value: rev40}}, "cf357b1"},
		{"键缺", []debug.BuildSetting{{Key: "vcs.time", Value: "2026-11-03T00:00:00Z"}}, ""},
		{"空值", []debug.BuildSetting{{Key: "vcs.revision", Value: ""}}, ""},
		{"短值原文", []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}}, "abc"},
	} {
		if got := shortRev(c.settings); got != c.want {
			t.Errorf("%s: shortRev()=%q want %q", c.name, got, c.want)
		}
	}
}

// TestShortSmoke 笼内容两态：ReadBuildInfo 败/无戳→""；有戳→7位hex。
func TestShortSmoke(t *testing.T) {
	s := Short()
	if s == "" {
		return
	}
	if !regexp.MustCompile(`^[0-9a-f]{7}$`).MatchString(s) {
		t.Fatalf("Short()=%q 非 \"\" 亦非 7 位短哈希", s)
	}
}
