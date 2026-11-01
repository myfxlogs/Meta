日期：2026-11-01
席位：施工（票0001）
现象：测试席位 spawn 后毫秒级即发 Adopt/Attach——unix socket 文件尚不存在，grpc 客户端**不重试直接 fast-fail** `Unavailable: no such file or directory`；席位侧同病理（网关重启间隙重连遇空窗即退）
一句话教训：grpc-go 对 unix ENOENT 不做 wait-for-ready 排队——「等对端 socket 出现」要自己有界重试链，不能指望客户端库兜住

## 环境
grpc v1.84.0，`grpc.NewClient("unix://"+absPath, insecure)`，libprotoc 无关

## 复现
helper seat 起 `net.Listen(unix, run/seats/<id>.sock)` 前，测试 `Adopt()` 一次调用即返回 ENOENT（不进连接退避）；worker 席位在网关 Close→New 间隙 `Send` 遇 socket 消失即错退

## 解法/绕法
测试侧 `adoptOnce` 外套有界重试循环（`time.After` 节拍，deadline 封顶——R2 边界条款的合法闹钟链，非轮询）；worker 席位重连亦走有界重试链且用 ctx 可撤流（Send 阻塞在 wait-for-ready 时靠 cancel 解）。网关侧收养**不重试**——单次尝试失败＝行动者判死，语义正确。

## 引用
票0001 验收锚 c/d/e/g；internal/gateway/helper_test.go `adoptOnce`/`waitSeatUp`/`attachAndHold`
