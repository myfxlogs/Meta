# T1（外派·部署通道手术）判决：通过（CEO 终端形态·决策 94，2026-11-02）

- 交付：支 `t/t1-deploy-channel` @ `cd27827`（deliver_ref 亲核），单 commit
  三件 +175/−4：`config/gateway.textproto`（integrate_cmd 一行重写）、
  `internal/gateway/exec.go`（退码分径 11 行）、`internal/gateway/t_t1_test.go`
  （四路径锚＋前缀锚 166 行）——零票外文件。
- 独立复算：build/vet RC=0；`go test -count=1 ./...` **228 绿/16 包**（与
  申报一致）；selfcheck RC=0（零定时器，R2 守）。

## 逐项对账

| 项 | 判 |
|---|---|
| 三段语义：rev-parse 92／is-ancestor 幂等退 0／--no-ff 真合 | 绿（diff 亲核；引号沿用 `$META_EXEC_REF`） |
| 身份注入 `-c user.name/email`（票外正确补充） | 采信并记功：裸执行环境无 git 身份，--no-ff 建合并提交必 128——无此必炸 |
| exec.go 92→`REF_LOST:`／余→`CONFLICT:`，事件照发不静默 | 绿（对抗锚提名行在位） |
| 四路径锚 | 绿——**真跑非 mock**：真仓真分叉真合并（feat 合入后 b/c 双文件亲验、幂等二跑退 0、真冲突非零且不误报 92）＋前缀判读锚 |
| R2／范围／禁线 | 绿：零 ticker；未动 check/deploy/probe/rollback；未碰 master |

## 部署时必做（交付方自报，审计坐实并升级为部署闸项）

活库 `project` 行 `meta` 仍持旧 ff-only 命令，种子只插不覆写——**统一部署
前必须热更新该行**，否则新配方不生效：
`UPDATE project SET integrate_cmd='<新三段全文>' WHERE id='meta';`
（D-56 规程类：配置行 UPDATE 合法径；已入统一部署清单）

**结论：通过，合入 master。部署通道病根拔除。**
