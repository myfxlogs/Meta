# 票 0034-p1-p2：rollback 对 integrate 冲突中断态假成功——半合并残留脏树无主

> 巡检席自立案（adjunct，不占相位不重派）。触发事件：4121（REF_LOST，判
> 噪声）＋4123（CONFLICT，真冲突）——诊断中发现伴生真异常入此票。
> 与 0034-p1-p1 同层并列（第三层），非补票链加深。

### 1. 目标

integrate 步（T1/0040 改版后＝no-ff 三方合）遇内容冲突中断时，
rollback 不得再无条件假成功——rollback_cmd 须按 `MERGE_HEAD` 实况
分径：冲突残留态须弃净（merge --abort）或显式告警留处置信号；
本轮已残留的半合并脏树须获人工处置决（CTO/施工裁）。

### 2. 为什么

- 事件 4120/4121 exit=92「REF_LOST」＝**竞态噪声**：13ms 后 4122 重试
  即解析出 `035faf1b` 并进真 merge；该 ref 在 `t/0034-p1-cage-identity`
  活且已推 origin——ref 未丢，钦定码语义与实况不符（verify 段撞 ref
  写入瞬间）。不因它立案，票面附记防再惊动订阅。
- 4122/4123 真 merge 撞 `docs/实现/债务台账.md` 内容冲突：HEAD 侧
  D-69~D-77 vs 分支侧 D-69 撞号——冲突本体在设计预期内（设计档
  预判触面唯一＋顺延改号），属施工可解，非本票对象。
- **真异常**：4128 rollback exit=0 假成功。`rollback_cmd`
  （config/gateway.textproto:40）＝无条件 `echo 本部署无破坏可回
  （merge已并历史不回退…决策81）`——本轮 merge 因冲突**未**并历史，
  前提失配。实证残留：`.git/MERGE_HEAD`=035faf1b 活、
  `docs/实现/债务台账.md` UU、index 带半合并产物（M exec.go、
  M deploy_test.go、A 判决/经验档）。主仓工作树脏且无主。
- 执行席 CTO-B（c793cd17，pid 4045057）进程已逝而 runs 仍 dead=false
  ——D-76 在账债（对账/停滞检测面）又一实证，不重复立案；事件 4128
  后无席位动作，票 0034-p1 悬「部署中」。
- 根因：integrate 从 ff-only 改 no-ff 三方合（textproto:36），引入
  「冲突中断留半合并态」新态；rollback 配方未同步升级（仍是决策81
  时代无条件 echo）；T1 退码分径（internal/gateway/exec.go:172-178）
  只改事件前缀（REF_LOST:/CONFLICT:）未改事后处置。旧时代 ff 红
  不落态故 rollback 空转无害（0038-p1×2、0040 integrate 档实证）；
  新时代每次冲突必现脏树。
- 危害直读：脏树拦主仓一切 git 操作——包括 0040 施工支自身的
  rebase（workdir=/opt/meta），不修则批 rebase 票落地即卡死。

### 3. 范围内

- `rollback_cmd` 按 `git rev-parse -q --verify MERGE_HEAD` 分径：
  有（冲突残留态）→ `git merge --abort`（回 merge 前 HEAD，不动
  历史，无决策81 ORIG_HEAD 风险）＋事件记 rollback_aborted；
  无 → 维持现文案 exit=0。
- T1 分径语义补「冲突中断」事后处置（事件面与 rollback 面对齐）。
- 本轮残留脏树的人工处置决：CTO/施工裁——abort 后批 rebase 重出
  （0040 流程）或手工解 D-69 撞号（分支侧顺延改号）续并。

### 4. 范围外（禁线）

- 不动决策81「不回退已并历史」原则——merge --abort 弃的是未完成
  合并，非历史回退；已并历史场景行为不变。
- 不代解冲突本体（台账撞号顺延＝施工支的活）。
- 不碰 D-76 对账/停滞检测面（另债在账）；不处理 CTO-B 僵尸 run
  标记（同属 D-76 面）。

### 5. 设计引用

- `docs/实现/设计-0034-p1-p1-部署通道批rebase.md`（触面预判、
  D-69 撞号顺延）。
- 决策81（ORIG_HEAD 陈旧指针错伤——rollback 现文案的来历）。
- 决策92（补票链深闸——本票与 p1-p1 同层并列，非加深）。
- T1 锚：`internal/gateway/t_t1_test.go:150-160`（退码前缀分径
  形制，本票单测仿此）。

### 6. 验收锚（机器可判）

- 新 rollback_cmd 在「MERGE_HEAD 活」场景：exit=0 且事后
  `git rev-parse -q --verify MERGE_HEAD` 解析失败（树净）。
- 「无 MERGE_HEAD」场景：维持现文案、exit=0、仓库零改动。
- 单测覆盖两径（仿 t_t1_test.go 形制，rollback 分径红绿各一）。

### 7. 验证命令

```bash
grep -n "rollback_cmd" config/gateway.textproto
go test -count=1 ./internal/gateway/ -run 'Rollback'
/opt/meta/bin/meta events --socket /opt/meta/run/meta.sock --ticket 0034-p1 | tail -5
```

### 8. 交付形制

票面八栏全量入库（本文件）；施工后 rollback 分径自证（exec 档两型：
abort 型＋echo 型）；Deliver 摘要四节（新增/修改面、根因、验证引、
下一步）。

## 附

- 优先级 P1（主仓脏树拦一切 git 操作，含 0040 施工支自身）。
- 依赖：0040 批 rebase 流程（残留处置路径其一）。
- 触发：merge_conflict 事件巡检（事件 4121/4123；本席 run=
  2ed7fde3f1e76ef8，provenance=gateway 巡检派发）。

## 判决史

（空——首判前无）
