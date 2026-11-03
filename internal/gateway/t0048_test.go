package gateway

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"meta/internal/db"
)

// TestT48_StampKeyVCSHash 票0048-p1 桩例（§6锚④）：version 第二段
// <stamp>+<vcs短哈希> 输出形后，deployStateOK 装着戳/期戳双侧剥 +*
// 尾判等——装戳带 hash 对期戳无 hash（code-head 次选径）剥尾仍真；
// 双侧同带 hash 同串仍真；异戳带 hash 仍翻负（对抗语义不退）。
func TestT48_StampKeyVCSHash(t *testing.T) {
	g, dir := newTestGateway(t, nil)
	ctx := context.Background()
	inst := filepath.Join(dir, "meta.inst")
	nb := filepath.Join(dir, "bin", "meta.new")
	if err := g.DB().UpsertProject(ctx, db.Project{
		ID: "P", RepoURL: dir, DefaultBranch: "master"}); err != nil {
		t.Fatal(err)
	}
	for i, c := range []struct {
		name string
		cur  string // install version 第二段
		newV string // newBin version（""＝桩返 err→code-head 次选径）
		head string // repoCodeHead 桩值
		want bool
	}{
		{"装戳带hash对期戳无hash剥尾判等", "abc-deploy+hashX", "", "abc", true},
		{"双侧同hash同戳判等", "abc-deploy+hashX", "abc-deploy+hashX", "abc", true},
		{"异戳带hash仍翻负", "zzz-deploy+hashY", "", "abc", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			vers := map[string]string{inst: c.cur}
			if c.newV != "" {
				vers[nb] = c.newV
			}
			stubDeploySeams(t, vers, c.head, nil)
			tid := fmt.Sprintf("t48%d", i)
			mkTicketAt(t, g, tid, db.PhaseDeploying)
			if err := g.DB().SetTicketProject(ctx, tid, "P"); err != nil {
				t.Fatal(err)
			}
			writeDeployLog(t, dir, tid, "1",
				fmt.Sprintf("upgrade: 已装 bin/meta.new → %s（prev=true）\n", inst))
			if got := g.deployStateOK(ctx, tid); got != c.want {
				t.Fatalf("deployStateOK()=%v want %v（cur=%q newV=%q head=%q）",
					got, c.want, c.cur, c.newV, c.head)
			}
		})
	}
}
