# 票 0034 判决：通过（审计席 devin/SWE-2-Max，2026-11-02，首判）

- 交付物：支 `t/0034-patrol-filter-spawnfix` @ `e109692`（已推 origin），
  commit `t0034: 巡检链两缺——过滤壳死因分类＋patrol-* 派发兜底`，
  diff +354/-17 共五件全在票面许可面（patrol.go/dispatch.go＋对应 _test.go）。
- 机器记录：施工未落 `/tmp/t0034*.machine-record.txt`；席位实录
  `run/artifacts/38cbaa5925ec1eae/atif.json` 各令退码＋输出原文在
  （build 静默绿、锚测 9 passed、全量 119 passed——T31 首红摘帽后绿、
  push/sentinel 退码 0）。**本席在交付 worktree 原命令独立复算**
  `/tmp/t0034-audit-recompute.txt`；对抗锚三突变在 scratch
  `/tmp/t0034-mut` @e109692 复算（已拆）。

## 逐锚对账

| 锚 | 机器证据（diff 行/复算输出） | 判 |
|---|---|---|
| 1 build/vet/gofmt/test | 复算：BUILD_RC=0、VET_RC=0、`go test ./internal/gateway/` `ok 24.687s` 全绿、selfcheck RC=0。**gofmt 字面零命中不达**：`gofmt -l internal/`=6 件（adapter/claude.go、caster.go、db/msg.go、db/phase_test.go、deploy_test.go、gateway.go）——实查 master 基线 9 件含 dispatch.go/patrol.go/t0030_test.go（stash 复跑实证 step17），本交付范围内五件零命中且**净减 3 件**；余 6 件系范围外票外旧债，修之则范围蔓延（票0010/0011 同款先例记观察不拦） | 绿（注记） |
| 2 ClassifyExit 表驱六 fixture | `-v` 复算：超时被杀/限额stderr/限额JSONL/起壳败/退码非零 5 fixture 全 PASS＋quotaLike 正反例段（7 正 4 反）＝第六面齐 | 绿 |
| 3 QuotaUnjudgeable | PASS——错误同时含「限额」＋「输出不可判」语境＋`tail=` 净化尾三要件（t0034_test.go:117-126 断言实核） | 绿 |
| 4 PatrolDutyFallback | PASS 两段：①InjectRoot 空→patrol_tick→立票→`shells_exhausted`（非 spawn_blocked 空提示）＋兜底件落 `run/inject/duty-*.md` 含票ID/META_DELIVER/信号原文；②InjectRoot 开但组装败→`inject_failed` 照落后续走兜底抵壳路由。`TestT30_PatrolRoute` PASS | 绿 |
| 5 对抗锚三提名行 | 本席 scratch 突变复算：删超时分支→`超时被杀` fixture 红（got `退码非零 exit=-1`）；翻 quotaLike 判定行→限额误分类＋反例红；删 dispatchOnce 兜底支→锚4 红（`shells_exhausted` 15s 超时不至＝回 D-40） | 绿 |
| 6 手验 meta inject | 部署中二进制＝master 旧码，兜底径物理不可活体验（现网 `--inject-root` 已在位）；锚4 同径全 fixture 覆盖，施工如实申报「部署后复验归并码票」 | 待部署后验（非否决） |

## 范围/禁线核对

- 五件全在票面「只碰 patrol.go/dispatch.go 与对应 _test.go」内。
  `t0031_test.go` 落名外——但与 T30 同性质缺陷期期望摘帽，锚1 强制
  （不改必红：atif step22 实测 `TestT31` FAIL），摘要「下一步」已申报。
- 禁线实核零碰：D-40 闸本体未动（dispatch.go:296+ 原样在）；「壳败＝放行
  全开＋filter_degraded」策略未变（三错误点仍 return error→全开）；
  哨兵契约/handleAdmin/订阅 kind 表/caster 路由/filter_shell 配置值/
  adapter 未碰；无定时器/轮询/心跳（selfcheck RC=0）；无 JSON/REST/新三方
  依赖（go.mod/go.sum 未动）；未并 master。

## 交付形制

支名符票、commit `t0034:` 前缀、已推 origin、worktree 位符、
`$META_ART_DIR/sentinel.log`＝`META_DELIVER:e109692`（stdout 同发）、
交付摘要五节齐（新增/修改面→根因→验证引 atif→下一步全量申报→可沉淀）。

## 暗债对账（令#E6）

diff 内零 TODO/绕过/半成品（grep 实证）。摘要「下一步」申报四项：
T31 落名外摘帽／gofmt 基线 9→6 件／锚6 部署前不可验／quotaLike 启发式
签名集——逐条与 diff＋机器记录对上。**瞒报零。**

## 质量三问（令#E5）

- **最优解**：兜底支放 `dispatchOnce` 单点（注入段后 D-40 前）覆盖
  routeSignal/caster 对账补发/手工 `meta dispatch` 全派发径——比在
  routeSignal 局部补更全更简；`newAdapter` 包级缝＝inject.go:17
  `assemble` 同款先例；`patrolDutyPromptFile` 复用 `patrolFile` 落盘径
  与 patrolPromptFile 同族。无更简等价形制可指。
- **第一性**：note 即诊断面——错误原文进 `filter_degraded` 处加分类词
  直击「裸 killed 不可分死因」根因，非 workaround 摞补丁；兜底件合成在
  派发漏斗一处而非多点散补。
- **洁净**：`outTail`（串内截断＋clean 净化）与 `logTail`（文件尾读）同族
  不同域非重复实现；新增函数全部被调用/被锚覆盖，无死码无调试残渣；
  注释与实现一致（对抗锚提名行亲验属实）。

## 债项

无新债入。基线 6 件 gofmt 旧债（上列六路径）留档归债务台账销号面，
非本票产出。

## 可沉淀打法（令#E8 附议）

「缺陷期期望摘帽」：修缺陷改变行为时 grep 全套 `_test.go` 找断言旧行为
者同步摘帽——**可沉淀**（机械可做；本案 T31 漏摘即红实证有效）。

## 结论

六锚五绿一待部署（物理不可验已申报）、范围零蔓延、禁线零碰、暗债零
瞒报、质量三问无指摘——**通过**。
