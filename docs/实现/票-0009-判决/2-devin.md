# 票 0009 判决档 2（壳：devin · 二判）

- 交付支：`t/0009-dispatch-guards` @ `0739145`（施工提交 `c1a1d91`；base master
  `8c35293`；`git ls-remote` 远端支头＝本地 HEAD——本席实测）
- 承接：首判 `1-claude.md` 已入支但席死于哨兵前（事件 258 audit_dead）。本档为
  二判，**全部锚由本席在交付 worktree 独立复算**（先例 0008 判决 2／本票判决 1
  同标准），不只信施工记录；本席复算记录 `/tmp/t0009-audit2-rerun.txt`。
- 施工机器记录 `/tmp/t0009.machine-record.txt`（mtime 2026-11-02 00:00:52，
  晚于施工 commit 23:43:52——commit 后验证态；席位日志尾部见 tee/变异实录）。

## 逐锚对账（票面 §6 七锚）

| 锚 | 机器证据（本席复算） | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 本席跑 `go build ./... && go vet ./...`→BUILD_VET_OK；`go test -count=1 ./...`→13 包全 `ok`（gateway 13.47s，tee 原文在 `/tmp/t0009-audit2-rerun.txt`）；`bash scripts/selfcheck.sh`→SELFCHECK_EXIT=0。与施工 tee 记录一致 | 绿 |
| 2 role 过滤（D-39） | `caster.go:218-220` `if rt.Role != "" && rt.Role != row.Role → continue`，位于 kind 闸后、note_contains 闸前——与既有串行闸同位（票面钦定位）。`TestRoleFilter`（caster_test.go:373）：deliver{role:施工}→dispatch 审计＋prompt 渲染含票号；deliver{role:CTO}→零 dispatch 只 notify；ping 空 role 行全匹配回归；multi 行 m1/m2 失格零动作、m3 双条件过动作 | 绿 |
| 3 空提示拒起（D-40） | `dispatch.go:196-210` 闸位于注入段（:180-190）后、`shellsFor`（:212）前。`promptFileEmpty`（:413）空路径/读不出/全空白三态机械判。`TestDispatch_空提示拒起` 四子径全断言 spawn_blocked（note 含「空提示拒起」）＋`RunsForTicket`==0＋failSet 空＋健康零迁移：①PromptFile=""+InjectRoot 未启；②不存在文件；③零字节；④InjectRoot 置位＋mock `assemble` 返 ""（`inject.go:17` 包级缝，落空文件被闸捕获）。生产表 grep 实证 spawn_blocked/seat_err 无消费路由——无再入派发环 | 绿 |
| 4 席位级死因禁标壳 | `seat.go`：`ExitInputErr=5`（:35）＋`codeInputErr=-2` 哨兵（:42）；`runChild` Build 失败/prompt 不可读两分支由 -1 改返哨兵（:188/:196）；`run()` 哨兵→`ExitWrapper` 归级（:140-147）；`ClassOfExit` 2/3/4/5→ExitWrapper（:521-524）；`ExitCodeOf(ExitWrapper)`=ExitInputErr（:519）。`reportShell` 席位级早退（dispatch.go:378-389）：seat_err 事件（壳名＋退码＋`logTail` 尾摘要）即返，不进 failSet/不调 NextHealth。`TestDispatch_席位级死因禁标壳` 四退码各落 seat_err×3（三候选各一）＋failSet 空＋健康未动；`TestRunChild_输入层` 双分支返哨兵；`TestRun_缺env`→ExitSeatErr | 绿 |
| 5 壳级回归 | `TestDispatch_壳级回归`：①61→failSet 全行＋健康不迁＋零健康事件；②a 62→limited＋shell_limited；②b 63→dead＋shell_dead。`route_test.go` 增 ExitWrapper 行（OK/DEAD 均不迁）——壳级三行语义未动 | 绿 |
| 6 配置 | `TestProdConfig`：生产 `config/caster.textproto` 加载绿；deliver 拆 CTO（断言零 dispatch）/施工两行；审计 dispatch 三行（deliver/audit_dead/spawn_needed）全带 `prompt_template:"caster/audit-review.md"`；模板存在且含 `{{ticket_id}}`/`META_VERDICT`/`meta events`/`meta phase`，render 后占位符消化 | 绿 |
| 7 对抗锚 a/b/c（本席独立变异复算，cp 备份回退，`git status --porcelain` 还原后均零行） | a. caster.go:220 前置 `false&&`→`TestRoleFilter` FAIL（`Dispatch=5 预期 3`——deliver{role:CTO} 又派审计）；b. dispatch.go:197 置 `false&&(…)`→`TestDispatch_空提示拒起` 红（spawn_blocked 8s 超时不至，文件缺失/零字节/注入产物空各子径红）；c. dispatch.go:378 置 `false&&(…)`→`TestDispatch_席位级死因禁标壳` 红（`seat_err 行数=0`——席位级死因落进 failSet 冤标径）。与施工机器记录三变异结论逐条一致 | 绿 |

**pb 再生**：本席跑 `protoc 3.21.12 + protoc-gen-go v1.36.12`（与仓内 pb 头一致）
再生 `meta.pb.go`→`git status --porcelain` 零行＝byte-identical，role 字段已入
raw descriptor（diff :3003-3013）。

## 范围与形制

- 范围内八件全中：proto/pb、caster process 闸、dispatch 空提示闸、inject.Assemble
  产物兜底、seat/adapter 席位级分级、caster.textproto 拆行、audit-review.md、单测。
- 范围外文件逐件核（非蔓延）：
  - `attach.go`＝`onRunDead` 适配 reportShell 新签名（加 `st` 参）——编译必要；
  - `gateway.go` `runState.failReported`＝同 run 去重——trySpawn 秒退窗径与
    onRunDead 判死径对同一死并发双报，不 dedup 则锚 4 测试要求的「seat_err
    三候选各一」必破；属本票「只落 seat_err 事件行」语义的必要伴随修正，
    壳级语义由锚 5 回归证实不动；
  - `dispatch_test.go`/`t0004_test.go`/`inject_test.go` fixture `/dev/null`→
    临时非空 prompt——D-40 闸后零字节必被拒起，原「正常派发」测试意图的机械
    必要适配，断言意图原样保留，非改测试迁就实现；
  - `route_test.go`＋2＝新分级入表驱动；`helper_test.go`＋7＝exitcode 测试模式；
  - `docs/经验库/2026-11-01-…git-checkout吃未提交改动.md`＝令#E1 一坑一文授权；
  - `docs/实现/票-0009-判决/1-claude.md`＝首判档（协议钦定落点）。
- 范围外禁线复核：audit_dead/spawn_needed 行只补 `prompt_template`（范围内钦定），
  语义未裁；无票型列/无 CEO 面接线路由（CTO 行零 dispatch 动作，测试断言在案）；
  NextHealth 表未动；无 Ticker/周期件。
- 形制：支名 `t/0009-dispatch-guards`／commit `t0009: <为什么>`／已推 origin
  （远端支头＝本地实测）✅；交付摘要**五节**＋收口两步门＋`META_DELIVER` 哨兵
  在席位日志 `run/logs/60da4b418d589afb.log` 尾部实录 ✅。

## 质量三问（令#E5）

- **最优解**：role 闸复用 process() 既有串行闸位；席位级复用 ExitClass 枚举＋
  ClassOfExit/ExitCodeOf 映射表；空提示闸落在「最终 PromptFile」单一机械判点，
  覆盖径/注入径/产物空四态同径收口。未见更简等价解未用。
- **第一性**：三闸各打在因果链机械可判点（路由匹配点/派发前最终产物点/退码
  分级表），非调用点散布补丁；`codeInputErr` 哨兵与真退码分桶（进程退码域
  0-255，-2 无碰撞）。
- **洁净**：无同逻辑两处、无死码（`assemble` 包级缝有测试消费）、无调试残渣；
  diff 内 grep TODO/FIXME/绕过零命中（唯一命中＝判决档/模板指引文字本体）。

## 债项节（令#E6 对账）

| 项 | 状态 |
|---|---|
| 摘要「下一步」三件（票 0010 部署闸、后续批次、config 部署同步归 CTO-B/运维） | 全票外正当申报，与 diff 零 TODO/零半成品对账一致——**无瞒报**；部署备忘票面 §8 本就归运维径 |
| 摘要§1 误列 `internal/adapter/adapter_test.go`（本票 diff 未触此件，该文件属 t0002） | **摘要小疵**——文件清单多列一件；摘要非证据（令#E4），diff 实证无该件改动，不累及判决，入档备查 |
| 设计引用 `docs/实现/设计-0007-阶段3.md` 不在 master（仅 设计-0004 在库；对象在 c/0007-design 支 commit 9321304） | **上游已申报债（判决 1 记为 D-0008a）**——票面引用栏受染但八栏自足机械可判、现场码六件全在 master，施工未受阻；不累及本票，挂账待 CTO |
| reportShell failReported 去重 | 伴随修正（见范围节），锚 5 回归绿证不伤壳级语义 |
| 并发遗留观察：`promptFileEmpty` 判后与席位 Build 间存 TOCTOU 窗 | 窗口内文件被删→席位 `codeInputErr`→seat_err 禁标壳兜底，链条自洽，非缺陷 |

## 可沉淀意见（令#E8 · 纯意见不构成否决）

摘要三条候选（席位级死因单一 ExitWrapper 分级、空/零字节 prompt 机械闸、路由
role 等值过滤空＝不滤）——**可沉淀**：皆「机械可判、不依赖人判」形制，与既有
退码表/禁式闸同构，且三提名行经两席独立变异实测红，可对抗验证。

## 结论

**通过**。七锚由本席独立复算全绿（含对抗锚 a/b/c 本席突变实测红、pb 再生
byte-identical——protoc 3.21.12 再生后 `git status` 零行）；范围净、暗债零
瞒报、形制全；与判决 1 结论一致。0007 连环事故三环（role 误派→空提示→误标
壳）各有机械防线互为犄角，复位后的无人值守审派链本票收口。
