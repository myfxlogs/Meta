# 票 req-9ce5efb2e4ad：票→项目挂接线（需求→设计→拆票）

> 需求票回填票面（fill_spec）：设计票的八栏——审＝核设计档在、子票立、派发真。

## 八栏

### 1. 目标

需求「票→项目挂接线」转设计：`NewTicket.project` 字段落地＋立票/派发径
`SetTicketProject` 接线＋存量票回填项目列合法径（不破网关唯一写者）。
产出：设计文档 commit 入仓、本票八栏回填、子票立＋派、本票 Deliver。

### 2. 为什么

部署闸对一切票不可服务：`ticket.project` 全表空（设计-0007 §193 列明
`NewTicket.project` 未落、无生产写径），exec 闸「无配方登记」必拒；
闸前十张 `待部署` 票（0003–0012，码已实际并入主干并在线）因无挂接
永不到完成；空 project 使项目锁退化为全表串行、设计锁对空票失效。
前置提醒：check 步 `sudo -n -u metaseat` 仍待业主
`/etc/sudoers.d/meta-runner` 行。

### 3. 范围内

- 设计文档 `docs/实现/设计-req-9ce5-票挂项目.md`（挂接矩阵/五判闸/
  自举序/边界）commit。
- 子票票面 `docs/实现/票-0015-票项目挂接.md`（八栏全量＋预算＋依赖）
  commit，经 `META_ADMIN:create_ticket` 立票。
- `META_ADMIN:dispatch` 派子票→施工（注入机未开期带提示文件）。
- 本票 `META_ADMIN:fill_spec` 回填（本文件）＋`META_DELIVER` 交付。

### 4. 范围外（禁线）

- 实现码（设计票不施工——施工链全在子票）。
- 积票部署批闸、per-project 配方注入、O 项目登记、sudoers 行代办。
- 存量票回填执行（部署后管理操作，设计 §4 钦定序——非本票动作）。
- 多票拆分（单内聚功能一票到底；拆两半皆不可独立验收）。

### 5. 设计引用

`docs/实现/设计-req-9ce5-票挂项目.md`；`docs/总设计.md` §6.1/§10/B1/B2/
D1/令#E17；`docs/实现/票-0010-部署闸.md`；`docs/实现/ceo_memory.md`
决策 63（闸前批手工部署＋本票立票经过）。

### 6. 验收锚（机器可判）

1. `docs/实现/设计-req-9ce5-票挂项目.md` 在仓且含：挂接矩阵、
   `attachProject` 五判、自举序三节——`grep` 断言三节名在。
2. `docs/实现/票-0015-票项目挂接.md` 在仓且 `inject.ParseTicketMD`
   解析过（八栏零缺）。
3. DB 中票 `0015` 存在且 spec 非空（`meta phase/events` 可查）。
4. `0015` 有 dispatch 事件与派发动作（事件史含 `dispatch`/`spawn` 行）。
5. 本票 spec 已回填（八栏零缺——`inject.CheckSpec` 过）。

### 7. 验证命令

```bash
grep -c "attachProject\|挂接矩阵\|自举序" docs/实现/设计-req-9ce5-票挂项目.md && \
meta phase --socket run/meta.sock --ticket 0015 && \
meta events --socket run/meta.sock --ticket 0015
```

### 8. 交付形制

设计票交付＝本票 Deliver（ref＝设计文档 commit hash）；摘要四节
（新增/修改面、根因、验证引、下一步）；收口两步门（归位扫描＋工作区净）。

## 附

- 优先级 P0；依赖：无；触发：requirement 事件（provenance=terminal）。
- 子票 0015 预算：施工席缺省。

## 判决史

（空——首判前无）
