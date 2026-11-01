日期：2026-11-01
席位：施工（票0002）
现象：真壳冒烟锚7——席位 Deliver 发出后进程即退，帧丢在 gRPC 传输缓冲（Send 返回≠冲上socket）：票停 已派、seat_run 显示 exit=0 却无 deliver 事件
一句话教训：无头席位收尾帧（Deliver/判决）必须等回执再退；gRPC Send 是把帧塞进 framer，不是落线

## 环境
meta 网关+`meta seat` 外壳，grpc-go unix socket 双向流

## 复现
spawn claude（SMOKE_OK prompt）→ 壳 5s 收尾 → shim `stream.Send(Deliver)` 返回 → `os.Exit(0)` → 网关侧事件流只有 claim/progress/requeue，无 deliver——快速壳必现（devin 慢壳因队列里还有帧排队得冲线时间而偶过）

## 解法/绕法
- `GatewayMsg` 加 `Ack` 帧：网关 handleDeliver 迁移落库后回执；shim 发 Deliver 后**有界等执**（3s 一次性闹钟）再退
- finishing 旗配套：Deliver 已发后流死不再进收养等待（票已交，无收养对象）
- 教训同族：`devin-sandbox-write-still-gated`（无头席位产出只信 stdout/回执，不信「调用了」）

## 引用
internal/seat/seat.go（finishing/ackCh）；internal/gateway/attach.go handleDeliver 回执段
