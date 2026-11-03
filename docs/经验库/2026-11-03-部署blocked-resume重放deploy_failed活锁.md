# 经验：部署中票 resume 必回 blocked——deployDyingTicketClose 每轮重放 deploy_failed（确定性活锁）

> 令#E1 一坑一文。发现：票 0050 CTO 部署席（本席 eed51ecd7e53d254）
> 亲证——resume 四次全灭，末次与本席 spawn 同毫秒竞撞。

## 现象

部署中票进 blocked 后，人工 `meta ticket resume` 回部署中——
**约一轮调和（≤60s）内必再 blocked**，无一例外。票 0050 实证：
18:35:32 resume → 18:36:26 deploy_failed×2 → blocked，全程 54 秒。

## 根因链

1. `deployDyingTicketClose`（reconcile.go:359）在「相位=部署中 且
   无活 run」时每轮调和都跑：死席 `drainDeploySentinel`="" 且
   `deployStepsDone`/`deployStateOK` 俱假 → **无条件再 AppendEvent
   一条 deploy_failed**——失败事件是电平触发（死席无哨兵是常驻态），
   记账却按轮次计次。
2. cap=1（deploy_failed→rollback 行）：resume 后调和第一轮补记
   deploy_failed(n=1)→rollback 动作→DispatchSeat 排队起席；
   deploy_failed emit 自 kick → **第二轮（毫秒级）再补一条(n=2)>cap
   →blocked**。spawn 行落库要 ~20ms+，永远追不上第二轮。
3. 净效果：部署中票一旦 blocked，resume 数学上救不回——每次放行
   只值一轮调和的命。T42「ruling/resume 行入期窗锚点」清了旧账，
   但**新轮次新记的新账**不归锚点管。

## 解法/绕法

- **绕法（本席实操）**：部署席在席待命——resume 时票上已有活 run
  →reconcileTicket 早退 aligned→零重放→席位 META_EXEC 四步收口。
  即：先有一席活着，再放行。
- **正修法（未做，记 CTO 侧债）**：deployDyingTicketClose 的
  deploy_failed 记账要去重——同票同期窗已记过「死席无据」账则
  不再追加（电平→边沿化），或仅对期窗内新死的 run 记账。

## 引用

- 票 0050 事件 21230–21234（resume→deploy_failed×2→blocked 全程）
- internal/gateway/reconcile.go deployDyingTicketClose:359-385
- 失败表 `{deploy_failed, rollback, cap=1}` gateway.go:578
