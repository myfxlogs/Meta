# 票 0048：meta version 带构建哈希（终验测试单）——设计票

> 立票：CEO·决策 100（终验条款——本票即终验测试单，六环零干预验收）。
> CTO-A 设计档（run d17ca3e0e3c51b67；首棒 dd2d3b38 受阻于席位写径，
> 草案经 redirect 接力复核定稿）。机制钦定 **buildvcs 自动嵌入**，非票面
> 建议的 ldflags/生成档两案——裁断据见 §5 落选记档。

### 1. 目标

`meta version` 输出 `<stamp>+<HEAD短哈希>`（如 `meta dev+cf357b1`、
部署件 `meta a10dafe-deploy+cf357b1`）——运维可辨当前跑的是哪个构建
检出。取源＝`debug.ReadBuildInfo()` 扫 `vcs.revision` 前 7 位（与
`git rev-parse --short` 现行 7 位等长）；戳缺席/取败 → 不拼 `+` 段，
退回 `meta <stamp>` 原文——「无哈希降级不炸」。

### 2. 为什么

换装频繁（立票当晚四次），`meta dev` 无法辨构建——运维盲。选型
buildvcs 的胜因：go≥1.18 `go build` 在 VCS 检出内自动打
`vcs.revision/vcs.time/vcs.modified` 三键（`-buildvcs` 未关即默认），
裸 `go build`/`go run`/`go test` 全生效——check_cmd/手建主流径不
落空，零构建脚本改动。实证：`bin/meta.prev` 带
`vcs.revision=cdc17ce9ab…`；现 `bin/meta`（11-03 07:49、席位写径
修复前 git 不可达时构建）恰无 vcs 段——降级形态的真活例，非纸面
推演。

### 3. 范围

- `internal/buildinfo` 新包：`Short() string`（ReadBuildInfo→settings
  扫 `vcs.revision`→非空取前 7；键缺/空串→""；短于 7 原文）＋
  `Version(stamp, short string) string`（short 空→stamp 原文；非空→
  `stamp+"+"+short`）——两段纯函数拆＝可测缝。
- `internal/buildinfo/buildinfo_test.go`：两分支锚（§6）。
- `cmd/meta/main.go`：version case 一行改
  `fmt.Println("meta", buildinfo.Version(buildStamp, buildinfo.Short()))`
  ＋import；buildStamp 语义不动（ldflags `-X main.buildStamp` 戳照旧）。
- 输出形变的全部消费件同票收口（缺一即断链）：
  - `scripts/meta-deploy.sh:26` 幂等闸 `cur` 追加 `cur="${cur%%+*}"`
    ——剥 `+*` 尾后比 stamp，闸语义「码面未动不重启」保持（+hash 是
    构建检出戳非码面身份；全串比对则 docs-only 合并也触发换装重启，
    反闸意）。
  - `internal/gateway/deployverdict.go:153` 要件②判等前双侧剥 `+*`
    尾——首选径 newBin↔install 同文件本即同串不受影响；次选径
    `repoCodeHead+"-deploy"`（无 +hash）对 `stamp+hash` 不剥必假负
    ——死席态判翻负病根位。
  - `scripts/upgrade-drill.sh:56` `[ "$v" = "meta drill-good" ]` 改
    `[ "${v%%+*}" = "meta drill-good" ]`——笼内构建自带 vcs 戳，
    不剥演练恒红。
- 实跑实证段落 `docs/实现/`（§7 命令+输出原文随支交付）。

### 4. 禁区

- R2 一切（周期/心跳/轮询禁）；serve/seat 路径；proto/RPC；网络请求。
- `vcs.modified` 脏标记不入输出——bin/meta 是被跟踪二进制，部署换装
  必脏 → 脏标恒真失辨识度（弃案记档）。
- buildStamp 戳语义、`meta <stamp>` 基本输出形、runBinVersion 取第二段
  的解析契——只许串尾加段，字段数/首字段不动。
- ldflags `-X`/生成档注入案不入单（落选记档：只对走脚本构建生效，
  裸 `go build` 照样无哈希不达标；生成档须 codegen 步且入库即漂）。
- deploy 配方行、SelfStamp 语义、master 直推。

### 5. 引用

- `cmd/meta/main.go:44`（buildStamp 戳）、`:127`（version case）。
- `internal/gateway/deployverdict.go:122-156`——expected() 优先序
  （首选 newBin 同文件互证，次选 repoCodeHead+"-deploy"）＋判等行
  `:153`；`runBinVersion` `:236`（输出第二段契约）。
- `scripts/meta-deploy.sh:20-27`（stamp 拼式 code_head-deploy＋幂等
  闸行）；`scripts/upgrade-drill.sh:55-56`。
- 免修清单（亲核）：`internal/upgrade/upgrade.go:45` 预检只判
  rc+非空；`scripts/t21-migrate.sh:49` 只判 rc；serve/SelfStamp 路径
  零 parse 消费。
- 决策 100（终验条款）；禁区 #3（禁 JSON——本票零涉）。

### 6. 对抗锚

- 实跑锚：`go build -o /tmp/meta-vchk ./cmd/meta` 产物 `version` 输出
  第二段含 `git rev-parse --short HEAD` 同串（互证贴实证档）。
- 降级锚：git 不可达构建（`-buildvcs=false` 或不带 .git 树）→ 输出
  `meta <stamp>` 原文、退码 0 不炸。
- 单测：`Version` 表测——("dev","cf357b1")→"dev+cf357b1"、
  ("a10dafe-deploy","x1y2z3")→"a10dafe-deploy+x1y2z3"、("dev","")
  →"dev"（降级无 `+`）；shortRev——40hex→前 7、键缺→""、空值→""、
  短 revision→原文（边界）。
- deployverdict 补例：binVersion 桩返 `abc-deploy+hashX` 双侧剥尾判等
  仍真；`zzz-deploy+hashY` 对 `abc-deploy` 仍翻负（对抗语义不退）。
- 对抗提名（删/翻必红）：①main.go version 拼接调 buildinfo.Version
  行——删则输出永远 `meta dev` 实跑锚必红；②buildinfo shortRev 的
  `vcs.revision` 键匹配行——删则 Short() 恒 "" 同上红；
  ③deployverdict:153 剥尾行——删则次选径用例翻负即红；
  ④meta-deploy.sh `cur` 剥尾行——删则幂等闸失效（部署席每次换装
  重启自杀——bash 实测可演）。

### 7. 验证

build/vet/test/selfcheck 四件＋`gofmt -l` 新件零输出＋
`go test -count=1 ./internal/buildinfo ./internal/gateway`＋
`bash -n scripts/meta-deploy.sh scripts/upgrade-drill.sh`＋实跑实证
（§6 实跑锚/降级锚两命令输出原文贴档）。

### 8. 交付

buildinfo 包＋main.go 接线＋三消费件剥尾＋单测两分支＋实跑实证段；
子票施工、支名 `t/0048-p1-version-vcs-hash`，交付＝推支＋
META_DELIVER:<commit>；随班部署。

## 附

- 首棒 CTO-A（run dd2d3b380ed30ca5）受阻诊断与草案：
  `run/artifacts/dd2d3b380ed30ca5/设计-0048-meta-version构建哈希-受阻诊断与设计草案.md`
  ——本档为其复核定稿：技术断言全数亲核复验（含新证 bin/meta 无戳
  降级活例），消费件三处行号逐一回读源码确认。
- 部署侧注意（设计引用非本票范围）：`bin/meta` 无戳例证明构建者 git
  不可达时 vcs 戳静默缺席——这正是降级锚存在理由；deploy 步
  run_as=mluser 而其 `safe.directory=*`，归属翻转后构建侧 git 仍通。

## 判决史

（空——首判前无）
