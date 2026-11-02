# 票 0041：部署 integrate 合并径——meta-integrate.sh＋配方接线

> 设计票 0037-p2-p1 的执行子票（CTO-A 拆票）。设计档：
> `docs/实现/设计-0037-p2-p1-integrate分叉径与ref校验.md`（16866b2）。

### 1. 目标

新增 `scripts/meta-integrate.sh`（形制照 meta-deploy.sh：仓内固定件、
绝对径调用、`set -euo pipefail`），并把 `config/gateway.textproto`
integrate_cmd（L36）一行改指 `bash /opt/meta/scripts/meta-integrate.sh`。
脚本按设计档§4 落五段：ref 三闸校验 → 前置闸 → 已并幂等 → ff →
`merge --no-ff` 分叉并合（冲突 abort＋fail loud 非零退）。并合对象恒为
本票交付支顶（交付＋判决档 commit 全含）。

### 2. 为什么

- 0037-p2 部署双败实证（run 804d19649fdf4c38/82160435991179f7 在案）：
  3144 污染 ref 直达 git（`t/0037-p2-precedent-inject哨兵已双发。等`），
  3146 `merge --ff-only` 撞分叉 fatal。配方零校验＋ff-only 单径＝机械死局。
- master 前移×ff-only 结构性复发：票 0040 批 rebase 只解当批——当下
  `t/0037-p2-precedent-inject` 与 master 已再分叉（CTO-A 复核
  mb=2e522e3）。
- 席位无自救径（禁区#8 部署只走配方命令）——防线必须长在配方脚本层。

### 3. 范围内

- `scripts/meta-integrate.sh` 新件（可执行位 100755）：S0-S7 按设计档
  §4 伪码落地——字节集闸 `^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`、候选支
  解析（`refs/heads|refs/remotes/origin` 下 `t/<TID>-*`，剥 `origin/`
  去重本地优先，剔除子票命名空间 `t/<TID>-p[0-9]*-*`）、rev-parse、
  D_alias 提取（`meta events` 末条 deliver note 首段；败→warn 降级不
  die；`META_DELIVER_REF` 置→跳 CLI 直取）、支系判（`C⊆T` ∥
  `C==D_alias`）、前置闸（HEAD=master ∧ 净树 ∧ 无 MERGE_HEAD）、
  幂等判、ff、`merge --no-ff`＋并后 ancestor 复核、冲突 `merge --abort`
  ＋非零退。
- `config/gateway.textproto`：L36 integrate_cmd 一行改指脚本绝对径——
  配方其余字节不动。
- 四场景＋别名径实证档：scratch 仓（mktemp -d）逐场景跑脚本留输出，
  经 `META_INTEGRATE_REPO`/`META_TICKET_ID`/`META_EXEC_REF`/
  `META_DELIVER_REF` 环境钩子注场景；档落 `run/artifacts/<本run>/`。
- `bash scripts/selfcheck.sh`＋`go build/vet/test` 四件绿（脚本虽非
  Go，全仓闸照跑防误伤）。

### 4. 范围外（禁线）

- 不改网关任何 .go（exec/锁/注入——B1 串行闸不动；校验全在配方脚本层）。
- 不 rebase 任何交付支、不回退 master（决策81）、不推 origin master。
- 不部署 0037-p2（补发环是网关的活——本票只保径通）、不替席位解冲突
  （机械件不判语义，冲突 abort＋事件即尽）。
- 不碰 claude 壳配额项（环境 2026-11-07 自愈，范围外）；无定时器/轮询/
  后台件（R2）；不改 roles/、不动他票相位与他支。

### 5. 设计引用

- `docs/实现/设计-0037-p2-p1-integrate分叉径与ref校验.md`（16866b2——
  全案唯一真源：三闸语义/D_alias/偏差记/生效链/四场景锚）。
- `config/gateway.textproto` L36-40；`scripts/meta-deploy.sh`（形制
  母本：绝对径/set -euo/幂等闸/stamp 码面）；`scripts/meta-probe.sh`
  （`META_BIN` 缺省径先例）。
- `internal/gateway/exec.go` L218-222（META_* envs）、L337-351
  （lastDeliverRef 兜底——D_alias 必要性来源）、L171-179（integrate
  非零→merge_conflict emit）；`config/caster/cto-deploy.md`（META_EXEC
  ref 可省约定）。

### 6. 验收锚（全部机器可判）

1. `bash -n scripts/meta-integrate.sh` 零错；`git ls-files -s` 见
   100755 可执行位。
2. 场景一·污染 ref：scratch 仓内置 `t/<tid>-x` 交付支，`META_EXEC_REF`
   为 `t/<tid>-x`＋CJK 残片拼接串 → 退码≠0、`rev-parse HEAD` 前后字节
   一致、无 `.git/MERGE_HEAD` 生成。
3. 场景二·分叉并合：master 与 `t/<tid>-x` 各独有 ≥1 commit → 退 0、
   `merge-base --is-ancestor <支顶> master` 退 0（含支顶全部 commit）、
   `git log -1 --format=%P` 恰两个 sha（merge commit）。
4. 场景三·已并幂等：支顶已 ancestor → 退 0、HEAD 不动、`git log -1
   --format=%P` 单亲（未产空合并）。
5. 场景四·ff：线性可快进 → 退 0、`rev-parse HEAD`==支顶、`%P` 单亲
   （无 merge commit）。
6. **对抗锚（删之/翻之必红）**：(a) 把字节集闸翻成恒真（注释或 `.*`）
   → 场景一必红（污染串直达 git）；(b) `--no-ff` 翻 `--ff-only` → 场景
   二必红。交付摘要引两处变异实测红录（原文行＋退码）。
7. 兜底别名径实证：`META_DELIVER_REF`=<不在支上的旧 sha>、
   `META_EXEC_REF`=同 sha → 脚本放行且并的是**支顶**非该 sha（
   master 含支顶、不含该旧 sha 本身若其真不在支系）——「别名放行但
   并顶」语义实证。
8. `bash scripts/selfcheck.sh` 零命中；`go build ./... && go vet ./...
   && go test -count=1 ./...` 绿；`git diff master --
   config/gateway.textproto` 恰 integrate_cmd 一行。

### 7. 验证命令

```bash
bash -n scripts/meta-integrate.sh
git ls-files -s scripts/meta-integrate.sh
cd <scratch 仓按 §6 场景构造>
META_INTEGRATE_REPO=<scr> META_TICKET_ID=<tid> META_EXEC_REF=<ref> \
  META_DELIVER_REF=<sha> bash <worktree>/scripts/meta-integrate.sh
cd <worktree>
bash scripts/selfcheck.sh
go build ./... && go vet ./... && go test -count=1 ./...
git diff master -- config/gateway.textproto
```

### 8. 交付形制

新支 `t/0041-integrate-merge-path`（基于 master 顶，独立 worktree
`~/meta-worktrees/t0041-integrate`）；commit 信息 `t0041: <根因式>`
写根因式说明；交付＝推支 origin＋末行哨兵 `META_DELIVER:<commit>`
（stdout 独占一行＋同行原文追加 `$META_ART_DIR/sentinel.log` 文件径
先行，D-42 双发）；交付摘要五节（新增/修改面→文件、根因、验证→引
机器记录、下一步全量遗留申报、可沉淀打法候选）＋收口两步门
〔令#E4/E6/E9〕；**不部署**（并入 master 走既有径）。

## 附

- 优先级 P1（部署通道结构性堵根治）；依赖：无；预算/壳序：缺省。
- 「精确全等」的落地语义按设计档§3 偏差记（字面全等破网关兜底径，
  已记档非擅改）；D_alias/子票命名空间剔除/origin 归一细节全按档。
- 环境钩子：`META_INTEGRATE_REPO`（测试笼代仓）/`META_DELIVER_REF`
  （别名直取跳 CLI）——仅测试与无网关环境用，生产网关不传即走缺省。
- 配置生效链：本票部署仍跑旧 integrate_cmd（鸡生蛋边界，设计档§5
  记档）；scripts/ 在 stamp 码面→部署换装重启→配方重种子生效；
  config-only 变更不触重启的陷阱已记档。
- 0037-p2 实战尾锚随补发环自走（非施工所能控，按票面记因径）。

## 判决史

（空——首判前无）
