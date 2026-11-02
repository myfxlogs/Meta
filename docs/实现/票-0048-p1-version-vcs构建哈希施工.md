# 票 0048-p1：meta version 带构建哈希——施工支

> 需求票 0048 拆出（CTO-A run d17ca3e0e3c51b67）。设计档
> `docs/实现/票-0048-meta-version构建哈希.md`（决策 100 终验条款）
> 为唯一设计源——机制已钦定 buildvcs 自动嵌入，照档落码不改型。

### 1. 目标

落 `internal/buildinfo` 包（Short/Version 两纯函数）＋
`cmd/meta/main.go` version 拼接行＋三消费件 `+*` 剥尾＋单测两分支＋
实跑实证段——`meta version` 输出 `<stamp>+<HEAD短哈希>`，git 不可达
构建降级 `meta <stamp>` 不炸。

### 2. 为什么

换装频繁运维盲（立票当晚四次换装，`meta dev` 不可辨构建）。本票为
决策 100 终验测试单的施工环——票小锚硬，六环零干预验收的一部分。
机制选型与落选记档全在设计档 §2/§4，照做不重裁。

### 3. 范围内

- worktree：`git -C /opt/meta worktree add ~/meta-worktrees/t0048-p1 -b
  t/0048-p1-version-vcs-hash`（禁主检写码——禁区 #8）；一切改动支内。
- `internal/buildinfo/buildinfo.go`：
  - `Short() string`——`debug.ReadBuildInfo()`→`bi.Settings` 扫
    `vcs.revision`→非空取前 7（与 `git rev-parse --short` 现行 7 位
    等长；短于 7 原文）；ReadBuildInfo 败/键缺/空串→""。
  - `Version(stamp, short string) string`——short 空→stamp 原文；
    非空→`stamp+"+"+short`。两段纯函数；取键逻辑拆纯函数缝
    （如 `shortRev(settings)`）供表测。
- `internal/buildinfo/buildinfo_test.go`：§6 锚③全例＋`Short()` 冒烟
  （笼内返 "" 或 `^[0-9a-f]{7}` 容两态）。
- `cmd/meta/main.go`：version case 一行改
  `fmt.Println("meta", buildinfo.Version(buildStamp, buildinfo.Short()))`
  ＋import；buildStamp/ldflags 戳语义不动。
- `scripts/meta-deploy.sh:26` 后追加 `cur="${cur%%+*}"`。
- `internal/gateway/deployverdict.go:153` 判等前双侧剥 `+*` 尾
  （行内或 stampKey 辅助——锚管行为不管名）＋桩例两条（§6 锚④）。
- `scripts/upgrade-drill.sh:56` 改 `[ "${v%%+*}" = "meta drill-good" ]`。
- 实证段：`docs/实现/票-0048-p1-实证.md`（或并入本票档实证段）记
  §7 实跑两命令+输出原文。

### 4. 范围外（禁线）

- R2 一切；serve/seat 路径；proto/RPC；网络请求；新增第三方依赖。
- `vcs.modified` 脏标记入输出（弃案）；vcs.time 同理不入。
- buildStamp 语义、`meta <stamp>` 输出基本形、runBinVersion 第二段契。
- ldflags `-X`/生成档注入案（落选记档不重议）；deploy 配方行；
  master 直推；部署动作（部署归 CTO-B 环）。
- reconcile/派发/审计面任何逻辑——本票纯 CLI 输出面。

### 5. 设计引用

- `docs/实现/票-0048-meta-version构建哈希.md`——唯一设计源（§3 范围、
  §4 禁区、§5 行号锚、§6 对抗提名全照其文）。
- `internal/gateway/deployverdict.go:122-156`（expected() 两径＋判等
  行）；`:236`（runBinVersion 第二段契）。
- `scripts/meta-deploy.sh:20-27`；`scripts/upgrade-drill.sh:55-56`。
- 免修亲核：`internal/upgrade/upgrade.go:45`（rc+非空判）；
  `scripts/t21-migrate.sh:49`（rc 判）。

### 6. 验收锚（全部机器可判）

1. 四件绿：build/vet/test/selfcheck 全过＋`gofmt -l` 新件零输出。
2. 实跑主锚：支内 `go build -o /tmp/meta-vchk ./cmd/meta` 后
   `/tmp/meta-vchk version` 输出第二段含
   `git -C /opt/meta rev-parse --short HEAD` 同串（互证贴实证档）。
3. 降级锚：`-buildvcs=false` 构建（或不带 .git 树构建）产物
   `version` 输出 `meta <stamp>` 原文、退码 0 不炸（实证档贴出）。
4. 单测两分支在且绿：`Version` 表测三例（拼接/复合戳拼接/降级无
   `+`）；shortRev 四例（40hex→前7/键缺/空/短值原文）；deployverdict
   桩例——`abc-deploy+hashX` 双侧剥尾判等仍真、`zzz-deploy+hashY`
   对 `abc-deploy` 翻负。
5. 对抗提名四处（设计档 §6 原文）：main.go 拼接行 / shortRev 键匹配
   行 / deployverdict 剥尾行 / meta-deploy.sh `cur` 剥尾行——任一删
   或翻对应锚必红（审计席按名行核）。
6. `grep '%%+\*' scripts/meta-deploy.sh scripts/upgrade-drill.sh` 两命中
   在。

### 7. 验证命令

`go build ./...`＋`go vet ./...`＋`go test -count=1 ./internal/buildinfo
./internal/gateway`＋`bash scripts/selfcheck.sh`（或配方钦定四件）＋
`bash -n scripts/meta-deploy.sh scripts/upgrade-drill.sh`＋§6 锚②③
实跑两件（输出原文贴实证档）。

### 8. 交付形制

支 `t/0048-p1-version-vcs-hash`（worktree 内提交、推支不并 master）＋
`META_DELIVER:<commit>` 哨兵双发（stdout 末行＋sentinel.log 原文行）；
交付摘要四节（新增/修改面、根因、验证引、下一步=全量遗留申报）＋
逐锚对账；随班部署。

## 附

- 设计档附段记首棒受阻链与本档复核史——返修续作同支（支已存在＝
  续用，见施工轨）。
- `bin/meta`（11-03 07:49 无戳件）即降级形态活例——遇「version 无
  哈希」先查构建时 git 可达性，非码病。

## 判决史

（空——首判前无）
