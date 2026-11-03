package db

import "fmt"

// Phase 票相位（总设计 §6.2）。
type Phase string

const (
	PhaseCreated   Phase = "已立"
	PhaseDesigning Phase = "设计中"
	PhaseAssigned  Phase = "已派"
	PhaseWorking   Phase = "施工中"
	PhaseReview    Phase = "待审"
	PhaseAuditing  Phase = "审计中"
	PhaseToDeploy  Phase = "待部署"
	PhaseDeploying Phase = "部署中"
	PhaseDone      Phase = "完成"
	PhaseVoid      Phase = "作废"
)

// NonTerminal 全部非终态相位（任一非终态→作废）。
var NonTerminal = []Phase{
	PhaseCreated, PhaseDesigning, PhaseAssigned, PhaseWorking,
	PhaseReview, PhaseAuditing, PhaseToDeploy, PhaseDeploying,
}

// Terminal 终态相位。
var Terminal = []Phase{PhaseDone, PhaseVoid}

// EdgeKind 边的触发因——同一对相位可有多条边（回队/Yield），靠 kind 区分。
type EdgeKind string

const (
	EdgeDesignStart  EdgeKind = "design_start"  // 已立→设计中：CTO-A 起设计
	EdgeDispatch     EdgeKind = "dispatch"      // 设计中→已派：派票
	EdgeClaim        EdgeKind = "claim"         // 已派→施工中：席位领票（先连后领）
	EdgeDeliver      EdgeKind = "deliver"       // 施工中→待审：交付
	EdgeAuditStart   EdgeKind = "audit_start"   // 待审→审计中：起审计实例
	EdgeAuditPass    EdgeKind = "audit_pass"    // 审计中→待部署：判决通过
	EdgeDeployStart  EdgeKind = "deploy_start"  // 待部署→部署中：起部署实例
	EdgeDeployDone   EdgeKind = "deploy_done"   // 部署中→完成：三锚全绿
	EdgeRequeue      EdgeKind = "requeue"       // 施工中→已派：席死/卡死/换壳回队
	EdgeReject       EdgeKind = "reject"        // 审计中→已派：驳回（同壳重做）
	EdgeDesignDead   EdgeKind = "design_dead"   // 补边：设计中→已立（CTO 席死）
	EdgeDeployFailed EdgeKind = "deploy_failed" // 补边：部署中→待部署（失败+已回滚）
	EdgeAuditDead    EdgeKind = "audit_dead"    // 补边：审计中→待审（审计席死，不计驳回）
	EdgeYield        EdgeKind = "yield"         // 补边：施工中→已派（Yield，理由入判决史）
	EdgeReqPending   EdgeKind = "req_pending"   // 补边：施工中→已立（CTO-A 需求待裁泊车，阶段3 唯一新增边）
	EdgeVoid         EdgeKind = "void"          // 任一非终态→作废（CEO 令）
)

// Edge 相位边表一行。
type Edge struct {
	From Phase
	To   Phase
	Kind EdgeKind
}

// Edges §6.2 边表钦定全边：主链 8 边＋回队/驳回＋补边 4 条＋非终态×作废 8 条。
var Edges = []Edge{
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
	{PhaseDesigning, PhaseCreated, EdgeDesignDead},
	{PhaseDeploying, PhaseToDeploy, EdgeDeployFailed},
	{PhaseAuditing, PhaseReview, EdgeAuditDead},
	{PhaseWorking, PhaseAssigned, EdgeYield},
	{PhaseWorking, PhaseCreated, EdgeReqPending},

	{PhaseCreated, PhaseVoid, EdgeVoid},
	{PhaseDesigning, PhaseVoid, EdgeVoid},
	{PhaseAssigned, PhaseVoid, EdgeVoid},
	{PhaseWorking, PhaseVoid, EdgeVoid},
	{PhaseReview, PhaseVoid, EdgeVoid},
	{PhaseAuditing, PhaseVoid, EdgeVoid},
	{PhaseToDeploy, PhaseVoid, EdgeVoid},
	{PhaseDeploying, PhaseVoid, EdgeVoid},
}

// Can 机械判：边表内有 (from,to,kind) 即合法，否则拒。
func Can(from, to Phase, kind EdgeKind) bool {
	for _, e := range Edges {
		if e.From == from && e.To == to && e.Kind == kind {
			return true
		}
	}
	return false
}

// ErrIllegalEdge 越边。
type ErrIllegalEdge struct {
	From Phase
	To   Phase
	Kind EdgeKind
}

func (e *ErrIllegalEdge) Error() string {
	return fmt.Sprintf("非法边: %s -[%s]-> %s", e.From, e.Kind, e.To)
}
