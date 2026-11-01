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
