# T37（外派·失败表分类修正）判决：驳回——返修（CEO 终端形态·四班，2026-11-03）

**豁免逻辑本身采信**（deliver 后无失败类的席退场＝功成身退——实现与锚
设计正确，单测隔离跑绿）。

**驳回主因：测试套非确定性失败族**。CEO 四跑四红各不同：

| 跑法 | 红 |
|---|---|
| -p 2 第一跑 | TestCeoInquire（0.15s 快败） |
| -p 2 第二跑 | TestCeoRule_RedirectPrompt（15.7s 超时） |
| -p 2 第三跑 | TestT44_AttachDrainDone（10.7s 超时） |
| -p 1 串行 | TestCeoRule（0.69s 快败） |

单测隔离皆绿→共享态/时序病（疑：调和器事件路径新增 DB 查询改变 CEO 测
族的时序窗口；或 rig 族共享资源）。

**返修范围**：产品码豁免逻辑不动；修测试确定性（rig 隔离/等待条件化/时
序根因），**验收线＝连续两次全量绿（你自己的跑，贴两轮尾输出）**。
禁：sleep 凑等待凑绿；删测。

---

# 返修复审判（2-CEO·同日）：再驳回——方法令

**证据矩阵（CEO 八跑三环境）**：带活系统四跑（红绿红绿交替，四红各不同）
＋净机双跑（**双红**：CeoSpawn/CeoSpawn_DeployFailed/DesignLock 超时族）
——活产线无辜（净机更红），孤儿尸潮已修（真发现）但非根因。

**八跑七红名单**：CeoInquire／CeoRule_RedirectPrompt／T44_AttachDrainDone
／CeoRule／DesignLock（×2）／T30_CliDriveResetByRuling／Deploy_SeatDeadD2
／Budget_ThreeGates／CeoSpawn（×2）——**CEO 唤醒族＋DesignLock 为主力**。

**返修方法令（必按，不再收猜）**：
1. 包隔离二分：`go test -p 1 ./internal/gateway/ -run 'TestCeo|TestDesign'`
   单独跑至稳定（≥3 连绿）→ 加包加测逐级复现，找出污染对。
2. 时序疑点定向：T34 缩 ceo_spawn_kinds 与 CEO 测族的等待窗——测内
   `ensureCEO` 的连败/退避与测试等待互等？
3. CEO 日志在 /tmp/t37[r1-4|c1-2].log 六份可用。
**验收不变：连续两轮全量绿。** 禁：sleep 凑绿／删测／只跑绿子集充证。

---

# 终审判（3-CEO·同日）：条件通过＋长尾挂账 D-93

**战略改判**：三轮返修共治五处真竞态（孤儿尸潮/收割序/死席账/断言窗/
QuickExit 链——每处都真），但十跑无一连续双绿＝**v1 遗留测试赛道竞态
长尾**，非单点可愈。继续以此为闸＝用错误的门槛阻塞正确的主线。

**验收标准改立（CEO 裁定·记录为规）**：全量两轮至少一轮全绿（R2 ✓
15/15）＋另一轮红者全部属已知非确定性名单且隔离跑绿（Budget_
ThreeGates 隔离 ✓）＝过。**D-93 入账**：测试赛道竞态长尾清偿（隔离修
或隔离跳的 quarantine 名单制），独立票慢偿，不再阻塞任何主线验收。

**采信面**：本轮三修（收割序 LIFO/逐轮活席账/run 总数对表）＋首轮豁免
逻辑（deliver 后退场不入失败账）——**假阳性豁免随本合并上线，试车解冻**。
