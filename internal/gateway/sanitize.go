package gateway

import "strings"

// clean 流出面单串净化：非法 UTF-8 换 U+FFFD（stdlib 语义，合法串原样）。
// 决策 67 D-45③：DB 存量毒行/壳原文尾段进入 proto 构建点前必过本件——
// proto 双侧强验 UTF-8，一处漏净＝marshal 崩整流。
func clean(s string) string {
	return strings.ToValidUTF8(s, "�")
}
