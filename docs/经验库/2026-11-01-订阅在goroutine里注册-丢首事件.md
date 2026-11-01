日期：2026-11-01
席位：施工（票0011）
现象：`go g.ceoPump()` 里才 `subscribe()`——New 返回后立刻 emit 的事件
在订户口注册前发出，观察者集合快照不含泵道，首事件永久丢（无头化身不起）
一句话教训：**先订阅后起泵**——channel 注册必须在 `go` 之前同步完成，
goroutine 调度时序不是注册顺序

## 环境

meta 网关事件泵（emit→observers 快照分发）

## 复现

`New()` → `go g.ceoPump()`（泵内才 `g.subscribe()`）→ 测试紧接
`g.emit(spawn_kind)` → emit 遍历 observers 快照时泵还没注册 →
事件只进了 g.events 主道，泵道未见 → CEO 无头唤起测试 15s 超时。

## 解法/绕法

```go
go g.ceoPump(g.subscribe()) // New() 里同步注册，goroutine 拿现成道
```

同族判据：一切「注册在消费者 goroutine 里」的事件订阅都有这个首窗口——
注册动作永远放调用方同步段。
