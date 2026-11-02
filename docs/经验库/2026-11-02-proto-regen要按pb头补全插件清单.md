日期：2026-11-02
席位：施工（票0021）
现象：票面给的 pb regen 命令只带 --go_out/--connect-go_out 两个插件——照跑后
meta_grpc.pb.go 未再生（新增 rpc 的服务桩缺失，下游 c.Reconcile 调用编译断）。
仓内 pb 是三件套（pb/grpc/connect），插件清单以生成件头为准，不以票面命令为准。
一句话教训：**regen 前先数仓内 pb 生成件**——每个文件头 protoc-gen-X 即所需
插件；票面/惯例命令漏插件时按头补全（go-grpc_out 属 protoc-gen-go-grpc，独立插件）。

## 复现
`protoc --go_out --connect-go_out meta.proto` 后 `git status` 只见两件变——
`meta_grpc.pb.go` 纹丝不动（它由 protoc-gen-go-grpc 生成，与 go_out 无关）。

## 解法
```bash
protoc -I proto --go_out=. --go_opt=module=meta \
  --go-grpc_out=. --go-grpc_opt=module=meta \
  --connect-go_out=. --connect-go_opt=module=meta proto/meta/v1/meta.proto
```
判据：regen 后 `git diff` 必须覆盖全部生成件中受影响的——新 rpc 三件都该动。

## 引用
票0021 proto 增 AdminGateway.Reconcile＋RouteTable.default_notify。
