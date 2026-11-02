# 判决：驳回（0034-p3 一骑）——态判主修过，detached 自检是「活着的死码」

支 `t/0034-p3-deploy-verdict` ref `ac21f97`。审计日 2026-11-02。

## 逐锚对账

| 锚 | 判 | 证据 |
|---|---|---|
| 态判收口（主修） | PASS | reconcileActor 部署中死席分支：末次 deploy exit=0+socket 探活→PhaseDone+deploy_ok+释双锁；EdgeDeployDone 合法边（phase.go:41/70）；releaseProjectLocks 签名合 |
| spawn 席包 scope 免连坐 | PASS | spawn.go systemd-run --scope 包席 argv——**此处 --scope 阻塞语义正是要的**（席生命周期=scope 存活期），席逃出 meta-gateway cgroup，restart 不灭席；失则直跑兜底 |
| detached 自检移位 | **未达** | 两病：①`spawnChecker` 用 `systemd-run --scope`——**--scope 前台阻塞**，c.Run() 等 checker 退出才算完，根本没 detach（要 detach 得去掉 --scope 走 transient service）；②`CheckCmd` 在 restart **之前**被起，`Probe` 秒连**旧网关**活口→即判绿退出——永远探不到新二进制，回退径仍不可达。setsid 备胎径同病 |

## 驳回项（返修面窄——只动 spawnChecker/CheckCmd）

1. detach 改 transient service：`systemd-run --user --quiet --collect
   --unit=meta-upg-check-<ts> -- <self> upgrade-check ...`（去 --scope），
   Run 即刻返回。
2. CheckCmd 加「等死再探」：先轮询 Probe **失败**（旧网关已灭，超时上限
   防 restart 未发）→再 Probe 成功（新网关起）→绿；全程有界。
3. 反向锚：模拟旧口先活后灭→新口起 的桩，断言 checker 判绿走的是新口；
   桩「旧灭新不起」→断言回退 prev 分支。

## 附记

- 态判主修＝破 0034 二十四连败的真件，机审已过——返修只补 checker，勿动
- upgrade exec 日志现落「自检已移交」行——锚②字面（健康自检绿/回退prev）
  在新机制下落到 checker 的 journal 而非 exec 日志，语义等价可议，返修时
  顺带让 checker 绿/退两行 append 到 deploy exec 日志更稳（--out 参数）
