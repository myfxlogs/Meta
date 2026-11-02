# 设计-0034-p1-p2：rollback 按 MERGE_HEAD 分径＋rollback_aborted 事件面

> 需求票 `0034-p1-p2`（巡检席自立案）→ CTO-A 设计 → 子票 0041 施工。
> 钦定轨（§1.4）：设计档 commit → 本票八栏回填 → 子票立＋派 → Deliver。

## 0. 实证链（复核全绿——patrol 诊断成立；残留已被手并消解）

- 配方实证（`config/gateway.textproto`）：
  - :36 integrate_cmd＝ref 校验（exit 92）→ ancestor 短路 → `merge --no-ff`
    ——T1/0040 改版后的 no-ff 三方合，冲突中断会留 `MERGE_HEAD`＋UU 半合并态。
  - :40 rollback_cmd＝**无条件 echo**「本部署无破坏可回（merge 已并历史
    不回退——决策81）」——前提＝「merge 已并历史」，与冲突中断态失配。
- 事件实证（0034-p1 事件史，本席逐行复核）：
  - 4120/4121 integrate exit=92 `REF_LOST:`——竞态噪声（ref 写入瞬间撞
    校验段，13ms 后 4122 重试即解析出 035faf1b），票面定性对，不立案。
  - 4122/4123 integrate exit=1 `CONFLICT:`——真冲突（债务台账.md 双侧
    追加撞号），属施工可解范围，非本票对象。
  - **4128 rollback exit=0**——无条件 echo 在 MERGE_HEAD 活态下假成功；
    此时 .git/MERGE_HEAD=035faf1b、债务台账.md UU、index 带半合并产物。
- 根因链：ff-only 时代 integrate 红不落态（仓库零改动），rollback 空转
  无害——决策81 文案正确性依赖此前提；no-ff 改版（T1/0040）引入
  「冲突中断留半合并」新态，rollback 配方未同步升级＝前提漂移。
- 残留处置现状（本席亲核）：**已被部署席手工 merge 消解**——主仓
  `2b5c700 合并 0034-p1 笼身份修复（判决+CEO验收通过 035faf1）`；
  当下 `git rev-parse -q --verify MERGE_HEAD` 解析败、`git ls-files -u`
  ＝0。绕闸径已记档 D-79（部署席手并 master——动机对径不对，另案）。
- 结构风险仍在：0036/0037-p2/0038-p1 仍待部署，其交付支未 rebase
  （0040 批 rebase 票已作废）——下次 integrate 再撞冲突，无条件 echo
  假成功必复现，脏树拦主仓一切 git 操作（含部署闸自身）。修复有即时对象。

## 1. 残留脏树人工处置决（票面§3 第三件——被现实接管，本席追认）

- 票面选项：abort 后批 rebase 重出（0040 流程）**或**手工解撞号续并。
- 现实路径：cto-deploy 席选了后者并已落地（035faf1 已并 master，
  2b5c700）。**处置决已被执行完毕**——本席追认既成事实，本票不再处置；
  手并绕闸本身另案 D-79（T4 补充面：部署席提示模板钉死禁手 git）。
- 副产教训配方化：同类残留此后由 rollback_cmd abort 径机械弃净——
  票面设想的「人工处置决」降级为配方默认径，人不再进场。

## 2. 方案：配方分径＋事件面补齐＋单测两径

### 2.1 `config/gateway.textproto:40` rollback_cmd 钦定形

```sh
if git -C /opt/meta rev-parse -q --verify MERGE_HEAD >/dev/null; then
  git -C /opt/meta merge --abort \
    && echo 'ROLLBACK_ABORTED: MERGE_HEAD 冲突残留已弃净——merge --abort 回 merge 前 HEAD（未并历史，决策81 不涉）' \
    || { echo 'ROLLBACK_ABORT_FAILED: merge --abort 失败，半合并残留需人工处置' >&2; exit 1; }
else
  echo 'rollback: 本部署无破坏可回（merge 已并历史不回退——ORIG_HEAD 陈旧指针会错伤他票顶提交，实证见决策81；二进制回退由 upgrade 自检兜底件）'
fi
```

- `-C /opt/meta` 与 integrate_cmd 同款——exec 步 cwd＝deploy 克隆
  （`internal/gateway/exec.go:196 ensureDeployClone`＋`:260 cmd.Dir`），
  主仓操作一律显式 -C，不得依赖 cwd。
- abort 径弃的是**未完成合并**（回 merge 前 HEAD），非已并历史回退——
  决策81 原则不涉；已并历史场景走 else 径行为不变。
- `ROLLBACK_ABORT_FAILED` 非零退＝fail-loud（exec 事件已带 exit=1
  与 tail），不留「想弃弃不掉还退 0」的第二假成功形态。
- 行内字面为钦定基线；施工可在锚内微调措辞，marker 前缀与分径骨架不动。

### 2.2 `internal/gateway/exec.go` 事件面——与 T1 块同位对齐

T1 分径块（:172-185）在 integrate 非零退时记 merge_conflict 并打
REF_LOST:/CONFLICT: 前缀——那是「冲突发生」事件面。**缺的是「冲突
事后处置」事件面**（票面§3 第二件钦定）。同位加 rollback 块：

```go
if req.Step == "rollback" && resp.ExitCode == 0 &&
    strings.Contains(resp.Tail, "ROLLBACK_ABORTED") {
    _ = g.database.AppendEvent(ctx, st.run.TicketID, "rollback_aborted",
        db.Event{Actor: st.run.RunID, Note: "rollback_aborted tail=" + resp.Tail})
    g.emit(Event{Kind: "rollback_aborted", TicketID: st.run.TicketID,
        RunID: st.run.RunID, Role: st.run.Role, Note: "MERGE_HEAD 残留已弃净"})
}
```

- marker 直比＝机械判（禁区口径合法，非语义裁决）；echo 径无 marker
  自然零事件；abort 败 exit≠0 本块不触发（exec 事件已留红痕）。
- 事件流自此成链：`merge_conflict(CONFLICT:)` → `rollback_aborted`——
  巡检可机读「冲突→弃净」闭环；缺后者即「脏树可能无主」可告警。
- 不用 recipe 内 `meta inject` 自报径：配方侧发 RPC 属越界耦合，
  事件语义归网关（禁区#7）；弃。

### 2.3 单测（仿 `t_t1_test.go` 形制，新档 `t0041_test.go`）

- 真命令径：test 从 `config/gateway.textproto` 读出 rollback_cmd
  （prototext 反解或行取），`/opt/meta` 字面置换为 fixture 仓径后
  `bash -c` 执行——锚的是**真配方**的分径骨架，非副本。
- abort 径 fixture：git init＋双支制造冲突 merge（留 MERGE_HEAD＋UU）
  → exit=0、输出含 `ROLLBACK_ABORTED:`、事后 `rev-parse -q --verify
  MERGE_HEAD` 败、`status --porcelain` 净。
- echo 径 fixture：无 MERGE_HEAD → exit=0、输出含决策81 文案、
  仓库零改动。
- 事件径：rollback 步 tail 带 marker → ticket_event 增
  `rollback_aborted` 行；无 marker → 无此行（driveSeat 或
  TestT1_IntegratePrefix 同位判读形制）。

## 3. 范围外与记档

- 不动 integrate_cmd／决策81／check·deploy·probe 配方；不代解任何票
  冲突本体（台账撞号顺延＝各交付支的活）。
- **T4 补充面去重**：台账 D-78（rollback 假成功行）修法栏原写「T4
  补充面（MERGE_HEAD 分径）」——本票即该修法的落地载体（票面早立于
  并案前）；子票交付顺手把该行标 done:票0041＋`外派施工提示词.md`
  T4 补充面第 1 条标已落地，T4 届时做 D-77/D-79 两件即不重。
- **台账 D-78 撞号记档**：台账现存两行 D-78（deploy_failed 补发断路／
  rollback 假成功）——编号撞车，后续清账以行内容区分；本票只动
  「rollback 假成功」行。撞号本身留 CEO/台账票另裁。
- 0040 作废后的批 rebase 缺位（0036/0037-p2/0038-p1 待部署支仍过期）：
  另案缺口，记档供巡检/CEO——本票不扩立。
- master 移动竞速同 p1-p1 记档：修的是「冲突残留处置」，不解
  「为何冲突」——结构性复发面（master 文档高频直提×支过期）依旧，
  唯 integrate 配方级改可根治，留 CEO 裁。

## 4. 验证主径

零实现码（本票），锚全机器可判（详见子票§6）：真 rollback_cmd 两径
fixture 红绿、`rollback_aborted` 事件落行、对抗锚＝删 exec.go marker
判读块→事件测红／配方回退无条件 echo→abort 径测红、`go build/vet/
test ./...` 全绿、主仓 `MERGE_HEAD` 解析维持败。
