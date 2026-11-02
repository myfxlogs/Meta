# 票 0034-p1 判决：通过（审计席·devin 独立实例，2026-11-02）

- 审计 run：`0a17087ebc54f87f`（shell=devin/swe-2-max），触发＝deliver（事件 2871，`20bc6f5 devin exit=0`）→ audit_start（事件 2872）；判决史空＝首判
- 施工 run：`d7f4dd86574115a2`（shell=devin/swe-2-high，exit=0 已死），worktree `~/meta-worktrees/t0034-p1`
- 交付物：支 `t/0034-p1-cage-identity` @ `20bc6f5`——单 commit `t0034-p1: check 笼内 uid 无名条目致身份判定全失效——sameUser 改 uid 数字真源比对`，merge-base＝master 梢（`master is ancestor` 实证），4 文件 +120/−13，未并 master
- 推支：`/opt/meta` `refs/heads/t/0034-p1-cage-identity`＝`20bc6f5` 与 `origin/t/0034-p1-cage-identity` 同 hash 亲核
- 机器记录：施工未落 `/tmp/t0034-p1*.machine-record.txt`；本席照票-0026 先例在交付 worktree **原命令复算**，全量记录 `/tmp/t0034-p1-audit-recompute.txt`（含笼内复跑与突变实证）

## 逐锚对账（票面 §6 五锚）

| 锚 | 机器证据 | 判 |
|---|---|---|
| 1 build/vet/gofmt/宿主 test | 复算 `20bc6f5`：`go build ./...` rc=0、`go vet ./...` rc=0；`gofmt -l internal/` 命中 5 文件——经 `/opt/meta` master 基线同款复跑（命中同 5 件＋deploy_test.go），证实为 **master 既有脏区非本票带入**，本批触碰两文件（exec.go/deploy_test.go）`gofmt -l` 零命中且顺手清掉基线脏的 deploy_test.go——照票-0010/0011 先例判绿；`go test -count=1 ./internal/gateway/...`→`ok 24.640s` | 绿 |
| 2 笼内主锚（三测） | 复算笼内（`docker run --rm --user 1000:1000 -v <交付worktree>:/work -w /work -v <gomodcache>:/gomod:ro -e GOMODCACHE/GOCACHE/HOME/GOFLAGS golang:1.27`，与两轮 check 失败 argv 同构，仅挂载点换交付树）：`TestDeploy_FourGreen` PASS 0.21s、`TestDeploy_RedRollback` PASS 0.16s、`TestExec_ContainerArgv` PASS 0.07s——原失败签名（deny -1 ×2＋panic@648）同环境转阴 | 绿 |
| 3 `TestDeploy_LegacyDenied` 保绿 | 同笼复跑 PASS 0.13s——deny 分支未删失、sudo 未复活（diff 中 exec.go deny 段零改动） | 绿 |
| 4 新表驱动锚两 fixture | 同笼 `TestExec_NumericRunAs` PASS（`同uid步直跑` 0.07s＋`异uid步仍拒` 0.05s）；**突变复算**：独立 worktree `/tmp/t0034p1-mut` 删 `sameUser` 数字直比分支→`同uid步直跑` 即红（exit -1 落 legacy deny）、`异uid步仍拒` 仍绿——钉住「删守卫→红」（突变仅落 scratch，交付支未动，已 `worktree remove`） | 绿 |
| 5 端到端解堵（重派部署→check 绿→deploy_done） | 票面 §8 自明「闸即自证（锚5 前置自验）」——部署闸跑的是含修复整合树，属部署相位自验项，本判时不满足触发条件；0034 票仍卡「待部署」，待壳恢复重派后由闸机器记录补证 | 前置自验，缓 |

## 交付形制与范围

- 支名 `t/0034-p1-cage-identity` ✓；commit `t0034-p1: <为什么>` ✓（body 含「为什么之前会错」根因段，令#E4 合规）；推支 ✓；不并 master ✓；`META_DELIVER:20bc6f5` 双发——deliver 事件 2871 ref 对上＋施工 ART_DIR `sentinel.log` 实录在账。
- 交付摘要五节（席位日志 `run/logs/d7f4dd86574115a2.log` 实录）：修改面四件逐一点名、根因（t0029 笼×t0031 deny 组合雷）、验证机器口径、下一步全量申报、可沉淀打法候选——形齐。
- **范围复核**：diff 文件清单 vs §3 范围内逐一对上——`internal/gateway/exec.go`（sameUser/selfIDs）、`internal/gateway/deploy_test.go`（648 判空＋stubSandboxSpec 夹具＋新锚）、`docs/实现/债务台账.md`（D-67）、`docs/经验库/…osGetuid.md`（令#E1「遇坑即写」常态义务，非蔓延）。四件全部有据。
- **范围外机械核对**：`git diff master...20bc6f5` 不含 patrol.go/dispatch.go 及其三测试件（t0034 交付零触碰）、不含 `config/gateway.textproto`、不含 docker 网络设施、deploy_failed 补发策略零改动（仅记账）；exec.go deny 段（legacy-sudo 径）零行改动——§4 禁线全守。
- 禁区自查：diff 增面扫 `TODO|FIXME|XXX|HACK|绕过|临时|fmt.Print|println` 零命中；无周期/定时/心跳式新增；无 JSON。

## 设计对账（票面 §5 引用）

- 引爆链复核：`b9d4a2f` 笼化 `--user uid:gid`＋`a8f7004` deny 分支组合——`exec.go` 现状 `runExecStep` 笼 argv `--user uid+":"+gid` 与 `sameUser` 守卫位置与票面描述逐点对上；`deploy_test.go:648` 原 `me, _ := user.Current()` 吞 err 裸解 `me.Uid` 经 diff 证实改为判空＋`os.Getuid()` 回退，panic 点消除（笼内复跑 PASS 实证）。
- 修法方向符票面授意「uid 数字直比回退（`os.Getuid()`）」：`sameUser` 先 ParseUint 直比、名 runAs 经 Lookup 解 uid 后同比——自身名解析从判定径摘除，与票面「禁复活 legacy-sudo」约束兼容。
- 失败原档对签：`run/exec/0034-check-1793593396709194586.log` 亲读——`deploy_test.go:252/306` deny -1 双 FAIL＋`panic@648`，与票面 §2 签名一致；本席笼内复跑同环境转阴，修复点与失败点一一对应。

## 质量三问（令#E5）

- **最优解**：`sameUser` 归一「run_as→uid→os.Getuid() 数字比」单径——数字 run_as 一次 ParseUint、名 run_as 一次 Lookup＋ParseUint，无更简等价解；新增 `selfIDs()` 收编笼 argv uid/gid 取源（顺带修掉旧回退 `"0:0"` 把容器跑成 root 的静默错值，diff `exec.go` 228 行区实证）。复用既有 `currentUser()` 无逆向依赖。
- **第一性**：直击本质——uid 是身份真源、名解析是显示层；非 workaround 摞补（未动 deny 语义、未加环境分支特判笼内）。
- **洁净**：无重复实现（`selfIDs` 单点收编，sameUser 不二次调 Current）；无死码（`currentUser` 仍经 `selfIDs` 触达）；测试夹具 `stubSandboxSpec` 注释与实现对齐；两源取 uid 顺序「passwd 可解优先、数字兜底」与实现注释一致。

## 债项节（暗债对账：diff 扫描 vs 摘要「下一步」）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| D-p1a | `gofmt -l internal/` 5 文件为 master 基线脏区（adapter/claude.go、caster/caster.go、db/msg.go、db/phase_test.go、gateway/gateway.go） | 已申报，本席 master 复跑实证对上 |
| D-p1b | D-67 只记账未施改（deploy_failed 补发无断路＋shells_exhausted 死锁）——票面钦定「本票只记账」 | 已申报，台账行内容与票面 §1 要求逐点对上 |
| D-p1c | 锚 5 端到端解堵待部署闸自验（0034 卡「待部署」） | 已申报，票面 §8 自明项 |
| D-p1d | 票面所写「记 D-66」编号在交付前已被 `453c76a` 占用，顺延 D-67 | 已申报（摘要＋commit body 双处明示）——非偏差，占位顺延是唯一正解 |

**瞒报零起**：diff 扫描零 TODO/绕过命中；摘要「下一步」三项与本席复算逐条对上。

## 沉淀意见（令#E8，纯意见不构成否决依据）

摘要所报「uid 是身份真源、名解析是显示层」三件套（数字直比＋判空回退＋吞 err 必判空）——**可沉淀**：已同票落 `docs/经验库/2026-11-02-容器笼内uid无名条目-身份判定要走osGetuid.md`，凡容器笼/`--user` 场景身份判定皆可套用，且本票的「宿主绿笼内红」复现手法（同 argv 同构复跑）本身是定位笼内退化的范式。

## 判决

**通过**。锚 1-4 全绿（锚 1/3/4 经本席在交付 worktree 原命令复算、锚 2 笼内同构转阴、锚 4 突变实证删之必红）；锚 5 属票面钦定部署闸前置自验项；范围净（四件全部在票或有令#E1 常态据）、形制齐、暗债零瞒报、§4 禁线全守。

META_VERDICT:pass:docs/实现/票-0034-p1-判决/1-devin.md
