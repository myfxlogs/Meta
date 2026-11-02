# 票 0021 判决：通过（审计席·devin 独立实例，2026-11-02）

- 审计 run：`66cc91f5413896e0`（shell=devin/swe-2-max），触发＝deliver
  （`7d8be8a` claude exit=0 有 result 帧）
- 施工 run：`d7a9546ac877d455`（claude/glm-5.3，接替席——前席 `9e7db341`
  完成 `a275f9c` 后 SIGCHLD+EOF 席死未交付）；交付＝两 commit
  `a275f9c`+`7d8be8a`（`t0021:` 前缀 ✓）；支 `t/0021-notify-split`
  推远端=7d8be8a（`git ls-remote` 亲证）；`git diff master...` 19 文件
  +774/−126
- 票面不在仓 md 而在 DB spec＋备案档 `c/0019-design:docs/实现/票-0021-表外
  通知与CTO分线.md`——与派发 prompt 内嵌 spec 逐节吻合（票-0008 备案先例）
- 复算方法：票面 §6/§7 全部锚本席在交付 worktree HEAD 实跑；**对抗锚三变异
  本席亲手实变异实跑**（不复述施工记录），每次复原 `git status` 零脏亲证

## 逐锚对账（锚 A1–A7 全绿）

| 锚 | 机器证据（本席实跑） | 判 |
|---|---|---|
| A1 默认通知 | `go test -count=1 -run TestDefaultNotify ./internal/caster/` PASS——种子表外 inject_failed→notify≥1；同批同 (kind,t1) 第二枚不报（notify=2 恰为 t1首枚+t3）；progress 非种子零；行匹配 spawn_needed 不回落（caster.go:225-279 matched 追踪＋defaultNotify） | 绿 |
| A2 手工路由转正式 | `TestProdConfig` PASS——audit_dead 行 notify＋dispatch 审计 prefer_shell 非空；shell_limited 行 notify 在；spawn_needed 拆 CTO-A/CTO-B 两行各指 cto-design/cto-deploy.md；`note_contains=="CTO"` 旧行存在即 Fatal；default_notify 块在 kinds 非空（caster_test.go:530-584） | 绿 |
| A3 分线正确 | `TestSpawnNeededCTOAB` PASS——真表直载：CTO-A note→Dispatch{Role:"CTO"} 且渲染 prompt 逐字节＝cto-design.md 源；CTO-B→cto-deploy.md；旧格式「对账补发: CTO」两行皆不中→落 default_notify 报 1 枚 | 绿 |
| A4 设计死归队 | `TestDesignDeadRespawn` PASS——PhaseDesigning 票 SIGKILL 死席→onRunDead→PhaseCreated（design_dead 边，db/phase.go:71 钦定边在表）＋ticket_event spawn_needed note 含「CTO-A」（attach.go:500-502,539-543） | 绿 |
| A5 对账口 | `meta reconcile --socket` 子命令在（main.go:98,827-854 adminCaster.Close＋reconcileCmd）；本席**独立 scratch 冒烟**：交付支编 /tmp/meta-t0021＋`serve --config 仓内 gateway.textproto --board-listen ""`→`reconcile ok 对账完成` rc=0；`TestReconcileRPC` PASS——已立票补发 spawn_needed(CTO-A)、活席收养、二次调用相位不动 run 不死（admin.go:168-175 调既有 g.reconcile） | 绿 |
| A6 D-35 收口 | `TestConnClose` PASS——fakeAdmin Close 计数：Once 径=1；Run 径流杀重连两轮 Dial→Close=2（caster.go:135,175 defer cli.Close()；AdminClient 接口加 Close caster.go:24-28） | 绿 |
| 对抗锚三变异 | **本席亲手实跑全红**：①`if matched && false`（保编译）→`FAIL TestDefaultNotify: notify=0 预期 2`；②真表 CTO-A 行模板改 cto-deploy.md→`FAIL TestProdConfig: CTO-A 行模板="caster/cto-deploy.md" 预期 caster/cto-design.md`＋`FAIL TestSpawnNeededCTOAB: CTO-A prompt 非 cto-design.md 源` 双红；③注释 attach.go:500-502 PhaseDesigning 支→`FAIL TestDesignDeadRespawn: 票 td 相位=设计中 预期 已立（10s 未达）`＝原病复现。三复原 git status 零脏 | 绿 |
| A7 全量 | `go build ./...`✓ `go vet ./...`✓ `go test -count=1 ./...`＝**155 测/15 包全绿**（与施工申报一致）；`gofmt -l internal/ cmd/` 零输出；`bash scripts/selfcheck.sh` rc=0 零禁式；`grep CTO-A|CTO-B|default_notify config/caster.textproto`＝行 39/53/54/67 | 绿 |

## 范围与禁线核对

- **范围内全中**：proto（default_notify＋Reconcile rpc）＋pb 三件 regen、
  caster.go/caster_test、gateway 七件、cmd/meta/main.go、config/caster.textproto
  ——一一对票面 §3。`gateway.go`+`ceo_test.go` 为 `Reconcile→reconcile` 改名
  机械连坐（新 RPC 同名避让），属 admin.go 实现件的编译必需。
- **范围外零碰**：边表/相位机/健康迁移未动；seat/adapter 未动；生产
  /opt/meta 配置与 DB 未碰（本席 reconcile 冒烟走 /tmp scratch sock）。
- **经验库三文**（变异回退/pb 插件清单/scratch 板口）＝令#E1 坑即写义务件，
  摘要已申报，非蔓延。
- **种子集逐枚对设计 §2.1**：textproto 21 枚与钦定列完全一致；
  spawn_blocked/seat_dead 正确不入（决策 75 钦定），零自增删。
- **发射端分线全改**：grep 全仓 spawn_needed AppendEvent——reconcile.go×2
  走 spawnLine（已立→CTO-A/待部署→CTO-B/余＝roleForPhase 本名）、attach.go
  CTO-B 锁释＋CTO-A 设计死、exec.go spawnLine、ceo.go CTO-A——无裸「CTO」
  note 残留；reconcileActor 回队径 roleForPhase(PhaseCreated)="" 不发 spawn
  ＝设计「CTO 不经此径」属实（master 既有非本票改动）。
- **禁线**：本席只添判决档未碰码（变异即复原）；新码无周期任务/心跳/JSON/
  REST（selfcheck 零命中覆盖）；无第二写者（全经 g.database）；default_notify
  回落与 spawnLine 皆机械查表非语义裁决（禁区#7 合规）。

## 暗债对账（令#E6）

diff 全量 grep TODO/FIXME/XXX/HACK/debug 残渣＝零命中。摘要「下一步」申报
两项与实证逐条相符：**无瞒报**。

## 债项节

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| D-0021a | A5「spawn_dup/去重留痕」腿未显式断言：TestReconcileRPC 证「补发＋活席不双派＋相位不动」，重复 spawn_needed 事件的 spawn_dup 留痕靠既有派发去重闸（不在单测径）——属实，归后续小票 | 已申报 |
| D-0021b | reconcileActor 对账径设计席死→回 已立 后 roleForPhase="" 不发 spawn_needed，须等下一轮 reconcile（重启或 meta reconcile 手动）补发——master 既有行为，活态径已由本票 onRunDead 补齐，对账径延迟属设计默许（「CTO 不经此径」） | 观察 |
| D-0021c | 重复 `meta reconcile` 对已立票每次再落一枚 spawn_needed（事件级无去重），caster 端靠派发去重闸产 spawn_dup——手动调用有界，设计 §2.5 已自认「幂等可重跑＋去重闸兜底」 | 观察 |

## 质量三问（令#E5）

- **最优解**：defaultNotify 复用 execNotify 单径不另起通知链；spawnLine()
  一个 switch 集中分线映射；Reconcile RPC＝既有 g.reconcile 薄壳（同径幂等
  不复制逻辑）；dnSeen map 为降噪最小结构。无更简等价形制可替。
- **第一性**：matched 标志位直击「无匹配→种子通知」语义非补丁；Close 在
  连接取得点 defer＝D-35 根因直修（非靠 GC 兜底）；PhaseDesigning 支补在
  相位 switch 本源非外围 workaround。
- **洁净**：同逻辑无两处；无死码（dnSeen 懒初始化兼容测试直调 process()）；
  无 TODO/调试残渣；textproto 头注与 §14 契约注释随语义同步改。

## 沉淀意见（令#E8，纯意见）

可沉淀：「接替席零信任复验」（前席 commit 当未审码，锚逐个自跑＋变异自证）
与「对抗锚变异必须保编译」——后者已入经验库，前者值得进施工席角色卡。

## 判决

**通过**。八锚本席独立复算全绿（A1–A7），对抗锚三变异亲手实跑断言级红；
种子集与分线映射逐枚对设计零偏差；范围零蔓延；暗债对账无瞒报。D-0021a 已
申报归后续票，b/c 为观察级设计默许项。生产换装（二进制＋真表→重启
caster→meta reconcile）按票面属部署径，归 CTO-B 后续票。
