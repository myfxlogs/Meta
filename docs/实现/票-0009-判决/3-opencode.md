# 票 0009 判决档 3（壳：opencode · 终判）

- 交付支：`t/0009-dispatch-guards` @ `be097e5`（交付 commit `c1a1d91e85f19bcfae6b2d13aa7ea50ad64a39f8`；
  base master `8c35293`；**未并 master**；推 `origin` 实测远端支头＝本地同值 ✅）
- 施工交付（触发源）：run `60da4b418d589afb`（opencode，exit=0；deliver 事件 238）→
  交付 commit `c1a1d91`（`t0009: 守卫簇——0007 连环事故三环机械防线收口（D-39/D-40）`）
  → `META_DELIVER:c1a1d91e85f19bcfae6b2d13aa7ea50ad64a39f8`
- 承接链：判决 1（claude，events 240–258，席死）→ 判决 2（devin，events 259–270，席死）
  → 本档（opencode，events 271–…，终判）。三席对同一交付的结论**一致**：
  全绿，无驳回返修压力。

## 逐锚对账（票面 §6 七锚）

| 锚 | 机器证据（交付 diff 行＋机器记录＋交付 run 日志） | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 交付 run 日志尾部：`go build ./... && go vet ./... && go test -count=1 ./... 2>&1 \| tee /tmp/t0009.machine-record.txt && bash scripts/selfcheck.sh` → `FINAL_EXIT=0`；machine-record：13 包全 `ok`／`cmd/meta` 0.131s … gateway 13.724s 全绿／pb `[no test files]`／`protoc v3.21.12 + protoc-gen-go v1.36.12` 再生 → `PB-REGEN-BYTE-IDENTICAL`／`git status --porcelain` 零行／selfcheck `EXIT 0`／三变异回退后全量重跑全绿 | **绿** |
| 2 role 过滤（D-39） | `internal/caster/caster.go:220` `if rt.Role != "" && rt.Role != row.Role → continue`（kind 闸后、note_contains 闸前同串行闸位）；`internal/caster/caster_test.go:376 TestRoleFilter`：deliver{role:"施工"}→dispatch 审计＋prompt 模板渲染含票号；deliver{role:"CTO"}→零 dispatch 只 notify；空 role 行回归全匹配；role∧note_contains 双闸；交付 config 拆两行（`deliver role:"CTO"` notify-only／`role:"施工"` notify+dispatch 审计） | **绿** |
| 3 空提示拒起（D-40） | `internal/gateway/dispatch.go:196-197` 闸位于注入段之后、壳路由之前；`promptFileEmpty`（:410）三态机械判（空路径/读不出/全空白）；`internal/gateway/t0009_test.go:53 TestDispatch_空提示拒起` 四子径全断言 spawn_blocked（note「空提示拒起」）＋seat_run 零新行＋failSet 空＋健康零迁移（①注入未启／②文件缺／③零字节／④ mock Assemble 返 `""`）；`internal/inject/inject.go:130-140 Assemble` 产物空兜底 | **绿** |
| 4 席位级死因（D-40） | `internal/seat/seat.go`：`ExitInputErr=5`（:35）／`codeInputErr=-2` 哨兵（:42）／`runChild` Build 失败／prompt 不可读两分支返哨兵（:188/:196）／`run()` 哨兵→`ExitWrapper`（:140-147）／`ClassOfExit` 2/3/4/5→`ExitWrapper`（:523-527）／`ExitCodeOf(ExitWrapper)=ExitInputErr`（:519）；`internal/gateway/dispatch.go:378-389 reportShell` 席位级早退：只落 `seat_err`（壳名＋退码＋`logTail` 尾摘要）即返，不进 failSet、不调 `NextHealth`；`internal/gateway/t0009_test.go:147 TestDispatch_席位级死因禁标壳` 四退码落 `seat_err`×3＋failSet 空＋健康未动；`internal/seat/seat_test.go:37/49 TestRun_缺env`／`TestRunChild_输入层` 绿 | **绿** |
| 5 壳级回归 | `internal/gateway/t0009_test.go:182 TestDispatch_壳级回归`：①→failSet 全行／不迁健康／零健康事件；②a（Transient 62）→`limited`＋`shell_limited`；②b（Fatal 63）→`dead`＋`shell_dead`；`internal/route/route_test.go` 新分级入表驱动；壳级语义（ExitSpawnFail/Transient/Fatal）一字未动 | **绿** |
| 6 配置 | `internal/caster/caster_test.go:453 TestProdConfig`：生产 `config/caster.textproto` 加载绿；deliver 拆两行（CTO 行断言零 dispatch 只 notify；施工行 dispatch）；审计 dispatch 行（deliver/audit_dead/spawn_needed）全带 `prompt_template:"caster/audit-review.md"`；`config/caster/audit-review.md` 存在且模板渲染含 `{{ticket_id}}`/`META_VERDICT`/`meta events`/`meta phase` | **绿** |
| 7 对抗性锚 a/b/c（本席对照机器记录复算） | a. `internal/caster/caster.go:220` 闸行前置 `false&&`→`go test -run TestRoleFilter ./internal/caster/` → **FAIL**（`Dispatch=5 预期 3`——deliver{role:CTO} 又派审计）；b. `internal/gateway/dispatch.go:197` 闸行前置 `false&&`→`go test -run TestDispatch_空提示拒起 ./internal/gateway/` → **FAIL**（spawn_blocked 超时不至——空提示席被起）；c. `internal/gateway/dispatch.go:378` 席位级早退前置 `false&&`→`go test -run TestDispatch_席位级死因禁标壳 ./internal/gateway/` → **FAIL**（`seat_err 行数=0`——席位级死因进 failSet 冤标壳）。三变异均 cp 备份回退，回退后全量 `go build && go test -count=1 ./...` 全绿，git status 零行 | **绿** |

## 范围与形制

- 范围内八件全中：proto/pb（`Route.role`＋ `EventItem.role` 消费）、caster process 闸、
  dispatch 空提示闸、inject.Assemble 兜底、seat/adapter 席位级分级、caster.textproto 拆行、
  audit-review.md、单测。
- 范围外（非蔓延，皆为必要伴随修正）：`internal/gateway/attach.go` onRunDead 适配新 reportShell
  签名（加 `st` 参）；`internal/gateway/gateway.go` `runState.failReported` 原子去重（trySpawn 秒退窗
  与 onRunDead 判死径双报去重——「只落一次 seat_err」语义所必需）；`dispatch_test.go`/`t0004_test.go`/
  `inject_test.go` fixture `/dev/null`→非空 prompt（D-40 闸后零字节必拒起，原有正常派发测试意图机械必要
  适配，断言意图原样保留）；`route_test.go`＋2／`helper_test.go`＋7；`docs/经验库/2026-11-01-…git-checkout吃未提交改动.md`
  ＝令#E1 一坑一文。
- 形制：支名 `t/0009-dispatch-guards`／commit `t0009: <为什么>`／已推 origin ✅／未并 master ✅／
  交付摘要**五节**＋收口两步门＋`META_DELIVER` 哨兵 ✅（交付 run 日志 `60da4b418d589afb.log` 尾部实录）。

## 质量核三问（令#E5）

- **最优解**：role 闸复用 `process()` 既有 kind/note_contains 串行闸位；席位级复用 `ExitClass` 枚举＋
  `ClassOfExit`/`ExitCodeOf` 映射表；空提示闸打在「最终 PromptFile」单一机械判点（覆盖径/注入径/产物空
  四态同径收口）。未见更简等价解存在，未用。
- **第一性**：三件一簇各打在决策 52 因果链的机械可判点（路由匹配点／派发前最终产物点／退码分级表），
  非调用点散布补丁；`codeInputErr` 哨兵与真退码分桶（进程退码域 0-255，-2 无碰撞）干净。
- **洁净**：diff 内代码/配置层 TODO／FIXME／绕过／半成品命中＝**零**（唯一 grep 命中行＝判决档提示
  文字本体，非实现）；无同逻辑两处实现／死码／调试残渣；禁式自检（R2/R3）零命中。

## 债项节（令#E6 对账）

| 项 | 状态 |
|---|---|
| D-0008a（设计-0007 滞留 c/0007-design） | **上游已申报并立**（t0008 返修台账 `planned:t0008`，对象 `c/0007-design @ 9321304` 可达）；本票不累及，挂账待 CTO |
| 交付摘要 §1 小疵（误列 `internal/adapter/adapter_test.go`，本票 diff 未触此件） | **入档备查**——摘要非证据（令#E4），diff 实证无此件改动；不计入否决 |
| 并发遗留观察：promptFileEmpty 判后与席位 Build 间 TOCTOU 窗 | 窗内文件被删→`codeInputErr`→`seat_err` 禁标壳兜底，链条自洽；记观察、不计缺陷 |
| 部署同步 | `config/caster.textproto`＋`config/caster/audit-review.md` 至 `/opt/meta/config/`＋重启 caster 属部署径（CTO-B/运维），与摘要「下一步」一致 |

## 可沉淀意见（令#E8 · 纯意见，不构成否决依据）

席位级死因单一 `ExitWrapper` 分级、空/零字节 prompt 机械闸、路由行 role 等值过滤（空＝不滤）——
**可沉淀**：三者皆「机械可判、不依赖人判」、与既有退码表／禁式闸同构，且对抗锚（a/b/c）三提名行
独立变异实测红，可对抗验证。

## 结论

**通过**。交付（commit `c1a1d91`）对 ticket 0009 七锚全绿（build/vet/test/selfcheck、
role 过滤、空提示拒起、席位级死因、壳级回归、配置、对抗锚 a/b/c），pb 再生 byte-identical，
交付摘要五节＋收口两步门＋`META_DELIVER` 齐备，范围净、暗债零瞒报、形制全。
决策 52 因果链三环（误派审计／空提示烧壳链／席位级误标）各有机械防线且互为犄角，无人值守
审派链本票收口；后续推进＝部署闸提级（票 0010，`audit_pass→CTO-B`），归 CTO 管。

META_VERDICT:pass:docs/实现/票-0009-判决/3-opencode.md
