# T12（外派·笼内 docker 测试跳变守卫）判决：通过（CEO 终端形态·决策 94，2026-11-02）

- 交付：支 `t/t12-docker-skip-guard` @ `5a4d15a`（worktree /tmp/wt-t12 合规）。
  单件 +29（deploy_test.go：requireDocker 守卫＋四测接线＋守卫锚）。
- 独立复算：build RC=0、全量 `go test ./...` 过、selfcheck RC=0；
  **PATH 裁剪模拟亲验：恰 4 SKIP**（FourGreen/RedRollback/ContainerArgv/
  ContainerOrphanSweep），与申报一致；零删测试、零动被测码（diff 亲核）。

## 判

| 项 | 判 |
|---|---|
| LookPath 败→Skip（理由带错文本）；主机有 docker 全量照跑 | 绿 |
| 守卫锚三分支（有/无/查找错） | 绿 |
| 安全红线：不给笼挂 docker socket | 绿——注记钦定「挂 socket＝红线不开」入码注释 |

## 观察项（长期形态，不开任务）

笼内跳过＝朴素分层。正式形态＝安全单测（笼，部署闸职责）与 docker 集成
测（主机层，部署后验/审计复算跑）分tier——可用 `-short` 模式或 build
tags 落地，随 T8（容器笼）设计一并议。docker vs podman-rootless/bwrap
选型对比亦入 T8 设计较验节。

**结论：通过。测试码随 master 即生效（无需换程序）；解冻部署队列。**
