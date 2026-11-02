# 票 0041：rollback_cmd 按 MERGE_HEAD 分径＋rollback_aborted 事件面

> 施工票：0034-p1-p2 需求 → CTO-A 设计（设计-0034-p1-p2-rollback分径.md）
> → 本票施工。先读范围外——那是你的禁区。

### 1. 目标

`rollback_cmd` 按 `git -C /opt/meta rev-parse -q --verify MERGE_HEAD`
实况分径：冲突残留态 `merge --abort` 弃净并 echo `ROLLBACK_ABORTED:`
标记（abort 败则非零退 fail-loud）；无 MERGE_HEAD 维持现 echo 文案
exit=0。网关侧补 `rollback_aborted` 事件（rollback 步 tail 检得标记
即记）——「冲突→弃净」闭环机读。台账 D-78（rollback 假成功行）销号。

### 2. 为什么

- 4128 rollback exit=0 假成功实证：integrate no-ff 冲突中断留
  MERGE_HEAD＋UU 半合并脏树，rollback 无条件 echo「无破坏可回」——
  前提（merge 已并历史）失配。0034-p1 事件 4120-4128。
- no-ff 时代每冲突必留脏树：ff-only 时 integrate 红不落态，rollback
  空转无害；T1/0040 改 no-ff 后配方未同步——前提漂移。
- 0036/0037-p2/0038-p1 待部署支未 rebase，再撞冲突即复现：脏树拦
  主仓一切 git 操作（含部署闸与批 rebase 类票自身）。

### 3. 范围内

- `config/gateway.textproto:40` rollback_cmd 改 MERGE_HEAD 分径——
  钦定基线形见设计档§2.1（`-C /opt/meta` 显式主仓径；abort 径 marker
  `ROLLBACK_ABORTED:`；abort 败 `ROLLBACK_ABORT_FAILED`＋exit 1；
  else 径原文案一字不动）。
- `internal/gateway/exec.go`：T1 merge_conflict 块（:172-185）同位
  加 rollback 块——`req.Step=="rollback" && resp.ExitCode==0 &&
  strings.Contains(resp.Tail,"ROLLBACK_ABORTED")` → AppendEvent＋emit
  `rollback_aborted`（设计档§2.2 钦定形；marker 直比＝机械判）。
- 新单测档 `internal/gateway/t0041_test.go`（仿 t_t1_test.go 形制）：
  真 rollback_cmd（自 textproto 读出，`/opt/meta` 字面置换 fixture
  径）两径 fixture 各一＋事件径一（tail 带 marker→事件落行；不带→无）。
- `docs/实现/债务台账.md` D-78「rollback 假成功」行：修法栏改
  「票0041（MERGE_HEAD 分径＋rollback_aborted 事件）」＋移已清区或
  标 done:票0041（台账既有形制从之）；**只动该行**——台账另有一行
  D-78（deploy_failed 补发断路）撞号，勿碰。
- `docs/实现/外派施工提示词.md` T4 补充面第 1 条（D-78 rollback
  假成功）标注已落地票0041——防 T4 届时重做。

### 4. 范围外（禁线）

- 不动 integrate_cmd／check_cmd／deploy_cmd／probe_cmd；不回退已并
  历史（决策81——merge --abort 弃的是未完成合并非历史）。
- 不碰 exec.go T1 前缀判读（:172-178 REF_LOST/CONFLICT 语义不动）、
  不加任何周期/定时件、不碰 D-76/D-77/D-79 诸案。
- 不部署、不并 master、不动他票相位与各交付支 rebase。
- 禁复活 legacy-sudo（`TestDeploy_LegacyDenied` 保绿）；禁占主仓
  /opt/meta 施工（worktree 规约）。

### 5. 设计引用

- `docs/实现/设计-0034-p1-p2-rollback分径.md`（钦定命令形/事件块/
  fixture 形制/范围外记档——先读）。
- 需求票面 `docs/实现/票-0034-p1-p2-rollback对冲突态假成功.md`。
- `config/gateway.textproto:36,40`；`internal/gateway/exec.go:172-204`；
  `internal/gateway/t_t1_test.go`（测试形制）；deploy_test.go
  `TestDeploy_RedRollback`（driveSeat 径）。
- 0034-p1 事件 4120-4128；决策81（rollback 现文案来历）/决策92。

### 6. 验收锚（机器可判）

1. `grep -E 'rollback_cmd.*MERGE_HEAD' config/gateway.textproto` 命中
   且同句含 `merge --abort` 与决策81 原文案段。
2. 单测 abort 径绿：fixture 仓半合并态（MERGE_HEAD 活＋UU）跑真
   rollback_cmd → exit=0、stdout 含 `ROLLBACK_ABORTED:`、事后
   `git rev-parse -q --verify MERGE_HEAD` 解析败、`git status
   --porcelain` 净。
3. 单测 echo 径绿：fixture 无 MERGE_HEAD → exit=0、输出含决策81
   文案、`git status --porcelain` 空、HEAD 不变。
4. 单测事件径绿：rollback 步 tail 带 marker → ticket_event 增
   `rollback_aborted` 行；tail 无 marker → 无该行。
5. **对抗锚**：删 exec.go 新增 rollback marker 判读块 → 锚4 必红；
   rollback_cmd 回退无条件 echo → 锚2 必红（MERGE_HEAD 残留仍活）。
6. `go build ./... && go vet ./... && go test -count=1 ./...` 交付支
   顶全绿；`TestDeploy_RedRollback`/`TestT1_*`/`TestDeploy_LegacyDenied`
   无回归。
7. 台账「rollback 假成功」D-78 行已标 done:票0041（或入已清区），
   另行 D-78（补发断路）原文未动。

### 7. 验证命令

```bash
grep -n "rollback_cmd" config/gateway.textproto
go test -count=1 ./internal/gateway/ -run 'Rollback|T0041'
go build ./... && go vet ./... && go test -count=1 ./...
git -C /opt/meta rev-parse -q --verify MERGE_HEAD || echo "主仓净"
```

### 8. 交付形制

支名 `t/0041-rollback-mergehead`；worktree `git -C /opt/meta worktree
add /opt/meta-wt/0041-rollback-mergehead -b t/0041-rollback-mergehead
master`（或自选径，禁占主仓）。交付＝推支＋stdout 末行哨兵
`META_DELIVER:<commit>`＋sentinel.log 双发〔E15/D-32/D-42〕；摘要
四节（新增/修改面、根因——必答「为什么之前会错」、验证引机器记录、
下一步全量遗留申报）〔令#E4/E6〕；收口两步门〔令#E9〕。

## 附

- 优先级 P1；依赖：无；预算/壳序：缺省。
- exec 步 cwd＝deploy 克隆（exec.go:196/260）——配方主仓操作一律
  `git -C /opt/meta`，不得依赖 cwd；测试同理字面置换 fixture 径。
- marker 字面 `ROLLBACK_ABORTED`（无冒号亦判中——Contains 语义）；
  echo 径文案不含此串，天然零事件。
- abort 败径（MERGE_HEAD 活但 --abort 红）：非零退＝fail-loud，
  exec 事件已带 exit=1＋tail；不另立事件种。

## 判决史

（空——首判前无）
