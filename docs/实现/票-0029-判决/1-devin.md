# 票 0029 判决：通过（外部审计·devin 施工自检复算，2026-11-02）

> 形制特例：本票为外部手术刀径——CEO 席（devin）直接施工并入，无席位链。
> 判决由施工实例自检复算给出，业主裁可推翻。

- 交付物：支 `t/29-check-docker` @ `b9d4a2f` 已并 master（merge commit 在账）；四文件码面＋配方行＋本票面
- 复算：`go build ./...` 绿；`go vet ./...` 绿；`go test -count=1 ./...` **199 过**（新增 3 锚含）；`scripts/selfcheck.sh` 绿
- 锚核：①`TestExec_SameUserNoSudo`——同用户步 sudo-argv.log 不存在＋stdout 实证 ②`TestExec_ContainerArgv`——docker run argv 12 要素全在（名/网/user/挂载/e 注入/镜像/bash）＋sudo log 不存在＋退场 rm -f ③`TestExec_ContainerOrphanSweep`——0.5s 超时杀→退-1→`docker rm -f meta-exec-*` 落 log
- 对抗面：删 sameUser 短路→锚1红；删笼分支→锚2红；删 defer rm→锚3红——三锚各自钉住一个删除突变
- 边界复核：遗留 sudo 径未删（异 run_as 无笼步仍走原径）；run_as 字段语义未动；clone 归网关用户（可信仓操作）；bwrap 弃用实证票面留档（Ubuntu 24.04 userns 禁）
- 存量兼容：99 个 gateway 旧测全过——`seatrun`/`owner` 伪用户仍走 sudo 径，旧锚语义不破
- 环境件：`docker network create --internal meta-sandbox` 已建（mluser 零 sudo）；DB 配方行 sandbox_spec 已热更新带笼（种子不覆盖活行的已知面）
- 暗债申报：①同用户判定靠 user.Lookup（NIS/LDAP 域下 Lookup 失败回落 sudo 径——fail 向更严不向更松）②meta-sandbox 网若被删，check 步 fail-closed 不静默降级 ③GOFLAGS=-buildvcs=false 因容器内 git 属主链——VCS 戳信息损失申报
- 结论：范围内全绿，形制与票面对齐（step_container_image 由标量改 map——票面意图为按步置笼，实现细化报业主备案）
