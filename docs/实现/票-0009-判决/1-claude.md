# 票 0009 判决档 1（壳：claude · 首判）

- 交付支：`t/0009-dispatch-guards` @ `c1a1d91`（base master `8c35293`；已推 origin，
  `git ls-remote` 远端支头＝本地 `c1a1d91e85f…`——实测）
- 施工自述机器记录：`/tmp/t0009.machine-record.txt`（mtime 2026-11-02 00:00:52，
  **晚于** commit 23:43:52——commit 后验证态，tee 原文＋pb 再生＋三变异＋selfcheck）
- **全部锚由本席在交付 worktree 独立复算**（先例 0008 判决 2 标准），不只信
  施工记录；本席复算记录 `/tmp/t0009-audit-rerun.txt`。

## 逐锚对账（票面 §6 七锚）

| 锚 | 机器证据（本席复算） | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 本席跑 `go build ./... && go vet ./...`→BUILD_VET_OK；`go test -count=1 ./...`→13 包全 `ok`（gateway 13.6s，rerun 原文在档）；`bash scripts/selfcheck.sh`→EXIT 0。与施工 tee 记录一致 | 绿 |
| 2 role 过滤 | `caster.go:220` `if rt.Role != "" && rt.Role != row.Role`（与 kind/note_contains 同位串行）。`TestRoleFilter`：deliver{role:施工}→dispatch 审计＋prompt 模板渲染含票号；deliver{role:CTO}→零 dispatch 只 notify；空 role 行（ping）全匹配回归；multi 行 role∧note_contains 双条件缺一不动（m1 施工+no、m2 CTO+go 均零动作，m3 施工+go 动作） | 绿 |
| 3 空提示拒起 | `dispatch.go:196-197` 闸位于覆盖径赋值后、`shellsFor` 前（票面钦定位）。`TestDispatch_空提示拒起` 四子径全断言 spawn_blocked（note 含「空提示拒起」）＋seat_run 零新行＋failSet 空＋健康零迁移：①PromptFile=""+InjectRoot 未启；②指不存在文件；③零字节；④InjectRoot 置位＋mock `assemble` 返 ""（`inject.go:16` 包级缝）。`promptFileEmpty`（dispatch.go:410）＝空路径/读不出/全空白三态机械判 | 绿 |
| 4 席位级死因 | `seat.go`：`ExitInputErr=5`（:35）＋`codeInputErr=-2` 哨兵（:42）；`runChild` Build 失败/prompt 不可读两分支由 `-1` 改返哨兵（:188/:196）；`run()` 哨兵→`ExitWrapper` 归级（:140-147）；`ClassOfExit` 2/3/4/5→ExitWrapper（:523-527）。`TestDispatch_席位级死因禁标壳` 四退码各落 `seat_err`×3（三候选各一）＋failSet 空＋健康未动；`TestRunChild_输入层` prompt 不可读与 Build 失败均返哨兵；`TestRun_缺env`→ExitSeatErr | 绿 |
| 5 壳级回归 | `TestDispatch_壳级回归`：①61→全行进 failSet＋健康不迁＋零健康事件；②a 62→limited＋`shell_limited`；②b 63→dead＋`shell_dead`。`route_test.go` 增 `NextHealth` ExitWrapper 两行（OK/DEAD 均不迁）——壳级三行原语义未动 | 绿 |
| 6 配置 | `TestProdConfig`：仓内 `config/caster.textproto` 加载绿；deliver 拆两行（CTO 行断言零 dispatch 动作，施工行 dispatch）；审计 dispatch 三行（deliver/audit_dead/spawn_needed）全带 `prompt_template:"caster/audit-review.md"`；模板存在且含 `{{ticket_id}}`/`META_VERDICT`/`meta events`/`meta phase`，render 后票号消化 | 绿 |
| 7 对抗锚 a/b/c（本席独立变异复算，cp 备份回退，还原后 `git status --porcelain` 均 0 行） | a. caster.go:220 前置 `false&&`→`TestRoleFilter` FAIL（`Dispatch=5 预期 3`——CTO 又派审计）；b. dispatch.go:197 前置 `false&&`→`TestDispatch_空提示拒起` **四子径全红**（tg1-tg4 spawn_blocked 超时——空席被起）；c. dispatch.go:378 前置 `false&&`→`TestDispatch_席位级死因禁标壳` **四退码全红**（`seat_err 行数=0`——席位级进 failSet 冤标壳）。与施工机器记录三变异结论逐条一致 | 绿 |

## 范围与形制

- 范围内八件全中：proto/pb、caster process 闸、dispatch 空提示闸、inject.Assemble
  兜底、seat/adapter 席位级分级、caster.textproto 拆行、audit-review.md、单测。
- 范围外文件逐件核（非蔓延）：
  - `attach.go` +4-2＝`onRunDead` 调用点适配 reportShell 新签名（加 `st` 参）；
  - `gateway.go` +1＝`runState.failReported` 字段——**伴随修正**：trySpawn 秒退窗径
    与 onRunDead 判死径对同一死并发双报，原可双记；ExitWrapper 双径必遇，去重
    为「只落一次 seat_err」语义所必需，壳级语义（锚 5）经回归测试证实不动；
  - `dispatch_test.go`/`t0004_test.go`/`inject_test.go` fixture 由 `/dev/null` 改
    非空 prompt——D-40 闸后零字节必被拒起，原有「正常派发」测试意图的机械必要
    适配，非改测试迁就实现（覆盖径断言原样保留）；
  - `route_test.go` +2＝新分级入表驱动；`helper_test.go` +7＝exitcode 测试模式；
  - `docs/经验库/2026-11-01-…git-checkout吃未提交改动.md`＝令#E1 一坑一文授权产出。
- 形制：支名/commit `t0009: <为什么>`/已推远端 ✅；交付摘要**五节**（修改面→文件、
  根因、验证→引机器记录「勿据自述」、下一步全量遗留申报、可沉淀候选）＋收口
  两步门（事实归位扫描/工作区净）＋`META_DELIVER` 哨兵——席位日志尾部实录在案 ✅。

## 质量三问（令#E5）

- **最优解**：role 闸复用 process() 既有串行闸位、席位级复用 ExitClass 枚举＋
  ClassOfExit 映射表，均无新间接层；未见更简等价解。
- **第一性**：D-40 打在「最终 PromptFile 判空」机械可判点（空/缺/零字节/产物空
  四态同径）；分级账本而非调用点散布 if——直击本质，无补丁摞叠。
- **洁净**：无同逻辑两处、无死码、无调试残渣；diff 零 TODO/FIXME/绕过标记
  （grep 唯一命中＝审计模板指引文字本体）。

## 债项节（令#E6 对账）

| 项 | 状态 |
|---|---|
| D-0008a（设计-0007 滞留 c/0007-design，master 工作树至今无此文件——本席实测 `ls docs/实现/｜grep 设计` 仅 0004） | **上游已申报在案**（t0008 返修支台账 D-0008a；对象 9321304 主仓可达，并否归 CTO 裁）。本票票面引用栏受染，但八栏自足机械可判＋现场码六件全在 master，施工全程未受阻（席位日志/经验库无受阻痕迹）——不累及本票，挂账待 CTO |
| 摘要「下一步」三件（票 0010 部署闸、后续批次、config 部署同步归 CTO-B/运维） | 全票外正当申报，与 diff 零 TODO/零半成品对账一致——**无瞒报**。部署备忘票面 §8 本就归运维径 |
| reportShell failReported 去重 | 伴随修正（见范围节），行为方向＝消除双报竞态，锚 5 回归绿证不伤壳级语义 |

## 可沉淀意见（令#E8 · 纯意见不构成否决）

摘要三条候选（席位级死因单一 ExitWrapper 分级、空/零字节 prompt 机械闸、路由
role 等值过滤空＝不滤）——**可沉淀**：三者皆「机械可判、不依赖人判」形制，与
既有退码表/禁式闸同构，且本案三变异实测红证明提名行可对抗验证。

## 结论

**通过**。七锚独立复算全绿（含对抗锚 a/b/c 本席突变实测红、pb 再生 byte-identical
——protoc v3.21.12 再生后 `git status --porcelain` 零行）；范围净、暗债零瞒报、
形制全；0007 连环事故三环（role 误派→空提示→误标壳）各有机械防线且互为犄角。

META_VERDICT:pass:docs/实现/票-0009-判决/1-claude.md
