package gateway

// 票 T36 v2 波3 锚测：Mandate 授权书（§3.3-C）＋影子期 record_only（§3.3-D）。
// 锚：mandate 缺/过期票不得进施工中；伪造提示词改 mandate 必败；
// record_only 下零新票零新席（would_create 行实证）。

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"google.golang.org/protobuf/proto"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// 锚1：mandate 缺——绕过立票径直插（无 mandate 列）→ 施工中/部署中 边必拒。
func TestT36_MandateMissingBlocksWork(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	d := g.DB()
	now := time.Now().UnixNano()
	sq, err := sql.Open("sqlite", g.cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sq.Close()
	if _, err := sq.ExecContext(ctx,
		`INSERT INTO ticket (id, title, phase, created_at, updated_at) VALUES (?,?,?,?,?)`,
		"tm0", "裸票", string(db.PhaseAssigned), now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Transition(ctx, "tm0", db.PhaseWorking, db.EdgeClaim, "test", ""); err == nil {
		t.Fatal("无 mandate 票进施工中未被拒")
	}
}

// 锚2：策略版本已撤——票 mandate 有效但版本被撤 → 边必拒。
func TestT36_MandateRevokedBlocksWork(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "tm1", db.PhaseAssigned)
	old := db.RevokedVersions
	db.RevokedVersions = map[string]bool{db.PolicyVersion: true}
	defer func() { db.RevokedVersions = old }()
	if _, err := g.DB().Transition(ctx, "tm1", db.PhaseWorking, db.EdgeClaim, "test", ""); err == nil {
		t.Fatal("撤版 mandate 票进施工中未被拒")
	}
}

// 锚3：立票径 mandate 落列＋不可变——UpdateSpec/注入面写不动它；
// 正常票边照过（正控）。
func TestT36_MandateImmutable(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	if err := g.DB().CreateTicketProv(ctx, "tm2", "票", nil, "feishu:o_abc", "P"); err != nil {
		t.Fatal(err)
	}
	tk, _ := g.DB().GetTicket(ctx, "tm2")
	if len(tk.Mandate) == 0 {
		t.Fatal("mandate 未随立票落列")
	}
	var m pbv1.Mandate
	if err := proto.Unmarshal(tk.Mandate, &m); err != nil {
		t.Fatal(err)
	}
	if m.SourceKind != "feishu" || m.SourceIdentity != "feishu:o_abc" || m.PolicyVersion == "" {
		t.Fatalf("mandate 推导错: %s", m.String())
	}
	// 伪造 spec 面写 mandate——UpdateSpec 只碰 spec 列，mandate 应不变。
	_ = g.DB().UpdateSpec(ctx, "tm2", testSpec(t))
	tk2, _ := g.DB().GetTicket(ctx, "tm2")
	if string(tk2.Mandate) != string(tk.Mandate) {
		t.Fatal("spec 写径动了 mandate 不可变列")
	}
	// 正控：有效 mandate 票进施工中照过。
	mkTicketAt(t, g, "tm3", db.PhaseAssigned)
	if _, err := g.DB().Transition(ctx, "tm3", db.PhaseWorking, db.EdgeClaim, "test", ""); err != nil {
		t.Fatalf("有效 mandate 票进施工中误拒: %v", err)
	}
}

// 锚4：record_only——项目配方缺省（非 live）：过闸只落 would_create
// 账行，零新票零新席；切 live 真立票（正控）。
func TestT36_RecordOnly(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, db.Project{ID: "P"}); err != nil {
		t.Fatal(err)
	}
	run := mkCeoRun(t, g, "feishu:o_x")
	ref, err := g.CeoSubmitRequirement(ctx, &pbv1.Requirement{
		RunToken: run.RunToken, Goal: "做个东西", Project: "P"})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Id != "" {
		t.Fatalf("record_only 不应立票: %s", ref.Id)
	}
	evs, _ := g.DB().Events(ctx, "")
	var found bool
	for _, e := range evs {
		if e.Kind == "would_create" {
			found = true
		}
	}
	if !found {
		t.Fatal("would_create 账行未落")
	}
	// live 正控：同需求真立票。
	if err := g.DB().UpsertProject(ctx, db.Project{ID: "P", AutoTicketMode: "live"}); err != nil {
		t.Fatal(err)
	}
	ref2, err := g.CeoSubmitRequirement(ctx, &pbv1.Requirement{
		RunToken: run.RunToken, Goal: "做个东西", Project: "P"})
	if err != nil {
		t.Fatal(err)
	}
	if ref2.Id == "" {
		t.Fatal("live 模式应真立票")
	}
}
