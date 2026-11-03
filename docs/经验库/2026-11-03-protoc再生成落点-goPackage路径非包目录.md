# protoc 再生成落点：--go_out=. 按 go_package 全径落盘，不是包目录直出

票 T34 实证（2026-11-03）。

## 坑面

`protoc -I proto --go_out=. --go-grpc_out=. --connect-go_out=. proto/meta/v1/meta.proto`

产物落到 `<cwd>/meta/internal/pb/metav1/`（go_package = "meta/internal/pb/metav1"
被当相对路径逐级建目录），**不是** 仓库内的 `internal/pb/metav1/`。
`git status` 看不到 pb 变更，容易误判"没生成"或"已覆盖"。

## 判法与收法

- 生成后 `find . -name '*.pb.go' -newer go.mod -mmin -3` 定位真落点；
- 本仓收法＝cp 回 `internal/pb/metav1/`（含 metav1connect/），删临时 `meta/` 树；
- 另一径是 `--go_opt=paths=source_relative`，但本仓惯例就是 cp 归位（票0015/0022
  文档同径）。

## 同场提醒

proto 删字段顺手 `reserved <号>; reserved "<名>";`——旧 textproto 配置/其他仓
旧生成码撞上重用号才不至于静默错位。
