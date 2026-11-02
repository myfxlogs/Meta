# 票-0043：部署态判收口与席位scope豁免（0034-p3 机制三修）

## 一、目标

实现 `docs/实现/设计-0034-p3-部署态判收口与席位scope豁免.md` 全部四机制，
使「deploy 步 restart 灭 meta-gateway cgroup」后部署仍能机械收口：
部署中票的 deploy_done 由 pending 意向书＋装着戳比对判定（不靠席活），
自检/回退由 transient scope checker 执行（可达非死码），席位包
`meta-seat-<runid>.scope` 免连坐且 adjunct 可收养。

## 二、为什么

票 0034 部署 24 连败：`upgrade.Do` restart 段后自检/回退是死码（进程死于
自发 restart），deploy_done 唯一径寄生于必死之席，网关重启把席位同灭再
误判席死为部署败——三缺互相咬死成循环。巡检已立案（0034-p3），设计档
已 commit 钦定机制；本票是唯一施工落点。起手支
`/tmp/wt-p3`（`t/0034-p3-deploy-verdict`，未提交）可接替参考——其
deployStateOK 两洞（cmd_run 行写不出 / Probe 早于 Serve）见设计档 §0.3，
**判据不可取，骨架可承**。

## 三、范围内

- `internal/upgrade/upgrade.go`：Do 新序（预检提 expect→prev 快照→spawn
  checker→install→写 pending→restart）；新增 `Check`＋`spawnChecker`；
  Config 加 `Pending/Ticket/ExecLog/Expect` 等位；RestartCmd=="" 径保
  原内联 Probe/回退。
- `cmd/meta/main.go`：新 `upgrade-check` 子命令；serve 把 `buildStamp`
  传 `gateway.Config.SelfStamp`。
- `internal/gateway/`：spawn.go（New() 一次探测落 seatScope＋argv scope
  包装＋kill 径 `systemctl --user kill --kill-whom=all` 优先）；
  reconcile.go（PhaseDeploying 前置 pending 态判三态；D-55 环扩 adjunct
  活席收养）；exec.go（envs 加 META_RUN_DIR/META_EXEC_LOG）；
  gateway.go（Config.SelfStamp＋seatScope 位＋探测缝）。
- `scripts/meta-deploy.sh`：upgrade 行加 --pending/--ticket/--exec-log。
- `docs/技能库/二进制换装三件套.md` 证错条同票修。
- 新测试文件（含对抗锚）；沿用 `internal/gateway` 测试包约定。

## 四、范围外（禁线）

- 不改票 0034 交付物语义、B1 锁语义、部署四步序/配方步名、proto/RPC 面
  （零协议变更——pending/verdict 是文件，触发是既有 reconcile RPC）；
  不动 meta-gateway.service、supervise、caster/patrol；exec 代跑步不
  scope 化；无心跳/周期/轮询新件；JSON 不入内（pending/verdict 用
  `k: v` 行）；不并修邻票（0038-p1-p2/0037-p2-p2/ddebcb9a）。
- 不 Deploy 不并 master：交付＝推支＋META_DELIVER。

## 五、设计引用

- `docs/实现/设计-0034-p3-部署态判收口与席位scope豁免.md`（唯一权威；
  时序三径/残险/禁线以该档为准）
- `docs/实现/票-0034-p3-deploy步restart杀全席.md`（需求原文+实证链）
- `docs/总设计.md` §5.1/§5.3（scope+收养钦定、对账=相位×预期行动者）
- `internal/upgrade/upgrade.go`、`internal/gateway/{spawn,reconcile,exec}.go`
- `roles/施工.md`（交付纪律）

## 六、验收锚

1. `export PATH=/usr/local/go/bin:$PATH && go build ./... && go vet ./... && go test -count=1 ./...` 全绿。
2. **态判三态**：部署中票＋pending{expect=S,ticket=T}＋SelfStamp=S
   →reconcile→相位完成＋deploy_done 事件；SelfStamp≠S→deploy_failed；
   无 pending→老径不判。**对抗**：判等行翻 `!=`→用例红。
3. **单次消费**：reconcile 后 pending 文件已删；ticket 不符→stale 事件
   留痕不误判。
4. **scope 包装**：seatScope=on→startRun argv 含
   `systemd-run --user --scope --unit meta-seat-<runid>`；探测缝关→直跑；
   实证契约（前台等待/退码传播）不符→回退直跑＋事件。
5. **checker 双径**：stub sock 绿＋install version==expect→exec-log 追
   「健康自检绿」＋last-verdict ok；sock 红→prev 回写 install＋restart
   调用痕＋「回退 prev」行；两 spawn 径俱败→还 prev 报错（install 未动）。
6. **adjunct 收养**：adjunct run＋活 pid＋票非终态→adopted 事件＋
   MarkAdopted。
7. exec envs 含 META_RUN_DIR/META_EXEC_LOG；`bash -n` 过
   meta-deploy.sh 且 upgrade 行含三新旗。
8. 对抗提名位（任一反删即红）：pending 消费即删行、scope argv 包装行、
   态判判等行。

## 七、验证命令

```
export PATH=/usr/local/go/bin:$PATH
go build ./... && go vet ./... && go test -count=1 ./...
go test -count=1 -run 'P3|Pending|Scope|Check|UpgradeCheck' ./internal/gateway ./internal/upgrade
bash -n scripts/meta-deploy.sh
bash scripts/upgrade-drill.sh   # 既有演练径回归（隔离实例不碰生产）
```

## 八、交付形制

- worktree `/opt/meta-wt/0043-deploy-verdict`＋支 `t/0043-deploy-verdict-scope`
  （可接管 /tmp/wt-p3 起手支改名续作，commit 从干净基底起）。
- commit 格式 `[角色:施工][0043] <why>`；交付＝推支＋
  `META_DELIVER:<commit>`（哨兵双发：stdout＋sentinel.log）。
- 摘要六节（新增/修改面、根因与机制、验证引、遗留/残险、建议沉淀、
  自评）；逐锚对账行不可缺。
