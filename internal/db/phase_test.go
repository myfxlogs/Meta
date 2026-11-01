package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// expectedEdges §6.2 钦定边表的独立副本——测试自带全量清单，实现表漏一边即红
// （验收锚 b：表驱动断言，漏一边＝红）。
var expectedEdges = []Edge{
	{PhaseCreated, PhaseDesigning, EdgeDesignStart},
	{PhaseDesigning, PhaseAssigned, EdgeDispatch},
	{PhaseAssigned, PhaseWorking, EdgeClaim},
	{PhaseWorking, PhaseReview, EdgeDeliver},
	{PhaseReview, PhaseAuditing, EdgeAuditStart},
	{PhaseAuditing, PhaseToDeploy, EdgeAuditPass},
	{PhaseToDeploy, PhaseDeploying, EdgeDeployStart},
	{PhaseDeploying, PhaseDone, EdgeDeployDone},
	{PhaseWorking, PhaseAssigned, EdgeRequeue},
	{PhaseAuditing, PhaseAssigned, EdgeReject},
	// 补边四条（A2）
	{PhaseDesigning, PhaseCreated, EdgeDesignDead},
	{PhaseDeploying, PhaseToDeploy, EdgeDeployFailed},
	{PhaseAuditing, PhaseReview, EdgeAuditDead},
	{PhaseWorking, PhaseAssigned, EdgeYield},
	// 补边（阶段3·票0011）：CTO-A 需求待裁泊车
	{PhaseWorking, PhaseCreated, EdgeReqPending},
	// 任一非终态→作废
	{PhaseCreated, PhaseVoid, EdgeVoid},
	{PhaseDesigning, PhaseVoid, EdgeVoid},
	{PhaseAssigned, PhaseVoid, EdgeVoid},
	{PhaseWorking, PhaseVoid, EdgeVoid},
	{PhaseReview, PhaseVoid, EdgeVoid},
	{PhaseAuditing, PhaseVoid, EdgeVoid},
	{PhaseToDeploy, PhaseVoid, EdgeVoid},
	{PhaseDeploying, PhaseVoid, EdgeVoid},
}

var allKinds = []EdgeKind{
	EdgeDesignStart, EdgeDispatch, EdgeClaim, EdgeDeliver, EdgeAuditStart,
	EdgeAuditPass, EdgeDeployStart, EdgeDeployDone, EdgeRequeue, EdgeReject,
	EdgeDesignDead, EdgeDeployFailed, EdgeAuditDead, EdgeYield, EdgeReqPending, EdgeVoid,
}

func allPhases() []Phase { return append(append([]Phase{}, NonTerminal...), Terminal...) }

// TestAnchorB_EdgeTable 边表逐边：合法边全过、非法边全拒、表无多无缺。
func TestAnchorB_EdgeTable(t *testing.T) {
	// 1) 每条钦定边都合法
	for _, e := range expectedEdges {
		if !Can(e.From, e.To, e.Kind) {
			t.Errorf("钦定边被拒: %s -[%s]-> %s", e.From, e.Kind, e.To)
		}
	}
	// 2) 实现表与钦定表一一对应（多一边/少一边都红）
	if len(Edges) != len(expectedEdges) {
		t.Fatalf("边表条数 %d ≠ 钦定 %d", len(Edges), len(expectedEdges))
	}
	want := map[Edge]bool{}
	for _, e := range expectedEdges {
		want[e] = true
	}
	for _, e := range Edges {
		if !want[e] {
			t.Errorf("实现表多出非钦定边: %s -[%s]-> %s", e.From, e.Kind, e.To)
		}
	}
	// 3) 全组合扫：钦定之外的 (from,to,kind) 全拒
	for _, from := range allPhases() {
		for _, to := range allPhases() {
			for _, k := range allKinds {
				if want[Edge{from, to, k}] {
					continue
				}
				if Can(from, to, k) {
					t.Errorf("非法边被放行: %s -[%s]-> %s", from, k, to)
				}
			}
		}
	}
}

// TestTransition 迁移落库：合法边相位推进＋事件留痕；越边拒且相位不动。
func TestTransition(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	mkTicket(t, d, "t1")

	if _, err := d.Transition(ctx, "t1", PhaseDesigning, EdgeDesignStart, "cto-a", ""); err != nil {
		t.Fatalf("合法边被拒: %v", err)
	}
	if p, _ := d.PhaseOf(ctx, "t1"); p != PhaseDesigning {
		t.Fatalf("相位=%s 预期 设计中", p)
	}
	var ille *ErrIllegalEdge
	if _, err := d.Transition(ctx, "t1", PhaseDone, EdgeDeployDone, "x", ""); !errors.As(err, &ille) {
		t.Fatalf("越边未拒: %v", err)
	}
	if p, _ := d.PhaseOf(ctx, "t1"); p != PhaseDesigning {
		t.Fatalf("越边后相位被改: %s", p)
	}
	// 驳回计数
	toPhase(t, d, "t1", PhaseAuditing)
	if _, err := d.Transition(ctx, "t1", PhaseAssigned, EdgeReject, "审计", "判决：退码不符"); err != nil {
		t.Fatalf("驳回边被拒: %v", err)
	}
	tk, _ := d.GetTicket(ctx, "t1")
	if tk.RejectCount != 1 {
		t.Fatalf("reject_count=%d 预期 1", tk.RejectCount)
	}
	// 判决史留痕：驳回理由在 ticket_event
	evs, _ := d.Events(ctx, "t1")
	found := false
	for _, e := range evs {
		if e.Kind == string(EdgeReject) && e.Note == "判决：退码不符" && e.FromPhase == PhaseAuditing && e.ToPhase == PhaseAssigned {
			found = true
		}
	}
	if !found {
		t.Fatal("驳回事件未留痕")
	}
}

// TestAnchorH_MigrationSmoke 迁移烟测：九表齐、WAL 开（验收锚 h）。
func TestAnchorH_MigrationSmoke(t *testing.T) {
	d := openTemp(t)
	tables, err := d.TableNames()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"ticket": true, "ticket_event": true, "seat": true, "seat_run": true,
		"shell": true, "project": true, "ceo_memory": true, "budget": true, "secret": true,
		"cmd_run": true, // 票0010 第十表（代跑留痕）
	}
	if len(tables) != len(want) {
		t.Fatalf("表数 %d ≠ 10: %v", len(tables), tables)
	}
	for _, tb := range tables {
		if !want[tb] {
			t.Errorf("非钦定表: %s", tb)
		}
	}
	if m, _ := d.JournalMode(); m != "wal" {
		t.Fatalf("journal_mode=%s 预期 wal", m)
	}
}

// TestAnchor4_BackupSmoke 备份烟测：VACUUM INTO 产物只读打开且表数一致（验收锚4）。
func TestAnchor4_BackupSmoke(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	mkTicket(t, d, "tb")

	dest := filepath.Join(t.TempDir(), "backup.db")
	if err := d.Backup(dest); err != nil {
		t.Fatalf("VACUUM INTO: %v", err)
	}
	ro, err := Open(dest)
	if err != nil {
		t.Fatalf("备份只读打开失败: %v", err)
	}
	defer ro.Close()
	srcTables, _ := d.TableNames()
	dstTables, _ := ro.TableNames()
	if len(srcTables) != len(dstTables) {
		t.Fatalf("备份表数 %d ≠ 源 %d", len(dstTables), len(srcTables))
	}
	// 数据随行：票在备份里
	if tk, _ := ro.GetTicket(ctx, "tb"); tk == nil {
		t.Fatal("备份缺票行")
	}
}

// ---- helpers ----

func openTemp(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func mkTicket(t *testing.T, d *DB, id string) {
	t.Helper()
	if err := d.CreateTicket(context.Background(), id, id+" 票", nil); err != nil {
		t.Fatal(err)
	}
}

// toPhase 沿主链把票推到指定相位（测试布置用），从当前相位续走。
func toPhase(t *testing.T, d *DB, id string, target Phase) {
	t.Helper()
	order := []Phase{PhaseCreated, PhaseDesigning, PhaseAssigned, PhaseWorking,
		PhaseReview, PhaseAuditing, PhaseToDeploy, PhaseDeploying, PhaseDone}
	kinds := []EdgeKind{EdgeDesignStart, EdgeDispatch, EdgeClaim, EdgeDeliver,
		EdgeAuditStart, EdgeAuditPass, EdgeDeployStart, EdgeDeployDone}
	pos := func(p Phase) int {
		for i, q := range order {
			if q == p {
				return i
			}
		}
		return -1
	}
	for {
		p, err := d.PhaseOf(context.Background(), id)
		if err != nil || p == target {
			return
		}
		i := pos(p)
		if i < 0 || i+1 >= len(order) {
			t.Fatalf("toPhase: 相位 %s 不在主链", p)
		}
		if _, err := d.Transition(context.Background(), id, order[i+1], kinds[i], "test", ""); err != nil {
			t.Fatalf("toPhase→%s: %v", order[i+1], err)
		}
	}
}
