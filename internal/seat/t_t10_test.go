package seat

import (
	"strings"
	"testing"
)

// TestT10_CageArgv 笼 argv 锚：image/net/user/mounts/env 白名单成型；
// 宿主 env 白名单外项剥光（凭据/宿主件不入笼）。
func TestT10_CageArgv(t *testing.T) {
	t.Setenv("META_SEAT_NET", "meta-seat")
	t.Setenv("META_SECRETS_DIR", "/run/secrets/r1")
	t.Setenv("META_ART_DIR", "/run/artifacts/r1")
	t.Setenv("META_PROMPT_FILE", "/run/inject/t1-x.md")
	t.Setenv("META_SEAT_MOUNT_MASK", "run,/home")
	t.Setenv("META_RUN_TOKEN", "tok")
	t.Setenv("META_SEAT_EXTRA_MOUNT", "/run/logs:/run/logs:ro")
	t.Setenv("HOME", "/home/tester")
	t.Setenv("LEAKY_HOST_SECRET", "should-not-pass")
	argv := cageArgv("golang:1.27", []string{"claude", "-p"}, "/work/t1",
		[]string{"FOO=1"})
	joined := strings.Join(argv, " ")
	for _, want := range []string{
		"run", "--rm", "--network meta-seat",
		"-v /work/t1:/work/t1", "-w /work/t1",
		"-v /run/secrets/r1:/seat-secrets:ro",
		"-v /run/artifacts/r1:/run/artifacts/r1",
		"-v /run/inject:/run/inject:ro",
		"--tmpfs /home",
		"-v /run/logs:/run/logs:ro",
		"-e HOME", "-e META_SEAT_NET", "-e FOO=1",
		"golang:1.27", "claude", "-p",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("cage argv 缺 %q:\n%s", want, joined)
		}
	}
	// 相对径 mask 被拒（返修2锚）：只 /home 一条 tmpfs 存活。
	if strings.Contains(joined, "--tmpfs run") || strings.Contains(joined, "--tmpfs /work") {
		t.Fatalf("相对径 mask 未拒: %s", joined)
	}
	if !strings.Contains(joined, "--tmpfs /home") {
		t.Fatalf("绝对径 mask 未过: %s", joined)
	}
	for _, ban := range []string{"LEAKY_HOST_SECRET", "GOPRIVATE", "AWS_SECRET", "META_RUN_TOKEN", "META_GATEWAY_SOCK", "META_SEAT_SOCK"} {
		for _, a := range argv {
			if strings.Contains(a, ban) {
				t.Fatalf("白名单外 env 入笼 %q:\n%s", ban, joined)
			}
		}
	}
}
