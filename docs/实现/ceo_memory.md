# ceo_memory（自举期迁移源）

> 网关立起时一次性导入 `ceo_memory` 表，导入后本文件归档、不再更新（同 `docs/凭证清单.md` 迁移源模式）。
> 每条决策四件套：决策＋理由＋触发事件＋来源标（provenance）。

## 2026-11-01

### 决策 1：接受审计第二轮合议（D1–D11 全同意）
- 触发：设计者合议回写（commit `6c6a9d0`）
- provenance：`system_event`（git log 核验，标记已全撤）
- 理由：D 系与五条红线及既有钦定文无新矛盾；A6 合议版（`budget_frozen`＋CEO 裁）权责一致，优于此间原案

### 决策 2：票 0001（网关骨架）批准进入施工——阶段 0 手工径
- 触发：业主指示「当前席为 CEO 席位，接管并推进」
- provenance：`terminal`
- 理由（自裁依据）：设计经两轮审计＋两轮合议，纸面收益已递减；风险清单「纸面」区三问（壳能否前台 exec／流带不带用量／无交互真不问）只能靠实测消除——最优解是让码回答
- **定向**（对施工）：
  1. 结构优先于性能：验收锚 a–h 八用例全绿即成功，不追求实现完备度
  2. 零第三方依赖外延：只用 Go 标准库＋grpc/connect/sqlite 驱动/protobuf（禁区允许清单）
  3. 锚即测试名：测试用例与票面锚一一对应，缺一边＝红
- 施工壳：**devin**（`swe-2-high`，沿业主令钉免费档）；备援按 §7 序 claude → opencode
- 设计引用钉：`6c6a9d0`（文档钦定态——总设计/禁区/角色卡/票面俱在其中）

### 决策 3：施工隔离形制（阶段 0 无网关时的手工等效）
- worktree＝`~/meta-worktrees/t0001`，支 `t/0001-gateway-skeleton`，**不并 master**
- 施工日志＝`~/meta-worktrees/t0001.devin.log`（全量 stdout/stderr，CTO 裁决与事后取证用）
- 用量账＝`--export ATIF` 落 `~/meta-worktrees/t0001.atif.json`（网关立起后补录 `budget`）
- 交付＝推支 `t/0001-gateway-skeleton` → `origin`（github.com:myfxlogs/Meta）＋业主会话验收（票面交付形制原样）

### 决策 4：施工壳权限档＝dangerous（bypass）
- 触发：业主 terminal 明示「使用 bypass 更好」——在前置事实（smart 档实测哑退、sandbox 缺 bubblewrap 不可用）之后给出
- provenance：`terminal`
- 理由：devin `-p` 无头径实测 smart 档拒一切需确认工具调用；无交互席位要能干活只剩 dangerous。隔离残余＝worktree＋非 root 用户＋网络留通；OS 级沙箱待 bubblewrap（已列待业主裁）

### 决策 5：设经验库（业主令）
- 触发：业主 terminal「项目需要 1 个经验库，存放所有遇到的坑和经验，遇到就写，不要等统一归纳」
- provenance：`terminal`
- 执行：`docs/经验库/` 一坑一文即写即 commit；总设计 §4 与四张角色卡同步一条义务（〔令#E1〕）；首批三条（devin smart 哑退／sandbox 缺 bwrap／git identity）当日入库
- 理由：席位无状态、实例会死——经验库是「项目仓文档记忆」的落点，与「记忆在 DB 与项目仓文档」钦定一致

### 决策 6：装 bubblewrap，施工壳档位策略定格
- 触发：业主 terminal「装，sudo 密码……」（授权经口头给出，密码不入任何档案）
- provenance：`terminal`
- 执行结果：`bubblewrap 0.9.0` 已装（`/usr/bin/bwrap`）
- **定格**：施工/审计等无头席位起法＝`devin -p --sandbox`（自主档＋OS 隔离，exec 只许写 workspace）；仅当 sandbox 故障时降级 dangerous＋worktree 隔离，且须在当次记录降级原因
- 附带决定：README「接手者按序读」补 `docs/经验库/` 与本文件——决定链对后续接手者可见（业主令：决定要记录，接手者要能知晓）

### 决策 7：设计者离线，CEO 全权期开始
- 触发：业主 terminal「设计者已离线，现在所有事务都需要你自裁，你是 CEO，你有绝对的、最高权限」
- provenance：`terminal`
- **生效内容**：
  1. 合议环节归 CEO——E1/E2 两道业主令由 CEO 追认并入钦定（总设计 §17 登记），后续修订同此
  2. 升级业主的唯一残余＝物理动作（充值/登录授权类）；「两难皆不可逆」也自裁，但须在记录中写明不可逆性评估
  3. 纪律不随权限放大而放宽：留痕、provenance、可逆性原则、机械闸，一样不少——全权的安全性来自这些，不来自信任
- **审计/派工链（设计者离线后的手工径分工）**：
  - 施工＝devin 无头（进行中）；审计＝**claude 无头独立实例**（异壳，禁区#9）——CEO 永不自审
  - 机器记录：网关未立时由 CEO 机械跑票面验证命令、原文落档（跑命令不是审计，判断仍归审计实例——B2 精神的手工等效）
  - 阶段 1 起的技术设计/拆票＝起无头 CTO 实例（CEO 不设计）

### 决策 8：飞书单向通知 stopgap 即刻启用（正式接入仍在阶段 3）
- 触发：业主问「飞书什么时候接入」——真需求是自举期手机可见进展
- provenance：`terminal`
- **两分法**：
  - **正式接入＝阶段 3 不动**（钦定宿主是网关外部面，通道归网关；锚需要全链）。依赖链：0 网关骨架（在跑）→1 壳适配器实测（最大不确定项，风险清单纸面区）→2 三席自举 →3 飞书+CEO面+前端
  - **stopgap＝即刻**：`scripts/feishu-notify.sh` 单向通知——只出站、无守护、无轮询（调用点＝退出钩子/判决落档，事件驱动），凭证读迁移源 env（600 校验），JSON 止于该件（R3 适配层条款），阶段 3 随票退役
- 已实测：首条通知 code:0 已送达业主会话
- 后续 spawn（审计席等）的包装命令一律挂退出钩子：`...; rc=$?; scripts/feishu-notify.sh "<结果>" || true`

### 决策 9：票 0001 施工交付核验＋审计席起法定格
- 触发：施工席退出（EXIT=0）
- provenance：`system_event`（进程退出＋日志/仓核验）
- **交付核验**：支 `t/0001-gateway-skeleton` 已推 origin（`227eaa1`）；提交格式合规；范围零越界（internal/cmd/proto/scripts＋.gitignore）；席位自写 4 条经验库（E1 义务履行）
- **机器记录**：CEO 机械代跑四命令全绿（build/vet/test/selfcheck，rc=0）——`~/meta-worktrees/t0001.machine-record.txt`＋`t0001.code.diff`（4557 行）已落档
- **审计席起法定格**：claude CLI bypass 被 Claude Code 安全闸拦（授权未点名此席）→ 改 harness 原生子代理：默认权限、只读、判决出 stdout＋写 verdict 文件——**比 bypass 更贴合 D4**（读机器记录、不亲自跑）。经验库同日条目固化两席起法分工
- 附：六项特别质询已注入审计（selfcheck 语义改动居首）

### 决策 10：票 0001 判决通过，并入 master——阶段 0 完成
- 触发：正审（devin swe-2-max，sandbox＋stdout 续跑径）出判决
- provenance：`system_event`（判决文件＋日志）
- **判决＝通过，两审独立一致**（正审 devin swe-2-max＋预审 claude；判决档入 `docs/实现/票-0001-判决/`）。正审证据链到 diff 行号：22 边零缺零多、五相位对账逐格、闹钟过期即响、伪连接/收养互证各具名测试、R2/R3 亲 grep 零命中、范围与形制合规
- **已并 master**（`9ea5945`，no-ff）并推 origin
- **遗留清单（阶段 1 票材，出自正审六备注）**：① 锚 4 措辞改「可打开且表数一致」或 `mode=ro` ② 「部署中死→先核回滚态」阶段 0 空真、部署闸票必补 ③ `armAlarm` 两锁间窄竞（幂等，观察项） ④ `已立` 对账发 spawn_needed 系机械延拓、§5.3 补一行更严 ⑤ `cmd/meta` 无直接单测（安全径已端到端覆盖，观察项）
- **网关激活刻意延后**：`meta serve` 待阶段 1 壳适配器就绪再起——骨架的 spawn 会真派席，无适配器＝空转循环；ceo_memory/票面导入 DB 与激活同步做。此为 CEO 决定，非遗忘
- 经验库＋2：sandbox 下 Write 仍被闸（判决走 stdout 重定向）、`-p` 内联提示词撞路径参数（一律 `--prompt-file`）

### 决策 11：引进 O 仓自我审计六项中的两件（令#E4）
- 触发：业主 terminal「看 O 项目自我审计（6 项），有没有必要引进做审计要求——对代码质量的审计」
- provenance：`terminal`
- 源：`/opt/o/skills/阶段交付/SKILL.md` 必备章节六项（背景范围/新增/修改/关键修复决策/验证/状态下一步）
- **裁决：引二婉四**——引进「根因义务」（修复必答「为什么之前会错」，入判决史防复发）＋「交付摘要四节」（stdout 尾产出，形制核验项但非证据）；婉拒四项因 meta 已有更强机制：票面八栏＞复述范围、diff＋范围前置闸＞自报清单、机器记录（B2）＞自报验证、DB 相位＋依赖票＞LOG 状态文件（后者是 v1 死法）
- 落点：`roles/施工.md`（工作法4/5）、`roles/审计.md`（工作法3）、`roles/CTO.md`（A 唤起拆票·交付形制栏）
- 附带裁定：O 的 13 道 pre-commit 门禁不进 meta 钦定——归 O 配方「检查命令」栏（§11 的用途），登记时自带

### 决策 12：补引 O 质量审计三项＋对抗性锚（令#E5）
- 触发：业主 terminal「我记得有对代码质量做审计——最优解/第1性原则/干净无冗余，这几项没引进？」
- provenance：`terminal`
- 源：`/opt/o/docs/collab.md` §7 交付自检 A–I（上一轮只看了 skills/阶段交付 六项、漏此主清单——CEO 疏失，业主纠正）
- **引进**：A 架构（更简等价解却未用＝败）、B 第一性（直击本质无多余间接层）、C 洁净（无重复/死码/TODO/调试残渣）→ 审计卡工作法第 4 条「质量核三问」；D 之半（对抗性锚「删关键行必红」）→ CTO 卡锚纪律
- **配套闸门**（此三项是语义判断，与「锚机器可判」的共处条件）：质量驳回举证门槛高于锚驳回——须 diff 行＋具体更优替代，说不出替代不得驳「非最优」；病根在设计→票面待裁回 CTO
- **不引**：E 合规（＝禁区＋配方注入）、G 锚对账（＝B2 机器记录，更强）、F 文档同步（＝设计引用＋Yield 设计漂移路径）、H 生产对拍/I 展示同源（O 域纪律，归配方）

### 决策 13：派发票 0002（阶段 1）——壳适配器×3＋路由＋预算＋闹钟
- 触发：业主 terminal「派」
- provenance：`terminal`
- **定向**：① 适配器接口先钉四件（起/流/用量/退码），壳差异止于 adapter/ ② 用量读不到＝如实「不可得」，禁估算（O 诚实纪律）③ live 冒烟与单测严格分离，默认测试零真调用 ④ 壳调用一律 `--prompt-file`（经验库）；三壳无头档位实测结论各写经验库 ⑤ 决策 10 遗留三项并入本票（锚 4 措辞／已立对账行／armAlarm 竞态）
- **降级记录**：施工席起法＝devin `-p --permission-mode dangerous`（非 sandbox）——t0001 审计实证 sandbox 下 exec 全废（bwrap 每命令失败），施工离不开 exec；业主对施工席 dangerous 有点名授权（决策 4），worktree 隔离照旧
- 设计引用钉：本 commit；票面＝`docs/实现/票-0002-壳适配器与路由.md`

### 决策 14：零暗债规则（令#E6）＋债务台账立账
- 触发：业主 terminal「是否有留债务是否要进审计门槛？」
- provenance：`terminal`
- **裁决：门槛审「瞒报」不审「无债」**——无债不现实（备注/遗留是工程常态），可审判的是隐瞒；债的处置权在 CEO/CTO（拆票时消化），不在审计
- 四条规则：施工全量申报（漏报＝驳回，瞒报重于缺陷）／审计暗债对账（对不上＝驳回，对得上＝判决债项节）／CEO 判决债项入账＋派票前过账（再延须理由）／无定时巡账，检查点＝派票事件（R2）
- 落点：`docs/实现/债务台账.md`（种子 8 条：票 0001 正审备注 5＋质询 1＋决策 10 激活延后＋凭证迁移）；三卡各一句（施工申报/审计对账/CEO 过账）

### 决策 15：devin sandbox exec 全废——根因确证，三方案待业主明示
- 触发：业主问「sandbox 下 exec 全废如何解？是否有影响？」
- provenance：`terminal`
- **根因**（已实测）：Ubuntu 24.04 `kernel.apparmor_restrict_unprivileged_userns=1` → bwrap `setting up uid map: Permission denied` → devin sandbox 内 exec 逐条失败
- **方案**：A 系统级放开（快/面宽，恢复旧默认态）｜B 仅 devin/bwrap 建 AppArmor profile（慢/面窄，官方推荐）｜C 维持现状（审计席只读＋stdout——D4 本就禁跑命令，exec 废对审计席是完美形态；施工席 dangerous＋worktree 已授权）。CEO 试 A 被分类器拦（Security Weaken 未被业主点名——拦得对），**动系统级缓解须业主明示**，CEO 建议：现阶段 C 已受控，阶段 2 网关统一 spawn 席位时按「沙箱形制」一并定型（届时 B 最符合无人值守的最小权限）
- **影响评估**：在飞票 0002 无扰（施工 dangerous）；对审计无实质影响（只读设计）；风险仅在于 dangerous 档施工的 OS 级隔离缺位——由 worktree＋非 root＋网络留通的三层残余承担，已知悉已记档

### 决策 16：债的生命周期——清债与销号（令#E7）
- 触发：业主 terminal「如果有留债，如何处理清债和消号？」
- provenance：`terminal`
- **相位同构于票**：open→planned:<票>→（判决通过）→done；accepted/ruled 为终态但业主可翻案；再延计数 K 次（缺省 2）后只余「票内清」或「accepted 写明后果」两出口——防腐烂
- **消号三规则**：凭判决销号（清于必填票号＋判决档，无判决＝无效）／只搬不删（审计面可回溯）／分工三不（施工不见台账防挑活、审计只验不搬、CEO 机械搬运；网关时代两事件机械联动）
- **验债即验锚**：拆票时把债写成验收锚——债清不清由锚判，不由自述
- 落点：`docs/实现/债务台账.md`「清债与销号」节

### 决策 17：opencode＝deepseek v4 flash free，每日限量——三条对策
- 触发：业主 terminal 告知配额特性
- provenance：`terminal`
- **影响评估与对策**：
  1. **在飞票 0002 有界**：冒烟票面钦定「各壳至少一次」、prompt 冒烟级；席位三壳实测已过，余量≈1–2 次小会话。若施工中途撞限＝②b 配额径——**这本来就是它要实现的处置路径，撞上了正好是活考题**（标不可用→换壳→懒探次日自愈）
  2. **审计零消耗**：B2 钦定审计读机器记录不重跑——正审不烧任何壳的配额
  3. **路由缺省固化**：opencode 在全部缺省序已居尾位（仅 devin+claude 双死才启用）；其登记行须加**低预算帽缺省**＋**每日限量特性标记**（②b 标死后懒探，次日自然恢复，无定时器——R2 合规）
- **衍生债（D-9）**：壳登记表/预算缺省必须**配置化**（textproto，R3 允许形）——若 0002 把缺省硬编码，此债升级为网关激活前必改

### 决策 18：施工席无 OS 隔离的风险账（评估入档，选型待业主）
- 触发：业主 terminal「sandbox 不用了？施工无 OS 隔离风险很大吗？」
- provenance：`terminal`
- **爆炸半径**（devin dangerous＝mluser 全权＋网络开）：可读 ~/.ssh（GitHub 私钥）、`/opt/meta-archive/feishu-manage.env`、**/opt/o/secrets 与 /opt/o-research/env（O 生产凭证+金融 DSN）**、本会话转录（含业主 sudo 密码——**视 sudo 为已暴露**）；可写 ~/.config 与 systemd --user 持久化；可外传
- **概率**：今天低（prompt 受控、仓内容自营）；**阶段 3+/登记 O 后不可接受**（施工读外仓内容跑外仓测试＝提示注入真实面——A7 注入链为此而设；禁区#18 恰是此闸）
- **结论**：非「要不要」而是「O 接入前哪点做」。三层立即动作：① 降权用户 `metaseat`（不动系统策略，砍掉凭证失窃这一最大半径，下次 spawn 起用——A7 降权与只读的 OS 落地）② sudo 密码轮换（在对话与本地转录出现过；建议 sandbox 定型、最后一次 sudo 后轮换）③ sandbox 方案 A/B 待业主一字令
- 自查：rtk tee 日志零命中密码（记过滤输出不记命令串）；暴露面=会话转录

### 决策 19：metaseat 降权座落成（业主令「建个用户吧」；sudo 密码业主裁不轮换）
- provenance：`terminal`
- **已落地并验证**（矩阵见经验库 metaseat 条）：
  - 用户 `metaseat`（uid 1001，无 sudo 组）；CLI 三壳可达（devin/claude/opencode）；devin 活体冒烟「正常」
  - **隔离六路全拒**：`~/.ssh`、`~/.claude`（转录，含密码）、`feishu-manage.env`、`/opt/o/secrets`、`/opt/o-research/env`、rtk 日志——席位爆炸半径从「mluser 全权」缩到「自家家＋CLI 只读树＋/opt 只读」
  - 收紧副作用：`~/.claude`→700（原 775，原本全机可读——顺手补了个洞）；`/opt/o/secrets` 等三目录→711
- **席位制式（t0003 起生效）**：施工席以 metaseat 跑；施工区＝`/home/metaseat/worktrees` 下**独立克隆**（不共享 `/opt/meta/.git`，主仓对席位只读）；**交付＝CEO 从克隆取提交代推 origin**（metaseat 无 ssh key，推送是 CEO 机械动作）；devin 档位仍 dangerous（用户级隔离即本轮主要收益；OS 级 sandbox 待 A/B 选型）
- **sudo 密码不轮换**（业主裁，接受残余）：残余=转录在 mluser 域可读——metaseat 已验证拒读 `~/.claude`，实际暴露面已缩到「mluser 自身进程」，与既有威胁模型一致
- 经验库＋2（metaseat 三坎、sudo 假阳性）
