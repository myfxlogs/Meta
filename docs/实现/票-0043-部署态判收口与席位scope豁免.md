# 票-0043：部署收口残面——pending 态判＋证据落档＋收养幂等/扩面

### 1. 目标

补齐 0034-p3 已合并实现（728adeb）未覆盖的三个残面，按设计档
`docs/实现/设计-0034-p3-部署态判收口与席位scope豁免.md` 收口：
① deployStateOK 判据死码→pending 意向书＋装着戳比对为部署收口主判据；
② 自检/回退证据落部署 exec 日志与 last-verdict（票面锚2 钦定行）；
③ 收养幂等（ adopted 事件风暴）＋adjunct 收养扩面＋spawn 探测/杀径加固。

### 2. 为什么

0034-p3 实现已先经并行链落地并入 master（ac21f97→a016b01→728adeb，
审计两骑）：checker transient service＋等死再探＋scope 包席俱达。
但三残面实测俱在：
- `deployStateOK`（reconcile.go）判据两洞——restart 径 `cmd_run` 行
  写不出（runExecStep 的 rec 在 cmd.Wait 后），启动对账先于 g.Serve
  故 Probe 恒红；现收口实靠「scope 幸存席→收养→幂等重跑」，席同死
  仍落 deploy_failed→撞幂等——判据仍寄生席存活，票面钦定未竟。
- checker 输出进 journal/discard——票面锚2「exec 日志内出现健康
  自检绿或回退 prev 行」未满足，回退态无机读档。
- 对账收养无幂等闸：本席亲见票 0034-p3 上 adopted 事件 28 连发
  （事件风暴）；scope 幸存的 adjunct 活席在 D-55 环无人认领；
  scope 包装无预探测（systemd-run 坏则每席起后秒死→全壳冤案）。

### 3. 范围内

- `internal/upgrade/upgrade.go`：Config 加 `Pending/Ticket/ExecLog/
  Expect` 等位；Do 在 install 后 restart 前写 pending 意向书
  （`k: v` 行：ticket/expect/install/prev/exec_log/ts）；CheckCmd
  追加 `--exec-log/--verdict`——结果行（健康自检绿/回退 prev）双写
  exec-log 与 `<rundir>/upgrade/last-verdict`；绿径补
  `meta reconcile --socket` poke 兜底触发。
- `internal/gateway/`：reconcile.go——PhaseDeploying 前置 pending
  消费（读+删，单次语义）＋SelfStamp 比对三态（==→deploy_done；
  ≠→deploy_failed note 带戳；stale→事件留痕），deployStateOK 降为
  次判据保留；收养幂等（st.adopted/run.Adopted 已置则不再发事件）；
  D-55 环扩 adjunct 活席收养。spawn.go——New() 一次探测落
  seatScope 位＋事件；kill 径 `systemctl --user kill --kill-whom=all`
  优先回退 kill(-pid)。exec.go——envs 加 META_RUN_DIR/META_EXEC_LOG。
  gateway.go——Config.SelfStamp＋seatScope＋探测缝。
- `cmd/meta/main.go`：serve 传 `buildStamp` 入 Config.SelfStamp；
  upgrade-check 加 --exec-log/--verdict 旗。
- `scripts/meta-deploy.sh`：upgrade 行加 --pending/--ticket/--exec-log。
- `docs/技能库/二进制换装三件套.md`：自检执行体径更新为 transient
  service（合并码已改，文档未同步——证错条同票修）。
- 新测试沿用 internal/gateway、internal/upgrade 测试包约定。

### 4. 范围外（禁线）

- 不回退已合并件：checker transient service 与等死再探、spawn scope
  包装、upgrade-check 子命令、既有 t_p3 测试语义——修不翻。
- 不改 B1 锁语义、部署四步序/配方步名、proto/RPC 面（pending/verdict
  是文件，触发是既有 reconcile RPC——零协议变更）；不动
  meta-gateway.service、supervise、caster/patrol；exec 代跑步不
  scope 化；无心跳/周期/轮询新件；JSON 不入内。
- 不并修邻票（0038-p1-p2/0037-p2-p2/ddebcb9a）；不 Deploy 不并
  master：交付＝推支＋META_DELIVER。

### 5. 设计引用

- `docs/实现/设计-0034-p3-部署态判收口与席位scope豁免.md`（唯一权威；
  §11 落地对账列明已合并/残面边界）
- `docs/实现/票-0034-p3-deploy步restart杀全席.md`（需求原文+实证链）
- `docs/总设计.md` §5.1/§5.3；merge 728adeb（现基线）与其审计链
- `internal/upgrade/upgrade.go`、`internal/gateway/{spawn,reconcile,exec}.go`、`cmd/meta/main.go`
- `roles/施工.md`（交付纪律）

### 6. 验收锚

1. `export PATH=/usr/local/go/bin:$PATH && go build ./... && go vet ./... && go test -count=1 ./...` 全绿（含 728adeb 带入的既有 t_p3 测试——回归不退）。
2. pending 态判三态：部署中票＋pending{expect=S,ticket=T}＋
   SelfStamp=S→reconcile→完成＋deploy_done；SelfStamp≠S→
   deploy_failed；无 pending→次判据/老径。对抗：判等行翻 != 即红。
3. 单次消费：reconcile 后 pending 文件已删；ticket 不符→stale 事件
   不误判；同 ticket 二跑不重判。
4. checker 证据：stub sock 绿→exec-log 追加「健康自检绿」＋
   last-verdict ok；sock 红→prev 回写＋「回退 prev」行＋verdict
   rolledback；等死再探既有测试不退。
5. 收养幂等：已 adopted 的 run 再对账→adopted 事件不增发（事件数
   断言）；adjunct run＋活 pid＋票非终态→adopted＋MarkAdopted。
6. spawn 探测缝：探测关→argv 无 systemd-run 前缀；开→含
   `--user --scope --unit meta-seat-<runid>`；kill 径断言
   `systemctl --user kill` 优先。
7. exec envs 含 META_RUN_DIR/META_EXEC_LOG；`bash -n` 过
   meta-deploy.sh 且 upgrade 行含三新旗。
8. 对抗提名位（任一反删即红）：pending 消费即删行、收养幂等闸行、
   exec-log 追加行。

### 7. 验证命令

```
export PATH=/usr/local/go/bin:$PATH
go build ./... && go vet ./... && go test -count=1 ./...
go test -count=1 -run 'P3|Pending|Scope|Check|UpgradeCheck|Adopt' ./internal/gateway ./internal/upgrade
bash -n scripts/meta-deploy.sh
bash scripts/upgrade-drill.sh
```

### 8. 交付形制

- worktree `/opt/meta-wt/0043-deploy-verdict`＋支 `t/0043-deploy-verdict-scope`
  （/tmp/wt-p3 起手支已被并行链消化——以 728adeb 为基另立新支）。
- commit 格式 `[角色:施工][0043] <why>`；交付＝推支＋
  `META_DELIVER:<commit>`（哨兵双发：stdout＋sentinel.log）。
- 摘要六节（新增/修改面、根因与机制、验证引、遗留/残险、建议沉淀、
  自评）；逐锚对账行不可缺。
