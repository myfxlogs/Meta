package gateway

// 票 T53⑩ 自闸：名单族烧穿片必须带 deepenShellPool（漏挂再犯实证）。

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var burnList = []string{
	"TestBudget_ThreeGates", "TestBudget_AttemptReset",
	"TestCeoRule_RedirectPrompt", "TestDesignLock",
	"TestDispatch_PromptFile覆盖",
	"TestCeoSpawn", "TestCeoSpawn_DeployFailed", "TestCeoSpawn_LastResort",
	"TestCeoInquire", "TestAnchorD_AllPhaseReconcile",
	"TestAnchorE_AlarmRecompute", "TestT26_StallKillDeploySeat",
	"TestT30_CliDriveResetByRuling", "TestT44_AttachDrainDone",
	"TestT45_WorkingCtoLineageFallsBack", "TestDeploy_SeatDeadD2",
	"TestDesignDeadRespawn",
}

func TestT53_PoolCoverage(t *testing.T) {
	files, _ := filepath.Glob("*_test.go")
	var body []byte
	for _, f := range files {
		b, _ := os.ReadFile(f)
		body = append(body, b...)
	}
	for _, name := range burnList {
		re := regexp.MustCompile(`func ` + name + `\(t \*testing\.T\) \{([\s\S]*?)\n\}`)
		m := re.FindSubmatch(body)
		if m == nil {
			t.Fatalf("名单片 %s 未在测试件现形", name)
		}
		if !regexp.MustCompile(`deepenShellPool`).Match(m[1]) {
			t.Fatalf("名单片 %s 未挂 deepenShellPool——烧穿普查面再犯", name)
		}
	}
}
