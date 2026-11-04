# T48 返修判决（2-CEO·2026-11-04 晨）

**驳回一项——reconcile 并发竞态**（锚测 TestT48_DeployFailedDedupe
在全包跑时红：首轮记 2 条而非 1——去重逻辑本身正确，但两个 goroutine
并发进 deployDyingTicketClose，同读期窗、各追加一条，竞态贯穿）。

**根因**：reconcileLoop 自身串行（同 goroutine 消费 ticker＋kick），但
直调径（AdminGateway.Reconcile RPC / 测试 `g.reconcile(ctx)`）可
与循环并发——无互斥。生产面同险：`meta reconcile` CLI 拨动时可与
kick 循环竞争。

**返修范围（只此一件）**：
`g.reconcile()` 入口加 `reconMu sync.Mutex` 串行化（Lock/Unlock 罩
整轮对账）——直调径与循环径从此互斥；不加 sleep 不改锚语义。

锚：全量（-p 1）TestT48_DeployFailedDedupe 稳定恰 1 条（三轮连绿）。
禁：R2′；deployDyingTicketClose 逻辑不动（去重已正确，缺的是并发壳）。
