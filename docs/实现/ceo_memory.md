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
