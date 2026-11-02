package db

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"

	pbv1 "meta/internal/pb/metav1"
)

// 入账消毒（决策 67 D-45①/决策 68）：SQLite TEXT 不验 UTF-8 而 proto
// 双侧强验——毒字节入库即埋雷，读出 marshal 必炸全流。一切写径实参
// 过 cargs，文本列在此收口；BLOB 列（spec wire）过 validSpecBlob 结构校验。

// clean 单串净化：非法 UTF-8 序列逐段替换为 U+FFFD（stdlib 语义，
// 合法串原样返回零分配）。
func clean(s string) string {
	return strings.ToValidUTF8(s, "�")
}

// cargs 写径实参映射：string→clean，其余类型（int64/[]byte/bool…）原样。
// 一切 ExecContext/Exec/写式 QueryRow 的 ? 实参过本件——覆盖而非逐列挑：
// 新增写径忘消毒的敞口在调用点机械消灭。
func cargs(args ...any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		if s, ok := a.(string); ok {
			out[i] = clean(s)
		} else {
			out[i] = a
		}
	}
	return out
}

// validSpecBlob spec BLOB 入库闸：空→原样（spec 可空语义不动）；
// proto.Unmarshal 败→err 拒写（坏 wire 进库＝注入机/检票面必炸，决策 67
// 实证毒 spec 十张）；成→canonical marshal 回写（顺带 UTF-8 强验——
// 栏内含毒字节的 spec 在此被拒，不落半废票）。
func validSpecBlob(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return b, nil
	}
	ts := &pbv1.TicketSpec{}
	if err := proto.Unmarshal(b, ts); err != nil {
		return nil, fmt.Errorf("spec blob 非合法 TicketSpec: %w", err)
	}
	out, err := proto.Marshal(ts)
	if err != nil {
		return nil, fmt.Errorf("spec blob canonical marshal: %w", err)
	}
	return out, nil
}
