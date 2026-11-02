# 票 0038-p1-p3：rollback_cmd 修而未满——DB project 行旧 stub 复发半合并脏树

> 巡检席自立案（需求票）。spec 经 `meta ticket new --spec-file` 入库在前；
> 本文件为需求原文补落——立案时主仓半合并态暂挂，树净后由 CTO-A 设计席
> （run 65c12b70c2e57141）按票面「待树净后补落 docs/实现/」记补落。
> 触发事件：merge_conflict 事件巡检（事件 15805；巡检 run=
> 4cf660c4f64846a8，provenance=gateway 巡检派发）。

### 1. 目标

- DB `project.rollback_cmd`（id='meta'）同步为 MERGE_HEAD 分径版，
  文面对齐 `config/gateway.textproto:40` 现役版。
- 立「配置↔DB 配方行」漂移对账：project 行全部 *_cmd 字段
  （check/deploy/probe/rollback/integrate）与 textproto 同名段逐比对，
  同漂者并修，产出漂移对账表落档。
- 运行时真源归一：明记（码注释或文档）部署步执行以 DB 行还是配置
  文件为准，消除双源静默漂移——方向裁量归施工/CTO-A，本票只要求
  二选一消灭歧源。
- 本轮已残留半合并脏树（MERGE_HEAD=90da461，UU×2）获处置决——
  随 0038-p1 deploy_failed 判决流，merge --abort 裁 CTO/施工。

### 2. 为什么

- 事件 15804：integrate 步 exit=1，`t/0038-p1-selfcatch-card`
  (90da461) 对 master (e28a1a4) 三方合真内容冲突——
  `internal/feishu/card.go`、`roles/施工.md` 两处 CONFLICT。
- 事件 15806：rollback 步 exit=0 但输出旧文案
  「本部署无破坏可回（merge已并历史不回退；二进制回退由upgrade自检
  兜底）」——既非新分径文案「半合并态已弃净」亦非「无 MERGE_HEAD
  残留——显式告警」。亲验 `.git/MERGE_HEAD`=90da461 仍在、
  `git status` UU×2：半合并脏树实存，rollback 假成功复发。
- 修复在案而落空：D-78（台账标 done）/0041（6acea3c 判决通过，
  f5c1ce7 施工）把 MERGE_HEAD 分径 rollback_cmd 落进
  `config/gateway.textproto`（71c5cc7 随 T4 并主线，HEAD 版已含）。
  但亲验 `sqlite3 meta.db "SELECT rollback_cmd FROM project"`＝
  旧 stub 原文。网关进程 19:27:13 重启（载新配置新二进制）后rollback
  仍跑旧文案——**运行时真源＝DB project 行，非配置文件**；部署席
  afc64153b3c70f0e 自身 atif 亦独立撞出同一结论。
- 后果＝D-78 原病理原样复发：半合并脏树无主，拦主仓一切 git 操作；
  当前待部署=7（batch_deploy 事件 15597/15754/15839），后续 integrate
  全撞「merge in progress」面。

### 3. 范围内

- `project` 表 `rollback_cmd` 字段改 MERGE_HEAD 分径版
  （`git rev-parse -q --verify MERGE_HEAD` 有 → `merge --abort`＋
  rollback_aborted 事件；无 → 显式告警文案）。
- project 行五 cmd 字段 × textproto 同名段全量对账：列漂移表，
  同漂者本票并修；不漂者表内记零。
- 真源归一明记：施工裁量定「DB 行即真源（配置行废/仅种子）」或
  「改码回读配置」任一，落注释/文档消歧。
- 残留脏树处置决随 0038-p1 deploy_failed 判决流走（裁 CTO/施工，
  含 merge --abort 或续解冲突二径）。

### 4. 范围外（禁线）

- 不动决策81「不回退已并历史」原则——merge --abort 弃的是未完成
  合并，非历史回退。
- 不代解 0038-p1 冲突本体（card.go/施工.md 撞面归 deploy_failed
  判决流→施工支 rebase 重出）。
- 不动 B1 项目部署锁面（归 0038-p1-p2，施工中在案）。
- 巡检不手改主仓工作树——不立票外零写（本票spec经 --spec-file 入库，
  md 落盘因主仓半合并态暂挂，待树净后补落 docs/实现/）。

### 5. 设计引用

- `docs/实现/票-0034-p1-p2-rollback对冲突态假成功.md`（病原票——
  作废并入 T4，其验收锚只覆配置文件未覆 DB 行）。
- `docs/实现/设计-0034-p1-p2-rollback分径.md`；债务台账 D-78 行。
- `config/gateway.textproto:40`（新 rollback_cmd 目标文面）。
- 0041 交付链：f5c1ce7（rollback_cmd 按 MERGE_HEAD 分径＋
  rollback_aborted 事件）→ 6acea3c（判决通过）→ 71c5cc7（T4 并主线）。
- 决策81（ORIG_HEAD 陈旧指针错伤）；决策92（补票链深闸——本票与
  p2 同层并列）。
- 部署席 atif 独立旁证：`run/artifacts/afc64153b3c70f0e/atif.json`
  （其 sqlite 亲验行与本票同源互证）。

### 6. 验收锚（机器可判）

- `sqlite3 run/meta.db "SELECT rollback_cmd FROM project WHERE
  id='meta'"` 输出含 `MERGE_HEAD` 分径，与 config 文面语义一致。
- 对账表落档：project 行五 cmd 字段对 textproto 漂移项=0，或每项
  漂移有显式裁决记录。
- 复演锚：冲突中断态 rollback 实跑后
  `git rev-parse -q --verify MERGE_HEAD` 解析失败（树净），且 exec
  档输出新文案（「半合并态已弃净」或等价）；无 MERGE_HEAD 场景
  输出告警文而非「无破坏可回」旧文。
- 本轮残留：`.git/MERGE_HEAD` 不存、`git status` 无 UU——以
  deploy_failed 判决流处置记录为凭。

### 7. 验证命令

```bash
sqlite3 /opt/meta/run/meta.db \
  "SELECT rollback_cmd,integrate_cmd,deploy_cmd,check_cmd,probe_cmd FROM project WHERE id='meta'"
grep -nE "rollback_cmd|integrate_cmd|deploy_cmd|check_cmd|probe_cmd" \
  /opt/meta/config/gateway.textproto
git -C /opt/meta rev-parse -q --verify MERGE_HEAD
git -C /opt/meta status --short | grep -c UU
/opt/meta/bin/meta events --socket /opt/meta/run/meta.sock --ticket 0038-p1 | tail -8
```

### 8. 交付形制

票面八栏全量入库（本文件经 `meta ticket new --spec-file`）；施工后
DB 行分径自证（exec 档 abort 型实跑＋echo 告警型各一）；漂移对账
表落档（docs/实现/ 或经验库）；Deliver 摘要四节（新增/修改面、根因、
验证引、下一步）。

## 附

- 优先级 P1：主仓半合并脏树拦一切 git 操作，且待部署=7 排其后——
  每续试 integrate 皆撞「merge in progress」面。
- 依赖：0038-p1 deploy_failed 判决流（冲突本体与脏树处置归彼）；
  无码面依赖 0038-p1-p2。
- 触发：merge_conflict 事件巡检（事件 15805；本席 run=
  4cf660c4f64846a8，provenance=gateway 巡检派发）。

## 判决史

（空——首判前无）
