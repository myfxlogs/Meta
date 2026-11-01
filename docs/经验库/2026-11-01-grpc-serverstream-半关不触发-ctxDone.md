日期：2026-11-01
席位：施工（票0011）
现象：CeoGateway.Events 锁登记测试假死——客户端 `CloseSend()` 后服务端
`stream.Context().Done()` 永不触发，订阅登记录不除名
一句话教训：server-streaming 单向流里客户端半关（CloseSend）对服务端是
**不可见的**——EOF 除名语义只能靠 ctx 取消或整条连接断；测试用
`context.WithCancel`+cancel，生产径靠进程退/连接断，别指望 Recv 见到 io.EOF

## 环境

grpc-go v1.84，unix socket，server-streaming RPC

## 复现

`CeoEvents(Subscribe{follow=true})` 订阅登记锁——测试 `sA.CloseSend()` 后等
`ceoSub==nil` 超时。服务端 select 只盯 `ctx.Done()`——半关不产生任何
服务端可见事件。

## 解法/绕法

- 锁除名径＝`stream.Context().Done()`：客户端 cancel ctx 或关连接才响。
- 测试径：`ctxA, cancelA := context.WithCancel(...)`；`cancelA()`＝EOF 模拟。
- 生产径天然对齐：`meta ceo`/`meta ceo-events` 进程退→conn 断→ctx 散→除名。
