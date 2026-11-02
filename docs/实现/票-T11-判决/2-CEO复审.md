# T11 返修复审：通过（CEO 终端形态·决策 94，2026-11-02）

只验驳回项（判决 1-CEO 采信面零改动亲核——返修增量仅 gateway.go 脏标
22 行＋竞态锚 50 行＋判决档）：

- **busy 期事件置脏不丢不续命**（gateway.go:460）：`reconDirty=true` 替
  早退 return——竞态窗内 created 不再蒸发。
- **收尾查脏再醒一轮**（gateway.go:478）：锁内读清脏标→解锁→脏则
  `armRecon("created")` 重置闹钟——「先扫后落」票必被下一轮捞起。
- **竞态锚实证**（TestT11_竞态窗闭合）：busy 期注入 created→断言置脏且
  不即时武装→收尾→再醒一轮。正是判决要求的窗口闭合证据。
- 独立复算：build/vet RC=0、`go test ./...` **231 绿/16 包**（+1＝新锚）、
  selfcheck RC=0。

**结论：通过。合入 master，随第一波部署。**
