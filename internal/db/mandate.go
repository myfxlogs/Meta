package db

// mandate.go —— T36 v2 §3.3-C Mandate 授权书：票表不可变列。
// 立票时由本层按立票径（provenance）推导落列，之后码面无 UPDATE 径；
// 范围变大＝新票。施工中/部署中 边前核仍有效（策略版本未被撤）。
// 席位提示词文字改变不了它——mandate 从不从注入/提示面取值。

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbv1 "meta/internal/pb/metav1"
)

// PolicyVersion 现行 Mandate 策略版本（网关启动时由 cfg 注入，缺省 v1）。
var PolicyVersion = "v1"

// RevokedVersions 已撤策略版本集——携该版本 mandate 的票不得进
// 施工中/部署中（网关启动时由 cfg.mandate_revoked 注入）。
var RevokedVersions = map[string]bool{}

// ErrNoMandate 票无授权书（或授权书损坏）不得进 施工中/部署中。
var ErrNoMandate = errors.New("mandate 授权书缺失/损坏")

// ErrMandateRevoked 授权书策略版本已撤。
var ErrMandateRevoked = errors.New("mandate 策略版本已撤")

// mandateKindOf provenance→来源种类映射：terminal（含空＝终端/admin 径）／
// feishu:*／其余一律 external。
func mandateKindOf(prov string) string {
	switch {
	case prov == "" || prov == "terminal" || prov == "gateway" || prov == "admin":
		return "terminal"
	case strings.HasPrefix(prov, "feishu"):
		return "feishu"
	default:
		return "external"
	}
}

// newMandate 立票时推导授权书（不可变列的唯一写点）。
func (d *DB) newMandate(prov, project string) []byte {
	kind := mandateKindOf(prov)
	strength := "inferred"
	if kind != "external" {
		strength = "explicit" // 终端/飞书白名单＝明示授权
	}
	b, err := proto.Marshal(&pbv1.Mandate{
		SourceKind:     kind,
		SourceIdentity: prov,
		ScopeProject:   project,
		Strength:       strength,
		PolicyVersion:  PolicyVersion,
		IssuedAt:       timestamppb.New(d.now()),
	})
	if err != nil {
		return nil
	}
	return b
}

// checkMandate 边核：mandate 存在＋策略版本未撤。在 Transition 同一 tx 内调。
func checkMandate(blob []byte) error {
	if len(blob) == 0 {
		return ErrNoMandate
	}
	var m pbv1.Mandate
	if err := proto.Unmarshal(blob, &m); err != nil {
		return fmt.Errorf("%w: %v", ErrNoMandate, err)
	}
	if RevokedVersions[m.PolicyVersion] || m.PolicyVersion != PolicyVersion {
		return fmt.Errorf("%w: %s", ErrMandateRevoked, m.PolicyVersion)
	}
	return nil
}
