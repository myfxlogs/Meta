# 票 0040：部署通道批 rebase——0034-p1/0036/0037-p2 三支交付支至 master 顶

> 设计票 0034-p1-p1 拆出（CTO-A）；设计档
> `docs/实现/设计-0034-p1-p1-部署通道批rebase.md` 含全实证链。

### 1. 目标

三支部署受阻交付支各 rebase 至执行时本地 master 顶并
force-with-lease 推 origin，使各票下次部署 integrate 步 ff 可合：
`t/0034-p1-cage-identity`、`t/0036-card-phase-elapsed`、
`t/0037-p2-precedent-inject`。

### 2. 为什么

- integrate 配方 ff-only（`config/gateway.textproto:36`），master 前进
  致三支分叉：0034-p1 integrate 三连红（事件 2898-2899 真红、
  2963-2964 复红、席死第三回）；0036/0037-p2 integrate 档同文红
  （`run/exec/0036-integrate-*.log`、`0037-p2-integrate-*.log`）。
- 不解则部署通道对全部待部署票串行堵，deploy_failed 自动补发持续烧
  CTO 壳候选（0034 三连败死锁前科）。
- master 仍会移动：rebase 目标＝执行时 `master` 顶，不写死 sha。

### 3. 范围内

- `t/0034-p1-cage-identity`（worktree `~/meta-worktrees/t0034-p1`）：
  rebase 至 master 顶，两提交（20bc6f5 修复＋99e11e6 判决档）随迁；
  台账冲突解＝保 master 侧 D-67/D-68 行，交付支 deploy_failed 债改号
  **D-69** 落账（行内容不改）；判决档 `票-0034-p1-判决/1-devin.md`
  内两处「D-67」指代同步改 D-69。
- `t/0036-card-phase-elapsed`（`~/meta-worktrees/t0036-card-elapsed`）：
  同法 rebase（预判零冲突；有则按既成内容最小解，不改语义）。
- `t/0037-p2-precedent-inject`（`~/meta-worktrees/t0037-p2-precedent`）：
  同法 rebase。
- 每支收尾：`go build ./... && go vet ./... && go test -count=1 ./...`
  于该支顶绿；`git push --force-with-lease origin <支>`；核
  `git merge-base --is-ancestor master <支>` exit 0。

### 4. 范围外（禁线）

- 不改任何票实现语义——只 rebase 移栽与台账改号/判决档指代同步，
  不增删码。
- 不改 integrate 配方、不动 master、不部署、不重派任何票。
- 不碰 0034/35/0038-p1/0037-p1 等其余票支与本票三支外分支。
- 禁复活 legacy-sudo（`TestDeploy_LegacyDenied` 保绿）。

### 5. 设计引用

- 设计档 `docs/实现/设计-0034-p1-p1-部署通道批rebase.md`（实证链/
  撞号推演/竞速诚实边界/票35另案记档）。
- `config/gateway.textproto:36` integrate_cmd；`config/caster/cto-deploy.md`。
- 0034-p1 events 2871-3167；exec 档 `run/exec/0034-p1-integrate-*.log`
  ×2、`0036-integrate-*.log`、`0037-p2-integrate-*.log`。
- patrol 立案档 `run/artifacts/c7a632a7b513ef4b/spec-0034-p1-p1.md`。

### 6. 验收锚（全部机器可判）

1. `git merge-base --is-ancestor master t/0034-p1-cage-identity &&
   git merge-base --is-ancestor master t/0036-card-phase-elapsed &&
   git merge-base --is-ancestor master t/0037-p2-precedent-inject`
   三句 exit 全 0（ff 可合）。
2. `git rev-parse origin/<支>` 与各本地支顶一致×3（已推）。
3. `git show t/0034-p1-cage-identity:docs/实现/债务台账.md`：`^| D-67`
   恰一行、`^| D-69` 恰一行且行内含「deploy_failed」义；判决档
   `git show t/0034-p1-cage-identity:docs/实现/票-0034-p1-判决/1-devin.md |
   grep -c D-67`＝0。
4. **对抗锚**：`git show t/0034-p1-cage-identity:internal/gateway/exec.go |
   grep -c "os.Getuid()"`≥2 且该支顶 `go test -count=1 -run
   'TestExec_SameUserNoSudo|TestExec_NumericRunAs' ./internal/gateway/`绿
   ——rebase 若丢 20bc6f5（sameUser 退回名解析）断言必红。
5. 每支 `git rev-list --count master..<支>`≥2（原提交随迁未丢）；
   三支顶各 `go build ./... && go vet ./... && go test -count=1 ./...` 绿。
6. 三支判决档仍在：`票-0034-p1-判决/1-devin.md`、`票-0036-判决/1-devin.md`、
   `票-0037-p2-判决/1-devin.md` 各于对应支 `git show` 可取。

### 7. 验证命令

```bash
cd /opt/meta && git fetch origin
for b in t/0034-p1-cage-identity t/0036-card-phase-elapsed t/0037-p2-precedent-inject; do
  git merge-base --is-ancestor master "$b" && echo "$b ff-ok" || echo "$b STILL-DIVERGED"
  git rev-parse "$b" "origin/$b"
done
git show t/0034-p1-cage-identity:docs/实现/债务台账.md | grep -cE "^\| D-6[789]"
git show t/0034-p1-cage-identity:internal/gateway/exec.go | grep -c "os.Getuid()"
```

### 8. 交付形制

原三支各 force-with-lease 推 origin（不另立支；冲突解入 rebase 内，
每支至多再加一个 `t0040: 台账改号随迁` 类解冲突提交）。哨兵
`META_DELIVER:<三支新顶 sha 逗列>`＋sentinel.log 双发〔E15/D-32〕；
交付摘要五节＋收口两步门〔令#E4/E6/E9〕。

## 附

- 优先级 P1（部署通道级堵）；依赖：无；预算/壳序：缺省。
- 施工中 master 再前进致 ancestor/lease 不满足：fetch 后同法重跑（幂等）。
- 0036/0037-p2 判决档已在支上、审计已过——rebase 不改其内容与判决
  有效性；0034-p1 同理（99e11e6 判决档随迁）。
- 0034-p1 之外票 deploy 重派走网关补发环，本票只保证 ff 可合态。

## 判决史

（空——首判前无）
