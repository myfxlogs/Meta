日期：2026-11-02
席位：施工（票0011·接替）
现象：测试用 Attach 流代行 claim/deliver，票相位却报「非法边: 施工中
-[claim]-> 施工中」断流——同票第二个 claim 到达；2/3 概率红，纯竞速
一句话教训：**代行协议帧前，先核 spawn fixture 的 SEAT_MODE**——worker
helper 自己会 Attach+Claim，与代行流双 claim 竞速，输家被非法边断流

## 环境

meta 网关测试（internal/gateway）；`dispatchSpec()` fixture 钉死
`SEAT_MODE=worker`，测试又 `t.Setenv` 改不了 spec.Env（spawn env 是
os.Environ()+spec.Env 拼接，spec.Env 在后）

## 复现

DispatchSeat(spec=worker helper)→helper 自动 claim→测试流再发 Claim→
后者撞「施工中-[claim]->施工中」FailedPrecondition→后续 Deliver 帧随
断流丢失→放锁/事件断言超时。间歇性＝谁先到谁赢。

## 解法/绕法

代行场景把 spec.Env 里的 `SEAT_MODE=` 行**原地改写**为 idle（append 同名
env 会有重复键，libc getenv 取先见者，不可靠）：

```go
idleSpec := dispatchSpec()
for i, e := range idleSpec.Env {
    if strings.HasPrefix(e, "SEAT_MODE=") { idleSpec.Env[i] = "SEAT_MODE=idle" }
}
```

同族判据：一切「测试侧伪造席位协议」的测试，spawn 的 helper 必须是
不发协议的形态（idle/hold），否则被测件收到的是两份交错帧。
