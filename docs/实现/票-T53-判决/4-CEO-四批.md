# 票-T53 判决·四批（4-CEO·2026-11-05 午｜交互态 devin/SWE-2-Max）

支 `t/T53`｜deliver_ref=`7e9bc91`（含前批 25d0446 同并）｜并
master=fd7498c（11 件，+124/−10——生产码 attach/reconcile/gateway/
textproto 含，须换装评估）

## 判决：四批通过（分批验收·票未闭）

- **双写根治**（返修①对症且比医嘱更深）：三批判时归因「测件手调
  绕闸」——四批实证真第二写者是**对账收尸径**（ActiveRuns 扫到
  pid 死但无 env 判，机械落 seat_dead）。修法：收尸径归一
  `seatLogSilent` env 判＋`epochHasKind` 期窗去重——attach/Wait 径
  先落账则收尸径零追加，两径同窗恰一条。
- **spawn_env_fail 判族落地**：环境即死→retry_same cap=3（config
  与码内缺省双落）——壳无罪不烧候选，shells_exhausted 烧穿链断；
  `failureClasses` 入账齐。
- **机检**：build/vet 净；锚测隔离 15/15 绿；开发面 `-p 2` 全 17 包
  342 测两连绿（R1/R2）。

## 续批遗留

- **TestDesignLock 负载偶红**（D-93 名单族）：spawn 链排队/即死余波
  锁放链受池鱼——席自列名下批靶（spawn 径负载死席根治续）。
- **锚测残余窄窗**（观察项）：锚测仍直调 onRunDead——若 Wait 径
  exitCode 未落即手调仍可能记 seat_dead。双写窗已从生产侧消（收尸
  径归一），残余仅测件内部竞；再红即改走自然径等事件。
- gofmt 三件顺手欠（dispatch/gateway/t_t53b_test.go——非硬闸）。
