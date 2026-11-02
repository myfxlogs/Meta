# 票-0045 判决（哨兵文件泵 inotify 事件源）——审计

支 `t/0045-sentinel-pump-watch`｜deliver_ref=`c39414a8531ca08a2d78e109663915934dbe8cac`｜基 `f306a10`（master 已进至 `80b6dca`——并发票推进，三路 diff 取 merge-base）

## 判决：通过

## 逐锚对账（每条只信 diff 与机器记录）

- **锚1 build/vet/test/gofmt/-race**：PASS（gofmt 字面项照先例记观察）。本席交付树 @c39414a 复算：`go build ./... && go vet ./...` rc=0；`go test -count=1 ./...`＝285 passed/16 pkg（与施工 atif step45 机器输出一致）；`go test -race -count=1 ./internal/seat`＝27 passed（atif step44 get_output 同款）。`gofmt -l internal/` 列 16 件——与 master 基线同款输出 sort+diff 比对**零增量**（adapter/claude.go、caster.go、db/{msg,phase_test,t0036_test,ticket}.go、feishu 三件、gateway/{feishu_card,gateway,inject,t_t18_test,t_t2_test}.go、route/route.go、upgrade/t_p3_test.go），本票触碰面 `gofmt -l internal/seat/` 零输出——字面零命中不达系 master 存量脏区，照票-0044/0034/0011/0010 判例判绿记观察。
- **锚2 静默壳主锚**：PASS。`sentinelwatch_test.go:19-48` TestRun_SilentShellExecWatch：argv 脚本 stdout 全程零行、sentinel.log 写 `META_EXEC:silentprobe` 后 sleep 6；`waitFrame(4s)` 存活期窗内桩网关收 `ExecReq{step:"silentprobe"}`，且断言探得时 run 未返（done 未出码＝watcher 泵实证、非 :320 终泵兜底产物）。施工 atif step31 机器输出 `--- PASS: TestRun_SilentShellExecWatch (6.01s)`；本席 -race 目标件复跑绿。
- **锚3 静默 DELIVER 姊妹锚**：PASS。`sentinelwatch_test.go:51-64` TestRun_SilentShellDeliverWatch——stdout 零行下 sentinel.log 写 `META_DELIVER:silentref`→ExitClean＋`Deliver.ref=="silentref"`；atif step31 `--- PASS (2.31s)`。
- **锚4 半行锚**：PASS。`sentinelwatch_test.go:67-100` TestRun_SilentShellHalfLine——残段 1.5s 窗零 ExecReq（无 `\n` 不消费）；补 `\n` 后 `ExecReq{halfx}` 恰一条（n==1 断言，pairDedup 双发去重不误伤）；atif `--- PASS (4.31s)`。
- **锚5 对抗提名位（删/翻必红）**：PASS（机器实证）。①摘除 watcher 泵调用行（`sentinelwatch.go:84-86` `if need { s.pumpSentinelFile() }`→`_ = need`）→atif step32 `--- FAIL: TestRun_SilentShellExecWatch (4.00s)` 断言级红「静默期 ExecReq 未入桩」；②摘除 pairMu 叶锁（`seat.go:675-676`）→atif step38 `--- FAIL: TestRun_ConcurrentPumpRace — race detected`。watcher 启动行（`seat.go:284`）删除案被①涵摄——泵径整体不活则同一用例必红，变异实证达意。
- **锚6 降级锚**：PASS。`sentinelwatch_test.go:119-146` TestRun_WatchDegrade——ArtifactDir 指不存在目录→`InotifyAddWatch` 败→stderr 留痕「sentinel watch」＋席照常 ExitClean＋stdout 哨兵径 Deliver.ref 不损；atif `--- PASS (0.31s)`。ArtifactDir=="" 面：`sentinelwatch.go:20-22` 首行 nil 直返；init/addwatch/pipe 任一步败各留痕＋nil（`:24-40`）——代码评审证降级不劣于现状。

## 范围核对

diff 五件：`internal/seat/sentinelwatch.go`（新件）、`sentinelwatch_test.go`（新）、`seat.go` +22-1、`seat_test.go` +38-9、`docs/经验库/…变异验证回退…` +13。前四件全在票面 §3「internal/seat（seat.go 或同包新文件）＋测试面」授权面；经验库坑档系令#E1 施工遇坑即记同生义务（0044 判例同款记范围内）。`config/`、proto/RPC、adapter 面零变更（diff stat 实证）；B1 串行闸、scanSentinel 匹配语义、pairDedup 配对契约（原文键/计数增减原行未动——仅外加 pairMu 两行）、sentinelOff 与 `\n` 收口语义、:317/:320 两泵点、META_* 契约文本——diff 实证均未碰。零定时器（`Select` nil timeout 唯一等待形）；零第三方依赖（import 全 stdlib，go.mod/go.sum 零 diff）；禁 fsnotify 合规。挂点：`watchSentinel()` 在 `cmd.Start()` 成功返回后、`go s.pumpGateway()` 同位（`seat.go:284`）；`cmd.Start()` 失败径在其前 return 不起；`cmd.Wait()` 后 `stopWatch()` 停管收口归队再走 :335 终泵——序符票面。锁序亲核：`pumpSentinelFile` 全体入 pumpMu（`:641-643`，sentinelOff/open/seek/read 在罩内）；`scanSentinel` 内 s.mu 各块叶域、pairDedup 自握 pairMu（`:675-676`）且三调用点（`:713/:745/:753`）均不在 s.mu 罩下——`pumpMu → scanSentinel → {s.mu|pairMu}` 单向无回取，全仓 grep 无持 pairMu/s.mu 再取 pumpMu 路径。支名/worktree/commit 格式 `[角色:施工][0045] <为什么>`／推支 origin=c39414a／`META_DELIVER:c39414a` 双发（`run/artifacts/ebe3d4da8955c443/sentinel.log` 亲见＋atif step56 stdout 尾行）——交付形制齐；摘要四节在、逐锚对账在、全量遗留申报在、可沉淀候选在。

## 偏差/债项

- **机器记录换源（记档非缺陷）**：`/tmp/t0045*.machine-record.txt` 未落盘——本席以网关收存 atif（`run/artifacts/ebe3d4da8955c443/atif.json`）逐步核，verify 命令退码＋输出原文俱在（step20-45），证力不弱于手留文件。
- **O-1（观察项）**：`fdSet/fdIsSet` 直写 `FdSet.Bits[fd/64]`——fd≥1024 时越界 panic。席进程常态 fd 数远低该阈（socket/pipe/inotify 个位量级），且 stdlib Select 为票面钦定径、x/sys 被禁引——残险记档不归修。
- **O-2（观察项）**：起机 `pumpSentinelFile()`（`sentinelwatch.go:58`「起机排干存量」）系票面外施工补充——幂等零成本、对 AddWatch 前后窗保守补捕，摘要「下一步」已申报，语义与「事件批后单泵」等价。
- **瞒报对账**：diff 零 TODO/FIXME/绕过/半成品（grep 实证；`fmt.Fprintln` 四处皆票面钦定降级留痕非残渣）。摘要「下一步」逐项对账：设计 §6 残险照做未扩修（真——diff 未碰 enqueue/sendPump 并存面/异质 fs/事件风暴/exit=-1 另案）；不代 0044/0038-p1-p2 活（真）；stdout 块缓冲壳本体、spawn_blocked 连发本体未动（diff 实证）；起机存量泵申报与 diff 实证一致。**瞒报零。**
- **过程坑（已记档）**：施工变异验证期 `git checkout` 误回退本票改动（atif step38-41 实录：pairMu 摘除回退连带 seat.go 五处全丢）——发现后重施改动并复跑全绿，坑型补记入 `docs/经验库/2026-11-01-变异验证回退-git-checkout吃未提交改动.md`（交付 note 携带）。终态 diff 即复绿后码，不受影响。

## 独立复算

本席在 `/opt/meta-wt/0045-sentinel-pump-watch`@c39414a 复跑（go1.27.x）：`go build ./... && go vet ./...` rc=0；`gofmt -l internal/seat/` 零输出、`gofmt -l internal/` 16 件与 master 基线 diff 零增量；`go test -count=1 ./...`＝285 passed/16 pkg；`go test -race -count=1 ./internal/seat`＝27 passed；`-race -run 'Sentinel|Exec|Pump|Silent|Watch|Race|Degrade|HalfLine'`＝16 passed。施工 atif 逐步对上：step20 vet 报新件 import 未用→step22 修后 BUILD-VET-FMT OK；step31 -race 全绿 19.488s 含五新例；step32 泵行变异→锚2 断言红；step38 pairMu 变异→race detected；step42 终码 FMT+BUILD+VET OK；step44 -race 27 passed；step45 全量 285 passed——逐条一致。

## 可沉淀打法（令#E8 附议）

**可沉淀**。①「协程化 run＋桩网关帧流＋`waitFrame` 限窗探帧」——「存活期内帧入桩」把 watcher 泵与终泵兜底两径结构性区分（探得时 done 未返＝铁证），凡「触发径存疑」类锚皆可复用此证明式；②对抗位变异用「删调用行保编译」出断言级红（非 build failed 假红）——与经验库变异档互证。建议入经验库定型。
