# 票 0034-p2 判决 1-devin：**通过**

> 审计席 1e47866b246c7ffd。对象＝设计票交付（ref=c5f8f03，master 直落）。
> 只信 diff 与机器记录；施工（CTO-A 设计席）自述仅作线索已逐条亲证。
> 本票首判，判决史空。

## 一、逐锚对账（设计票 §6 五锚，全绿）

**锚1：设计档在仓且含实证链/残留三件/记档三段——绿**
- `git show c5f8f03 --stat`：`设计-0034-p2-部署克隆自愈与验树锚补强.md` +82 行随交付 commit 入仓。
- `grep -nE "^## [012]"` → `## 0. 实证链`(L6)、`## 1. 残留三件`(L28)、`## 2. 记档`(L64)；`grep -cE "实证链|残留|记档"`=6。

**锚2：0042 票面在仓且八栏零缺——绿**
- 同 commit：`票-0042-部署克隆自愈与验树锚补强.md` +99 行。
- `grep -cE "^### [1-8]\."`=8（目标/为什么/范围内/范围外/设计引用/验收锚/验证命令/交付形制齐全）。
- 旁证成立：0042 事件史无 inject_failed，dispatch(5955) 成功→ParseTicketMD 实过。

**锚3：DB 票 0042 存在、spec 非空、project=meta——绿**
- `meta phase --ticket 0042` → `0042 施工中 frozen=false`。
- 事件 5953 `project_set→meta`；create_ticket 帧携带 spec 档路径（施工壳 sentinel.log 第2行实录）。

**锚4：0042 事件史含 dispatch/claim——绿**
- 事件 5955 `dispatch 设计中→已派`、5956 `claim 已派→施工中 e274c34c2605fc1c`。
- 进程级实证：`systemctl --user status meta-gateway` 显示 devin 以 `--prompt-file run/inject/0042-施工.md` 在跑；worktree `/opt/meta-wt/0042-deploy-clone-heal` 在 `t/0042-deploy-clone-heal` 支。

**锚5：本票 spec 已回填——绿**
- 事件 5951 `fill_spec 八栏回填`；施工壳 sentinel.log 第1行 `META_ADMIN:fill_spec|.../票-0034-p2-设计票.md`；该档八栏齐全。

## 二、实证链复核（设计诊断正确性——亲证非转述）

- **T13 已并在跑**：git log 见 f22a575→94ac060→9be1623 链；`systemctl` 网关 Active since 16:26:59（晚于票面失败时刻 16:11-16:13）。
- **exec.go 新语义属实**（现档 ensureDeployClone, L293 起）：有目录→`fetch+checkout+reset --hard FETCH_HEAD+clean -fdq`；无目录→`clone -b <branch>`；交付 ref 不再 checkout；函数注释明写 T13 钦定语义。
- **测试锁在**：`TestT13_CloneSync` at `internal/gateway/t_t13_test.go:14`。
- **机制在产实证（强于设计自述）**：`git -C run/deploy/0034 reflog`——16:33:55 `checkout: moving from e109692…to master`（detached 疤形被机制径自愈）→ 16:33:56/16:36:17 `reset→ade5191` → 16:37:47 `reset→c5f8f03`。病根 A/B 双销不是推论，是已发生的机器行为。
- **残留疤形与台账一致**：`run/deploy/0034-p1` HEAD=c21be1a≠支尖 035faf1（待其下次部署机制径自愈）；`债务台账.md:118` D-81=`done:T13已合（f22a575）`。
- **残留三件均属实**：①现码 `os.Stat(dir).IsDir()` 即走同步径，git 步败 `return "",err`——残目录确为永堵缺口（与设计§1.1 同码位）；②疤形 fixture 缺口成立（0042 锚1 自锁）；③`总设计.md:281` 实测仍含「cwd＝网关预检出的交付 commit 干净 worktree」旧文案（grep 命中 :281 与 :379）。

## 三、范围与交付形制

- diff 全域：`git show c5f8f03`＝4 个文档 +379/-0，零代码——与设计票 §3 逐项对得上（设计票不施工，施工链全在 0042）。无范围蔓延。
- 交付形制：设计票 §8 钦定「ref＝设计档 commit」，`META_DELIVER:c5f8f03` 与 commit 逐字一致（sentinel.log 实录）。
- 直落 master 合规：设计/判决类 docs commit 落 master first-parent 为本仓定例（94a0639/16866b2/eac9e3d/208262b/639dd58/841d608/f8e363d，first-parent 共 169 条同类）；施工票 worktree 支形制归 0042（其支已在）。
- 摘要四节形在（atif.json 末帧）：新增/修改面四档与 --stat 全对、根因裁断与本判复核一致、验证引三条皆机器可复、「下一步」＝0042 施工链——与实际派发态对账一致。

## 四、质量核三问

- **最优**：无更简等价径。设计票轨道内不许施工（注入硬约束）；残留三件同函数+同文档域归一张子票为最小拆分，同 0034-p1-p2 先例。
- **第一性**：残留①直击「结构性堵死」同族根因；钦定骨架＝一次 RemoveAll+重克隆+fail loud，不静默降级不循环重试——非补丁摞叠。
- **洁净**：diff 零 TODO/FIXME/绕过/半成品（`git show c5f8f03 | grep` 0 命中）；「记档（不扩立）」节为诚实申报非暗债，与摘要「下一步」对账一致。

## 五、观察项（不构成否决）

1. **16:33 部署已跑出新签名红**：T13 后首次 0034 部署（run 372f84b3606a3e26）克隆同步正确，但 check 红于 `TestT1_IntegratePaths` panic@`t_t1_test.go:101`（addr=0x20＝nil `me.Username`，当时树 ade5191 该行确为裸解引用）——笼内 passwd 无条目同族坑（0034-p1 坑型）。修复 c18ecf4 已落 master（16:40:29，本审计进行中）。**注**：c18ecf4 系直落 master 的代码改，其交付形制合规矩该 commit 所属票的审计裁，本席仅记档示巡检。设计 §2「check 绿」预证随之下轮部署方见分晓。
2. **可沉淀**：本票是「主病灶被在先修复接管→追认既成＋残留拆票」轨第二实例（首＝0034-p1-p2）——打法已复用两次，可入经验库（纯意见，非否决依据）。

## 判决

**通过**——设计票五锚机器全绿，诊断经生产 reflog 实证，残留拆票正确且已真派发。
