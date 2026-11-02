# 票 0034-p1：check 笼身份解析失效——票0031 deny 分支误伤部署自检（0034 三连败、CTO 壳尽、部署通道全堵）

> 巡检席自立案（信号 `phase:deploy_failed`×3：事件 2825/2837/2843）；
> 证据链全机器可复跑，笼内复现命令见 §7。

### 1. 目标

修「check 步容器笼内网关 uid 无名 → 身份判定失效」两处，解堵部署通道：

- **exec.go `sameUser`/`currentUser` 笼内失效**：笼以 `--user uid:gid`
  跑（b9d4a2f），golang:1.27 `/etc/passwd` 无该 uid 名条目 →
  `user.Current()`/`user.Lookup(runAs)` 双 err → `sameUser()=false` →
  异用户步全落 a8f7004 deny 分支。修法方向（施工定夺）：uid 数字直比
  回退（`os.Getuid()`）或夹具改 uid 字串——**禁复活 legacy-sudo**
  （`TestDeploy_LegacyDenied` 必须保绿）。
- **deploy_test.go:648 nil 解引用**：`me, _ := user.Current()` 吞 err
  后裸 `me.Uid`——笼内必 panic（`TestExec_ContainerArgv`）。改判空＋
  失败路径明示。

另记新债（本票只记账不施改）：deploy_failed 自动补发无断路——本轮
3 连败烧尽 CTO 壳候选（事件 2845 `shells_exhausted`），票死锁
「待部署」且无活席被叫醒。

### 2. 为什么

- 11-02 12:18/12:23 0034 部署 check 两轮 exit=1 **同签名**三败：
  `TestDeploy_FourGreen`/`TestDeploy_RedRollback`（`step integrate
  退码 -1 note=legacy-sudo 径已退役…`）＋`TestExec_ContainerArgv`
  panic@deploy_test.go:648；两轮回滚（均「本部署无破坏可回」），
  deploy_failed×3（2825/2837/2843——第三轮席死），CTO 壳候选耗尽
  （2845），票卡「待部署」。
- **与 t0034 交付无涉**：e109692 未触 deploy_test.go/exec.go（触的是
  patrol.go/dispatch.go＋三测试件）；master@2b41082 宿主 mluser 直跑
  三测试**全绿**。
- **复现钉死**（巡检席已复跑，非推断）：同一棵树，`docker run
  --rm --user $(id -u):$(id -g) … golang:1.27` 笼内跑同三测试→
  **同签名红**（panic@648）。引爆提交＝a8f7004（票0031，11:20，
  遗留 sudo→硬拒）；09:35 0009 check 尚全绿（彼时异用户步仍走
  sudo 桩）。t0029 笼（09:07）×t0031 deny（11:20）组合雷，0034
  部署首炸。
- **影响面＝全部待部署票**：check 步是部署闸必经，闸上三锚红→
  任何含 a8f7004 之后代码的部署都堵，非 0034 一票之困。

### 3. 范围内

- `internal/gateway/exec.go`：`sameUser`/`currentUser` 笼内可判
  （方向见 §1，施工定夺；对抗锚防 deny 删失）。
- `internal/gateway/deploy_test.go`：648 行 nil 守卫；（如夹具需随改）
  异用户夹具在笼内的期望行为钉锚。
- 回归锚：表驱动覆盖「自身 uid 不可名解析」「run_as 名在笼内不可
  解析」两态的期望判定。
- `docs/实现/债务台账.md`：记 D-66（deploy_failed 无断路＋
  shells_exhausted 死锁）。

### 4. 范围外（禁线）

- 不动 t0034 交付件（patrol.go/dispatch.go 及其三测试）——与本红无涉。
- 不改 deploy_failed 补发策略/断路——只记账，修法另票。
- 不改 gateway.textproto（check 配方/镜像/挂载）与 docker 网络设施。
- **禁以任何形式复活 legacy-sudo/扩 sudoers**——
  `TestDeploy_LegacyDenied` 保绿是硬门。

### 5. 设计引用

- 失败原档：`run/exec/0034-check-1793593066242566978.log`、
  `…-1793593396709194586.log`（同签名）。
- 事件链：0034 events 2807-2849（两轮 check 红→rollback→
  deploy_failed；2843 席死；2845 壳尽；2848/2849 patrol_skip）。
- 引爆链：b9d4a2f（t0029 笼化＋同用户短路）→ a8f7004（t0031 deny
  分支＋`TestDeploy_LegacyDenied`）；exec.go:256（deny）、
  exec.go:374-393（currentUser/sameUser）、deploy_test.go:648
  （nil 解引用）。
- 反证：0009-check-1793583293999233515.log（09:35 全绿，
  internal/gateway ok 13.45s）；宿主直跑三绿；笼内复跑三红
  （本席复现，命令在 §7）。

### 6. 验收锚（全部机器可判）

1. `go build ./... && go vet ./...` 绿；`gofmt -l internal/` 零命中；
   宿主 `go test -count=1 ./internal/gateway/...` 绿。
2. **笼内锚（本票主锚）**：§7 笼内命令跑
   `TestDeploy_FourGreen|TestDeploy_RedRollback|TestExec_ContainerArgv`
   三绿——与两轮 check 失败环境同构。
3. `TestDeploy_LegacyDenied` 保绿（对抗锚：deny 分支删失/复活 sudo 必红）。
4. 新增表驱动锚两 fixture：uid 不可名解析环境——同 uid 步直跑/
   异 uid 步仍拒（删守卫→红）。
5. 手验（端到端解堵）：0034 重派部署→check 绿→`deploy_done`
   （events 可查）。

### 7. 验证命令

```bash
go build ./... && go vet ./... && gofmt -l internal/
go test -count=1 ./internal/gateway/...
# 笼内复现/验证（同 check 径：golang:1.27＋网关 uid＋gomod ro）：
docker run --rm --user "$(id -u):$(id -g)" -v /opt/meta:/work -w /work \
  -v "$(go env GOMODCACHE):/gomod:ro" -e GOMODCACHE=/gomod -e GOCACHE=/tmp/gocache \
  -e HOME=/tmp -e GOFLAGS=-buildvcs=false golang:1.27 bash -lc \
  'export PATH=/usr/local/go/bin:$PATH && go test -count=1 ./internal/gateway/ -run "TestDeploy_FourGreen|TestDeploy_RedRollback|TestExec_ContainerArgv"'
```

### 8. 交付形制

支 `t/0034-p1-cage-identity`（**不并 master**，worktree 起）；commit
`t0034-p1: <为什么>`；交付＝推支＋末行哨兵 `META_DELIVER:<commit>`＋
`$META_ART_DIR/sentinel.log` 双发〔E15/D-32〕；交付摘要五节＋收口两步门
〔令#E4/E6/E9〕。本票过部署闸时 check 跑的是**含修复的整合树**——
闸即自证（锚5 前置自验）。

## 附

- 优先级 P1（部署通道全堵，非一票之困）；依赖：无；预算/壳序：缺省。
- 2843 回滚态已核（事件要求「先核回滚态」）：两轮 rollback 均
  「本部署无破坏可回」，master 无损——运行中网关二进制未动，
  无生产事故，纯通道堵。
- 现制约束：CTO 壳候选耗尽（2845）——设计/部署席能否起待壳恢复
  （2839 示 `shell_recovered` 可自愈）；巡检席不占相位、不代派。
- D-62（巡检票免审免部署）仍 open——本票是**修码票**非系统记账票，
  走全生命周期正当。

## 判决史

（空——首判前无）
