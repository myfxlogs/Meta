# 设计-0037-p2-p1：integrate 配方补分叉径＋ref 校验（0037-p2 双败根治）

> 需求票 `0037-p2-p1`（巡检席自立案·run b5639111b127cf3f）→ CTO-A 设计 →
> 子票 0041 施工。钦定轨（§1.4）：设计档 commit → 本票八栏回填 → 子票立
> ＋派 → Deliver。

## 0. 实证链（复核全绿——三证独立，堵面结构性非抖动）

- **3144 exec exit=-1**：`META_EXEC_REF`＝`t/0037-p2-precedent-inject哨兵已双发。等`
  ——支名与席自报文残片拼接（哨兵长行截断类第三实证：一证 spec 半行→NULL
  票、二证 0B 日志、本证 exec ref 实参）。配方对 ref 零校验，污染实参
  直达 git。
- **3146 exec exit=1**：`merge --ff-only` 撞分叉 fatal——
  `run/exec/0037-p2-integrate-1793597920196294120.log` 原文
  `Diverging branches can't be fast-forwarded`。交付支独有 2 commit、
  master 同窗前移 2 commit；fallback `merge-base --is-ancestor` 同败 →
  merge_conflict → deploy_failed。
- **结构性复发（本席当下复核，非票面转述）**：票 0040 批 rebase 只解当批
  ——`t/0037-p2-precedent-inject` 与 master 已**再次**分叉（master 顶
  6cb1193、支顶 642c7fc、merge-base=2e522e3）。多席公司 master 在交付
  窗口内前移是常态：0034-p1/0036/0038-p1 同因 exec 档在案。
- **deliver ref 时效坑（本席复核新证）**：0037-p2 deliver 事件 ref＝
  `61bf9de`（交付时支顶），票 0040 rebase 后支顶＝`642c7fc`，
  `61bf9de` 已**不在**支上——「与 deliver ref 字面全等比对」会把网关
  自填的合法兜底 ref 拒掉（§3 偏差记）。
- 席位无自救径：禁区#8 部署只由配方命令执行，配方无分叉支＝机械死局；
  回滚径无恙（rollback exit=0，无破坏可回）。

## 1. 钦定取舍：分叉并合径＝`merge --no-ff`

票面钦定二选一，取推荐项 `merge --no-ff`：

- 交付 commit 哈希在 master 可达＝审计锚「交付内容逐字节可溯」保住；
  重放保线性改写哈希＝锚断（票 0040 rebase 实证：61bf9de→b35643f、
  6632782→642c7fc）。
- master 历史 append-only（决策81 不回退同向）——merge commit 自带
  「何票何支」名，并合点可考。
- 真冲突 fail loud：`git merge --abort` 恢复净树＋非零退 → 网关
  integrate 非零即 emit `merge_conflict`（exec.go:173 既有锚）→ 巡检
  订阅在位。**机械件不判语义，冲突不解只报。**

## 2. ref 校验三闸（全在 merge 之前——拒则 master 字节不动）

闸序即拒序，任一不过 die（stderr 明文＋非零退）：

1. **字节集闸**：`META_EXEC_REF` 非空且全匹配
   `^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`——ASCII refname 全集；CJK、
   空白、控制符、`~^:` 后缀操作符全拒。3144 类污染串此闸即死。
2. **可解析闸**：`git rev-parse --verify --quiet "${R}^{commit}"` → C；
   不可解析拒。
3. **交付支系闸**（票面「与票 deliver 支名精确全等比对」的语义落地）：
   - 候选＝`refs/heads/t/${META_TICKET_ID}-*` ∪ `refs/remotes/origin/t/
     ${TID}-*`，剥 `origin/` 前缀去重（本地名优先），剔除子票命名空间
     `t/${TID}-p[0-9]*-*`；恰一支 → B（本票交付支）、T=tip(B)。
     0 支＝「票无交付支」拒；≥2＝「交付支歧义」拒。
   - **受理 iff `C ⊆ T`**（`merge-base --is-ancestor C T`——支名/
     `refs/heads/` 全形/支系上任一 commit 皆同名一物的等价写法）
     **或 `C == D_alias`**（§3 第二条腿）。
   - **合并对象恒为支顶**：R 只当闸不当靶——拼写差异不影响并进来的
     是什么；任一合法拼写并的都是 B 支顶 T（交付＋判决档 commit 全含）。

### D_alias（deliver 事件别名——降兜用不硬依）

`meta events --socket "$META_GATEWAY_SOCK" --ticket "$TID"` 末条
`deliver` 行 note 首段 → `rev-parse --verify --quiet` → D_alias。

- 存在的理由：①部署席可不附 ref——网关兜底取最近 deliver ref
  （exec.go:167-169）；②rebase 后 deliver 事件里的旧 sha 已不在支上
  （本票实证：61bf9de ⊄ 642c7fc-顶支）。纯 `C⊆T` 会把网关自填的合法
  兜底 ref 与「R=deliver sha」写法一并拒死。
- `C==D_alias` 放行后**合并对象仍是支顶 T**——语义＝「你要并的是本票
  交付，交付支已前移，并其现顶」。非「并旧 sha」。
- 提取失败（CLI/socket/输出格式）→ **降级无别名照走**（warn 不 die）
  ——`C⊆T` 主判仍硬；D_alias 只作放行别名不作拒据。
- 测试钩子：`META_DELIVER_REF` 已置则跳过 CLI 直取其值（场景实证与
  无网关环境用）。

## 3. 偏差记（与票面字面口径的出入及理由——如实记档非擅改）

| 票面字面 | 本设计 | 理由 |
|---|---|---|
| 与「票 deliver 支名」**精确全等** | `C⊆T`（支系全等）∥ `C==deliver sha` 别名 | deliver ref 常是交付时 sha；判决档 commit 在其后且 rebase 会改写——字面全等破网关兜底径与支名/全形异写 |
| 锚「交付 commit（61bf9de/6632782）在 master 可达」 | 同义锚＝**交付支顶**在 master 可达 | 票 0040 rebase 已把两 sha 改写为 b35643f/642c7fc——锚指「交付内容可溯」非字面 sha |
| 五段序「校验→ff→已并幂等→…」 | 校验→已并幂等→ff→并合→fail loud | ancestor 判先于 ff 试并更省且幂等语义显式（ff 对已并 ref 本就平凡退 0，互换等价） |

## 4. `scripts/meta-integrate.sh` 形制（照 meta-deploy.sh）

仓内固定件、绝对径调用、run_as=mluser、`set -euo pipefail`、零循环零
定时（R2 净）：

```bash
REPO=${META_INTEGRATE_REPO:-$(cd "$(dirname "$0")/.." && pwd)}
# cwd 不可信（部署 clone 是票码副本——票0029 钦定径同构）；一律 git -C "$REPO"
die() { echo "meta-integrate: 拒步 $1" >&2; exit "${2:-3}"; }
# S0  R=$META_EXEC_REF（空→die 2）；TID=$META_TICKET_ID（空→die 2）
# S1  字节集闸（§2.1）
# S2  for-each-ref 候选支解析→B、T（0/≥2→die 3，§2.3）
# S3  rev-parse→C；D_alias 提取（败→warn 降级）；
#     C⊆T ∥ C==D_alias，否则 die 3
# S4  前置闸：symbolic-ref HEAD==refs/heads/${META_DEFAULT_BRANCH:-master}
#     ∧ status --porcelain 空 ∧ 无 $REPO/.git/MERGE_HEAD——任一不过 die 4
# S5  merge-base --is-ancestor T HEAD → exit 0（已并幂等，不产空合并）
# S6  merge --ff-only "$B" → exit 0（与旧配方行为等价径）
# S7  git -c user.name=meta-integrate -c user.email=meta-integrate@meta.local
#       merge --no-ff -m "meta-integrate: ${TID} <= ${B}" "$B"
#     成→merge-base --is-ancestor "$B" HEAD 复核 → exit 0
#     败→[ -f $REPO/.git/MERGE_HEAD ] && merge --abort；die 5
#     （非零退→网关 emit merge_conflict＝「留 merge_conflict」钦定径）
```

- 身份：`-c user.*` 行内钉死——不赖仓/用户配置，scratch 测试笼同律。
- 并发：B1 项目部署锁串行已保——脚本不加锁不轮询。
- 不推 origin：与旧配方同——integrate 只更本地 master，origin 同步非
  本步语义。

## 5. 配方改动与生效链

- `config/gateway.textproto` L36：`integrate_cmd` →
  `bash /opt/meta/scripts/meta-integrate.sh`（仓内固定件绝对径——与
  deploy/probe 同形制；票 clone 拿不到它）。
- 生效链：子票并 master → deploy 步 meta-deploy.sh（`scripts/` 在
  stamp 码面 → 换装重启）→ `serve --config /opt/meta/config/
  gateway.textproto` 重种子 project 表（UpsertProject ON CONFLICT 全列
  更新）→ 新 integrate_cmd 生效。
- **记档边界（config-only 陷阱）**：若将来纯 textproto 改动无码面随行
  → stamp 不变 → meta-deploy.sh 幂等跳过不重启 → 配方**不生效**——
  config-only 变更须手工 `systemctl --user restart meta-gateway`。
  本票脚本与配置同行不受影响。
- **鸡生蛋诚实边界**：0041 自身部署仍跑**旧** integrate_cmd——届时
  master 若再分叉同败；新脚本上线后下一次补发自愈。结构性竞速无法
  在本票内消灭，如实记。

## 6. 四场景对抗锚（子票锚的母本）

| 场景 | 构造 | 判 |
|---|---|---|
| 污染 ref | `t/<tid>-x`＋`哨兵已双发。等` 类拼接串 | 拒步≠0、master HEAD 字节不动、MERGE_HEAD 不生 |
| 分叉并合 | master 与交付支各独有 ≥1 commit | 退 0、支顶（含判决档形 commit）在 master 可达、merge commit 双亲在 |
| 已并幂等 | 支顶已 ancestor | 退 0、HEAD 不动、无空合并（`%P` 单亲） |
| ff | 线性可快进 | 退 0、HEAD==支顶、无 merge commit |
| 变异对抗 | 删/翻字节集闸 → 污染串直达 git 必红；`--no-ff` 翻 `--ff-only` → 分叉场景必红 | 施工摘要引变异实测红录 |

**实战尾锚**（非施工所能控，按票面记因径）：0037-p2 现卡 待部署，
`spawn_needed` 补发环在转——新径上线后首个 CTO-B 部署执新脚本走通
待部署→部署中→完成 事件链为终证；若壳配额未复窗撞上，按实情记因
不归票红。

## 7. 引用

- `config/gateway.textproto` L36-40（integrate_cmd/sandbox）；
  `internal/gateway/exec.go` L218-222（META_* envs）、L337-351
  （lastDeliverRef 兜底径）、L171-179（integrate 非零→merge_conflict）；
  `config/caster/cto-deploy.md`（META_EXEC ref 可省约定）。
- 实证：`run/exec/0037-p2-integrate-1793597920196294120.log`（ff fatal
  原文）；0037-p2 events 3125/3144/3146/3157/3194；本席 `merge-base`
  当下复核（§0）。
- 前案：`docs/实现/设计-0034-p1-p1-部署通道批rebase.md`（权宜径＋
  「唯配方改可根治」记档）；`docs/实现/票-0040-批rebase解部署通道.md`。
- 坑档：`docs/经验库/2026-11-02-哨兵长行截断-spec路径半行-NULL票.md`、
  `2026-11-02-哨兵spec文件须在宿主可读径.md`、`2026-11-02-devin-
  stdout块缓冲-哨兵全季末落.md`。
- 禁区#8（配方径部署）、决策81（master 不回退）、R2（零轮询零定时）。
