# 票 29：check 步容器化＋部署闸零 sudoers 化（外部手术刀票·基建前置）

> 票面（外部施工输入）；本票由 CEO 席外部手术刀径直接施工并入——
> 部署闸自身无 sudoers 不可过闸，本票即解此结的前置件。

### 1. 目标

部署闸 exec 五步脱 sudoers：同用户步短路直跑＋check 步（唯一不可信码位）
改 docker 容器笼。落地后闸对 meta 配方全程零 sudo——本票自身成为
闸「核实即完成」首验票。

### 2. 为什么

- 现状：`exec.go` 恒 `sudo -n -u <run_as>`——run_as=mluser 的四步也被卷入；
  无 sudoers 行全步红，16 张待部署票空转。
- 业主裁：不落常驻 sudoers 授权（NOPASSWD:env＝任意代跑面），容器笼代之。
- 归档对照：meta-archive door5 把安检外包 GitHub CI 故零隔离需求——
  本版自托管无 CI 层，本地安检责任在己，容器是正确边界。
- bwrap 弃用实证：Ubuntu 24.04 `apparmor_restrict_unprivileged_userns=1`
  拦死非特权 userns（uid map EPERM/RTM_NEWADDR EPERM）。
- 现成条件全齐：mluser∈docker 组、dockerd 29.3.0、`golang:1.27-alpine`
  已本地拉取（贴 go.mod 1.27.2；modernc sqlite 纯 Go 无 CGO 可跑 alpine）。

### 3. 范围内

- `internal/gateway/exec.go` 三分支：container_image 置→docker run；
  run_as＝网关进程用户→直跑；余→遗留 sudo 径（不删，他项目可用）。
- `proto/meta/v1/meta.proto`：`SandboxSpec` 加 `container_image`（field 4）＋pb 重生成。
- `ensureDeployClone`：clone 归网关用户直跑（clone＝自家仓可信操作，不再 sudo）。
- docker argv：`--rm --name meta-chk-<tid>-<纳秒>` `--network meta-sandbox`
  `--user 1000:1000` clone→/work rw；`~/go/pkg/mod`→/gomod ro；
  `GOMODCACHE=/gomod GOCACHE=/work/.gocache`；META_* 五变量 `-e` 透传；
  超时/退场 `docker rm -f` 兜底扫孤儿。
- `config/gateway.textproto`：meta 配方 sandbox 置 `container_image:"golang:1.27-alpine"`。
- 一次性环境件（非码）：`docker network create --internal meta-sandbox`
  （lo 通无外通——board 测试绑 127.0.0.1 需 lo；mluser 可建零 sudo）。
- 测试四锚入 `internal/gateway/`。

### 4. 范围外（禁线）

- 不动 sudo 二进制路径/D1 禁网关自身用户语义（注释更新：笼从用户切换容器）。
- 不动 seat/caster/board/事件面；不动既有配方其余字段语义。
- 不引新 Go 依赖；不建 docker 镜像（用现成 golang:1.27-alpine）。
- run_as 字段保留为审计身份主张档，不改语义。

### 5. 设计引用

- 决策 79（外派刀九连并＋sudoers 实证）；台账 D-48 配方写径新债同包核。
- exec.go:119 行沙箱注（D1 诚实妥协）；gateway.textproto project.meta.sandbox。

### 6. 验收锚

1. 同用户步 argv 无 sudo（run_as=mluser 步断言前缀为 env 非 sudo）。
2. container_image 步 argv 含 docker run/镜像名/meta-sandbox/--user/-e 注入。
3. 超时径 docker rm -f 被调（mock exec 断言孤儿扫）。
4. 对抗锚：删短路或删笼分支→对应锚红。

### 7. 验证命令

`go build ./... && go vet ./... && go test -count=1 ./... && bash scripts/selfcheck.sh`

### 8. 交付形制

- 支 `t/29-check-docker`；deliver_ref＝支头 commit；META_DELIVER 哨兵径由
  CEO 外部径代发（本票不走席位链）；审计判决档外补后入 待部署，
  由部署闸「核实即完成」翻 完成——顺带首验。
