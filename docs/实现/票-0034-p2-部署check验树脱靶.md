# 票 0034-p2：部署 check 验树脱靶——笼挂首克隆交付 ref 旧树而非 integrate 结果树＋克隆永不刷新（0034 批部署笼内复红、p1/T12 修复空转）

> 巡检席自立案（信号 `phase:deploy_failed`×1：事件 5304，链 5179 deploy_start→5259 integrate 绿→5260 check 红→5303 rollback 绿）；
> 证据链全机器可复跑，笼内对照命令见 §7。

### 1. 目标

修「部署四步验树不一致」两处，解堵 0034 部署终验（锚6，判决档 7a44e2c）：

- **exec.go 验树脱靶（病根 A）**：check/probe 笼挂的是 `run/deploy/<票>` 克隆
  （exec.go:196 `ensureDeployClone`→:240 `-v workDir:/work`），而 integrate 在
  `/opt/meta` 主树跑（textproto integrate_cmd `git -C /opt/meta`）。ref 为已并
  支尖（HEAD 祖先）时 integrate 走 ancestor 短路 no-op 绿＝实际验的是 master，
  check 验的仍是旧支尖树——**两步验的根本不是同一棵树**。修法方向（施工
  定夺）：check/probe 验树改＝本次 integrate 结果（并后 HEAD）；至少
  ref∈HEAD 祖先时验 HEAD。
- **ensureDeployClone 缓存永不刷新（病根 B，exec.go:299-301 目录在即返回）**：
  克隆仅首部署时 `clone＋checkout ref` 一次，ref/支前进后钉死旧树。已显形：
  `run/deploy/0034-p1` HEAD=c21be1a ≠ 支尖 035faf1（0040 rebase 后克隆未随）。
  修法方向：每部署 fetch＋checkout 现 ref，或克隆路径键含 ref sha。

### 2. 为什么

- 本次失败全链（事件 5179→5259→5260→5303→5304，`deploy_failed` note
  `META_DEPLOY:failed devin exit=0`）：
  - integrate log 首行 `e10969255c452517362e0693f3a36ac09b0aad35`＝
    META_EXEC_REF（t/0034-patrol-filter-spawnfix 尖）；
    `merge-base --is-ancestor e109692 d26b175`=true（b66f53f 已并）→
    integrate_cmd ancestor 短路 exit 0，master 停 d26b175。
  - check 笼挂 `run/deploy/0034`（实测 HEAD=e109692）：笼内
    `TestDeploy_FourGreen`/`TestDeploy_RedRollback` deny -1 ×2（note=
    legacy-sudo 径已退役）＋`TestExec_ContainerArgv` panic@deploy_test.go:648
    ——与票 0034-p1 §1 老签名**逐字一致**；该树 645 行 `me, _ :=
    user.Current()` 弃 err＋648 行裸 `me.Uid` 与 panic 栈逐行吻合。
  - **两道修复都在 master 且完好在档，红因非回归**：21226a2∈d26b175
    （exec.go:391/400 数字 uid 直比现档）；T12 跳变守卫（d26b175 合并
    6fb84e4）现档。check 根本没验到含修复的树——p1 修复与 T12 守卫对此
    部署双空转。
- 后果：0034 终验结构性堵死；deploy_failed 补发循环已起（5307/5318/5319
  spawn_needed），现被 0038 B1 锁 spawn_blocked 掩护，锁一释即同签名复红
  烧 CTO-B 壳（0034 三连败前科，0040 §2）。同类在飞：0036/0037-p2/0038-p1/
  0040 克隆俱在 `run/deploy/`，机制修前一律同雷。

### 3. 范围内

- `internal/gateway/exec.go`（ensureDeployClone、步进树选取/笼挂）＋对应
  gateway 测试件
- `config/gateway.textproto` 配方若需随改
- `run/deploy/` 旧克隆随修复一次性重建（须由机制径触发，非手删冒充——
  见 §4）

### 4. 范围外（禁线）

- **禁以「手删 run/deploy/0034 克隆」冒充修复**——一次性解堵≠机制修，且对
  已并 ref 重克隆仍验旧支尖树（病根 A 未动），下次部署同签名复红
- 禁复活 legacy-sudo（`TestDeploy_LegacyDenied` 保绿）；禁动 0038 锁语义/
  B1 串行闸；禁手并 master；照例 worktree 支交付，禁占主仓直改
- T12 跳变守卫语义不动（4 SKIP 口径以 T12 判决为准）

### 5. 设计引用

- 票 0034-p1（笼身份解析失效——本票红签名原主，修复在档勿重修）
- 票 0040（批rebase——integrate 分叉类前科；rebase 补丁法对本类无效：
  已并票 ref 恒落后 master，唯有验并后树可解）
- 票 T12 判决 1-CEO（4 SKIP 亲验——守卫生效前提＝check 真验 master 系树）
- 设计-0034-p1-p1-部署通道批rebase.md、票 0029（check 笼化初心＝跑票
  代码；ref 已并时票代码≡master 真后代，初心与验并后树不冲突）

### 6. 验收锚（全部机器可判）

1. **同景复部署绿**：`run/deploy/0034` 旧克隆（e109692）在场不手删，重部署
   0034 → integrate/check 双绿（笼内 docker 系测试按 T12 语义恰 SKIP）→
   probe 绿 → deploy_done，票 0034 出待部署
2. **验树一致性**：部署期 exec 事件/日志可证 check 步笼挂树 HEAD＝本次
   integrate 后 master HEAD（旧克隆不复活）
3. **克隆随动**：支尖前进后重部署（现场例：0034-p1 克隆 c21be1a≠035faf1）
   → 克隆 HEAD 追到本次 ref/结果树
4. **对抗锚**：回删「祖先时验 HEAD」选取逻辑 → 用 e109692 类旧克隆复跑
   §7 命令① 必红（红＝锚在）
5. `TestDeploy_LegacyDenied` 保绿；exec.go:299-301 缓存早退删改须带
   「克隆不随支前进→红」测试锁

### 7. 验证命令

（与 5260 失败 run 同构 argv，仅换挂载树——树差即病根双跑对照）

① 旧克隆树（复红）：
```
docker run --rm --user 1000:1000 \
  -v /opt/meta/run/deploy/0034:/work -w /work \
  -v <gomodcache>:/gomod:ro -e GOMODCACHE=/gomod -e GOCACHE=/tmp/gc \
  -e HOME=/work -e GOFLAGS=-buildvcs=false golang:1.27 bash -lc \
  'export PATH=/usr/local/go/bin:$PATH && go test -count=1 ./internal/gateway/ -run "TestDeploy_FourGreen|TestDeploy_RedRollback|TestExec_ContainerArgv"'
```
② master 树（对照应绿/恰 SKIP）：同上，仅 `-v /opt/meta:/work`
（GOCACHE 指 /tmp 免污主树）

### 8. 交付形制

照例：worktree 支 `t/0034-p2-*` 交付，本票文件＋判决档随支；全绿＋§6 锚
全过方提判。

## 附

- 证据文件：`run/exec/0034-integrate-1793607112093333896.log`（ref sha）、
  `run/exec/0034-check-1793607118859321484.log`（红全文）、
  `run/exec/0034-rollback-1793607209754557672.log`（"本部署无破坏可回"）；
  `run/deploy/0034` HEAD 实测 e109692、`run/deploy/0034-p1` HEAD c21be1a
- 观察项（不入本票范围）：在跑网关 rollback 文案（"merge已并历史不回退"）
  与现档 textproto rollback_cmd（MERGE_HEAD 分径）不一致——疑 0041 在飞
  支换装时序或配方漂移，留 0041 席核对

## 判决史

（空）
