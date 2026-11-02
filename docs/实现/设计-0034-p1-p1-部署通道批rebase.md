# 设计-0034-p1-p1：部署通道批 rebase——ff-only×master 前进分叉（0034-p1/0036/0037-p2 三支同因）

> 需求票 `0034-p1-p1`（巡检席自立案）→ CTO-A 设计 → 子票 0040 施工。
> 钦定轨（§1.4）：设计档 commit → 本票八栏回填 → 子票立＋派 → Deliver。

## 0. 实证链（复核全绿——patrol 诊断成立，且堵面比票面更大）

- integrate 配方（`config/gateway.textproto:36`）：
  `git -C /opt/meta merge --ff-only "$META_EXEC_REF" || git -C /opt/meta
  merge-base --is-ancestor "$META_EXEC_REF" HEAD`——ff-only，分叉即红。
- 0034-p1 事件史：2898/2899 integrate exit=1 `Diverging branches can't be
  fast-forwarded`（部署席只读坐实）；2963/2964 第二部署席复红同文；
  82b0b9a2 席死第三回——三连 deploy_failed 同因。
- **同因已打穿整通道**（本席复核 run/exec 档，非推断）：
  - `run/exec/0034-p1-integrate-*.log` ×2——ff 红；
  - `run/exec/0036-integrate-1793596861943697835.log`——同文 ff 红；
  - `run/exec/0037-p2-integrate-1793597920196294120.log`——同文 ff 红。
  0034 一支是「Already up to date」（e109692 已并 master）——不在堵集。
- 拓扑复核（2026-11-02 当下）：master=deb4dc8（本地顶，领先 origin
  8c1b58a 三个 docs 提交）；
  - `t/0034-p1-cage-identity`=99e11e6（含未推 origin 的判决档一提交），
    mb=453c76a，ahead=2，与 master 触面交集＝债务台账.md 唯一；
  - `t/0036-card-phase-elapsed`=07cba7c，mb=bb43a3f，ahead=2，
    触面（internal/db、feishu、gateway 卡）与 master 新增零交；
  - `t/0037-p2-precedent-inject`=6632782，mb=f119f11，ahead=2，
    触面（inject/precedent+testdata）与 master 新增零交。
- 部署重派环健在：网关对 deploy_failed 持续 `锁释补发 spawn_needed`
  （0034-p1 事件 3032-3159），卡点在 CTO 壳尽（claude 限额至 11-07）
  与 B1 串行锁排队——支修好，下次补发 integrate 即绿，无需人工重派。

**结论**：patrol 票面诊断对，但堵面＝三支同因非一支。单修 0034-p1
只解一个槽位，串行闸一放 0036/0037-p2 照撞同墙、再烧壳——按票面
§2「部署通道对全部待部署票联动堵」的通道级立意，子票做**批 rebase**。

## 1. 方案：三支批 rebase 至执行时 master 顶

| 支 | worktree | 随迁提交 | 预判冲突 |
|---|---|---|---|
| t/0034-p1-cage-identity | ~/meta-worktrees/t0034-p1 | 20bc6f5(修复)+99e11e6(判决档) | 债务台账撞号（§2） |
| t/0036-card-phase-elapsed | ~/meta-worktrees/t0036-card-elapsed | 2 | 预期零 |
| t/0037-p2-precedent-inject | ~/meta-worktrees/t0037-p2-precedent | 2 | 预期零 |

每支同构：worktree 内 `git fetch origin && git rebase master`
（master＝执行时本地顶，不写死 sha——master 仍会前进）→ 解冲突 →
`go build ./... && go vet ./... && go test -count=1 ./...` →
`git push --force-with-lease origin <支>`（lease 撞＝fetch 重 rebase
再推，幂等）→ 核 `merge-base --is-ancestor master <支>`。

## 2. 0034-p1 台账撞号处置（patrol 预判已过期，顺延一档）

- patrol 预判「D-67 双定义、交付支改 D-68」——**master 侧 D-68 已被
  决策91 占用**（裸立票不点火，8c1b58a），交付支 deploy_failed 断路债
  顺延改号 **D-69**。
- 冲突面唯一：债务台账.md 双侧追加行。解＝保 master 侧 D-67（卡更新
  循环）/D-68（裸立票）行＋交付支行改号 D-69 落账，行内容一字不改。
- 随迁修正：判决档 `docs/实现/票-0034-p1-判决/1-devin.md` 两处「D-67」
  指代同债（44/46 行）同步改 D-69——同债同号保可追溯；该档其余不动。

## 3. 范围外与记档

- 不改 integrate 配方（ff-only→no-ff/自动 rebase＝另一票过设计席）。
  本席意见记档：master 文档高频直提与 ff-only 结构性相斥，「master 前进
  →待部署支批量过期」会周期性复发，值得 CEO 裁是否立项（候选：integrate
  步容忍 behind 时先 rebase 再 ff，或部署闸绑 origin/master 单一真源）。
- 票 35 另案异常（不属本堵集）：deliver ref=b063b70 落在 master 历史
  （integrate 将平凡「Already up to date」过），但仓内无 t/35-* 交付支；
  其 deploy 两连「席死」属壳尽类，与分叉无关——记此供巡检/CEO 另裁。
- 伪红族已在原票面§2 记档（META_EXEC 参数零校验，污染 ref 直达 git
  才败）；防呆立票留 CEO/后续设计票——本票不扩。
- master 移动竞速（诚实边界）：rebase 顶取执行时 master；若 master 在
  rebase 与下次部署间再前进，同墙可复发——结构性，唯配方改可根治。
  子票锚断「执行后当下可 ff」，不承诺永久免疫。

## 4. 验证主径

零实现码，锚全机器可判：`merge-base --is-ancestor`×3、origin 顶一致、
台账 D-67 单行/D-69 落账、判决档 D-67 清零、对抗锚＝0034-p1 支 sameUser
`os.Getuid()` 数字比存续（丢 20bc6f5 修复则断言必红）、三支顶
build/vet/test 绿、随迁提交计数≥2。
