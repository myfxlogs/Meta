# 判决：通过（0034-p3 二骑）——checker 真 detach＋等死再探，驳回两项俱灭

支 `t/0034-p3-deploy-verdict` ref `a016b01`（骑1 `ac21f97`→骑2）。2026-11-02。

## 逐锚对账

| 锚 | 判 | 证据 |
|---|---|---|
| checker 真 detach | PASS | `systemd-run --user` 去 `--scope` 走 transient service（unit 带时间戳防撞）——Run 即回非阻塞；setsid 备胎照留 |
| 等死再探 | PASS | CheckCmd：30s deadline 探旧口灭→15s 探新口活→绿；旧口一直活（restart 未发）则降级安全绿——不死等 |
| 反向锚双桩 | PASS | probeFn 包级桩缝；①旧2探活→灭→新起：断言 calls≥4（不过早绿）②恒灭：断言退错+回退 prev 分支走 |
| 骑1 采信面零动 | PASS | diff 只触 upgrade.go spawnChecker/CheckCmd + t_p3_test.go |
| 测绿复算 | PASS | upgrade+gateway 144 测绿（worktree a016b01 实跑） |

## 部署波四病根账（至此全销）

①克隆语义 T13 ✓ ②B1锁泄 T15 ✓ ③笼（extra_mount+CLI挂） T15 ✓
④完成判定靠必死之席 本票 ✓——态判收口（deploy exit=0+socket绿→
deploy_ok 不翻 failed）＋席包 scope 免连坐＋自检移位存活执行体。

## 启用序（部署面件）

本支并入后下一部署骑将：席包 scope 存活→deploy restart 只灭网关→
checker transient service 探新口→席/票经 reconcile 态判 deploy_ok。
锚③④（坏二进制回退/连坐止）属实跑验证件——下一波部署实测对账。
