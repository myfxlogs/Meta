# 票-0044 判决（部署态判双径失守三补）——审计

支 `t/0044-deploy-verdict-drain`｜deliver_ref=`5cc184ff84f8e2ed5788838d92627dccecaaa681`｜基 master=`e28a1a4`（支内经 `31cf3b9` 跟进 master，无冲突面）

## 判决：通过

## 逐锚对账（每条只信 diff 与机器记录）

- **锚1 build/vet/test/gofmt**：PASS（注记）。本席在交付树复算：`go build ./...` rc=0、`go vet ./...` rc=0、`go test -count=1 ./...`＝280 passed/16 pkg（与施工 atif step55 机器记录 280/16 一致）；`gofmt -l internal/`＝15 件——经与 master 基线同款输出 sort+diff 比对**零增量**（adapter/claude.go、caster.go、db/msg.go、db/phase_test.go、db/t0036_test.go、db/ticket.go、feishu 三件、feishu_card.go、gateway.go、inject.go、t_t18_test.go、t_t2_test.go、upgrade/t_p3_test.go），本票触碰五件 `gofmt -l` 零输出——字面零命中不达系 master 存量脏区，照票-0010/0011/0034 先例判绿记观察。
- **锚2 attach 径哨兵定边**：PASS。`attach.go:584-597` 两级判前置——`drainDeploySentinel(st.run.RunID)` done→`deployDone`（完成边+deploy_ok+双锁释+消闹钟，deployverdict.go:58-71）；failed→note 改记「哨兵在盘 META_DEPLOY:failed」。`t0044_test.go:100-135` AttachDrainDone 真实径：DispatchSeat→部署中→SIGKILL→waitPhase(完成)＋`EdgeDeployDone` 在案＋`emitted(deploy_ok)`＋tk44b 派成（双锁释实证）；`:140-163` AttachDrainFailed 全绿桩布景下 failed 仍翻负＋note 含「哨兵在盘」。
- **锚3 attach 径态判兜底**：PASS。`:168-190` AttachStateOK 无终哨兵＋三要件齐（已装行/戳对/Probe 桩绿）→完成边+deploy_ok；`:194-238` AttachStateRed 三子案（无已装证据/戳不符/Probe 红）各翻→deploy_failed＋note「先核回滚态」原文案不变（D-2 不退）。
- **锚4 reconcile 径排干**：PASS。`reconcile.go:198-215` 死 run→drainDeploySentinel→done→deployDone，sentinel=="" 才落 deployStateOK（排干先于态判，Probe 结构性红不挡）。`:242-266` ReconcileDrainDone 以 Probe 桩红布景判完成；`:270-304` ReconcileDrainFailed 两案（failed→「哨兵在盘」note；无终哨兵+态判无据→「先核回滚态」note）。
- **锚5 deployStateOK 换源**：PASS。deployverdict.go:128-156 二取一（exec 日志已装行首选 deployverdict.go:162-198 glob 取最新份/末条已装行/剥（prev=…）尾/相对径 newBin 对 repo_url 解析；cmd_run exit=0 旁证 install 位落 META_INSTALL 或 /opt/meta/bin/meta 缺省）；`:87-99` expected() 期戳首选 NewBin version 次选 code-head+-deploy（pathspec `*.go proto cmd internal scripts go.mod go.sum` 与 meta-deploy.sh:18 逐字同源）；`:113-121` 装戳判等+Probe。`:309-363` DeployStateOKResourced 四案全过（零 cmd_run+已装行+戳对→true；新份顶掉已装行→false；戳异→false；cmd_run 旁证→true）；t_p3_test.go 换源口径改造未弱化（仍断言无据翻负）。
- **锚6 对抗提名位（删/翻必红）**：PASS。①删 attach 排干调用→AttachDrainDone 红（施工 atif step57 实测）；②翻装戳判等 `cur != want`→`==`→DeployStateOKResourced＋TestP3_DeployStateOK 双红（atif step57 实测）；③删 `g.emit(deploy_ok)`→AttachDrainDone:130＋AttachStateOK:188 双红（本席交付树突变复算）；④翻取行前缀 `META_DEPLOY:`→`META_EXEC:`→AttachDrainDone/Failed＋ReconcileDrainDone/Failed 四红（本席交付树突变复算）。
- **锚7 签名绝迹**：PASS。AttachDrainDone `:126-128` 断言物理全绿布景（已装行+戳对+done 在盘）席死后 `EdgeDeployFailed` 不在事件史。

## 范围核对

diff 六件：`internal/gateway/deployverdict.go`（新件）、`attach.go`+19、`reconcile.go`+25-30、`t0044_test.go`（新）、`t_p3_test.go`+15-3、`docs/经验库/…mkToDeploy…mkTicketAt.md`。全在票面 §3 授权面（含「gateway 包新文件」许可与「测试面沿用 internal/gateway 约定」）；经验库坑档系令#E1 同生义务（0038-p1 判例同款记范围内）。proto/RPC 零变更；handleExec 三重闸、哨兵契约文本、meta-deploy.sh 幂等闸、p1/p2/p3 修复件语义、非部署相位席死径——diff 实证均未碰。支名/worktree/commit 格式 `[角色:施工][0044] <为什么>`/推支/`META_DELIVER:5cc184…` stdout 独占行＋sentinel.log 双发（run/artifacts/65df1a0f605edabb/sentinel.log 亲见）——交付形制齐。

## 偏差/债项

- **O-1（观察项，非缺陷）**：`deployverdict.go:68` 显式 `releaseProjectLocks` 与 `transition` 自带释锁（`gateway.go:639-644` 出离部署中即释 integrate+deploy）语义重复——设计 §2 钦定序照做的双保险；本席删该测试仍绿（锁释语义由 transition 兜底＋tk44b 派成断言仍覆），方向保守无害。
- **瞒报对账**：diff 零 TODO/FIXME/绕过/半成品（grep 实证）。摘要「下一步」逐项对账：0043 支 reconcile 同区并序冲突（真——0043 仍待部署未并，其 2017c51 改同文件同区，归 CTO 并序裁）；设计 §6 残险照做未扩修（真）；非部署相位排干不挂（diff 实证只挂 PhaseDeploying 两径）；stdout 块缓冲未碰（diff 实证）；`META_INSTALL` env 复用部署脚本同名缺省（deployverdict.go:151-154 对 meta-deploy.sh:10）；gofmt「基线 12 件」申报与实测 15 件微差（T18 等在其施工窗口并入 master 所致，「存量非本票带入」口径成立——基线 diff 零增量亲核）。**瞒报零。**

## 独立复算

本席在 `/opt/meta-wt/0044-deploy-verdict`@5cc184f 复跑（go1.27.2）：build/vet rc=0；`go test -count=1 ./...`＝280 passed/16 pkg；`-run 'Deploy|Drain|Sentinel|StateOK|Death'`＝17 passed；`TestT44|TestP3_DeployStateOK` 集＝11 passed；gofmt 五件零输出、基线 diff 零增量；突变三发本席实测（emit 删→2 红、取行前缀翻→4 红、释锁行删→绿等价赘件）。施工机器记录经 atif 逐步核：step40 首跑 12 绿、step42 全测 278 绿、step55 并 master 后 280 绿、step57 两发突变红、step58 gofmt 五件零＋17 目标绿——逐条对上。`sqlite3 cmd_run 0034 deploy`=0 行（atif step61 复跑参照，修复前证据结构性缺席实证）。

## 可沉淀打法（令#E8 附议）

**可沉淀**。两打法俱直击 0034 族根因且本票实证复用：①「死判时刻对在盘终态文件一次末泵排干」——事件驱动单发不违红线，可泛化至其余相位哨兵消费；②「跨 restart 存活证据（落盘日志/装着文件互证）优先于 DB 事后记」——restart 杀记录者场景的唯一可靠证源。建议入经验库定型（本票经验档已记 fixture 坑一件）。

META_VERDICT:pass:docs/实现/票-0044-判决/1-devin.md
