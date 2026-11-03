# 票-0051 判决（meta status 一行状态命令（复测试车））——审计席

支 t/0051-status-cmd｜deliver_ref=38f53fd｜基 5c7a50d（master）

## 判决：通过

## 逐锚对账（只信 diff 与机器记录）

- 版本戳要素：PASS——交付支构建二进制实跑 `meta status --socket
  /opt/meta/run/meta.sock` → 行首 `meta dev+38f53fd`；码面走
  `buildinfo.Version(buildStamp, buildinfo.Short())`，与 `meta version`
  同源（cmd/meta/main.go statusCmd 段首行拼接）＝设计引用「T38 版本戳」径。
- 在途计数要素：PASS——实跑输出 `在途=1 已立=0 设计中=0 已派=0 施工中=0
  待审=0 审计中=1 待部署=0 部署中=0`；语义 `len(NonTerminalTickets)-blocked`
  成立——`NonTerminalTickets`＝phase NOT IN 终态含 blocked
  （internal/db/ticket.go:154-166），减除后八个非终态相位逐项分桶；
  终态不计有测试面（mkPhase SD→完成 不入计）。
- blocked 计数要素：PASS——`blocked=1` 单列（实跑＝0050 在 blocked）；
  码面相位循环内 `PhaseBlocked` 分支 `blocked++; continue` 不入 counts。
- 空板显式零：PASS——`TestStatusCLI` 空板断言逐字段 `=0` 全列（含全部
  八相位键）；审计侧复跑 `go test ./cmd/meta/ -run TestStatusCLI` → PASS；
  施工实证档 scratch 空库实跑全零同载。
- 一行输出：PASS——测试 `oneLine` 断言严格单行（单 `\n` 结尾方格）；
  实跑两行观察均单行。
- 纯读零写：PASS——diff 内零写径调用：AdminGateway.Ping（只读拨验）＋
  NonTerminalTickets/ActiveRuns（纯 SELECT）＋`db.Open` 存量只读用法
  （同 catchrate）；`selfcheck.sh` 零命中。
- 验证命令（四件＋实跑输出实证）：PASS——审计独立复算四件全绿（末节）；
  实跑实证档 `docs/实现/0051-status-实证.md` 载生产 up/死 socket
  svc=down 降级/空板零/库缺失报错不建空库四面。
- 交付形制：PASS——`status` 子命令＋usage 行＋`statusCmd`＋
  `TestStatusCLI`（空板/满板/blocked/svc down/库缺失/单行六面）＋实证档
  ＋坑档；sentinel.log 原文 `META_DELIVER:38f53fd 坑档：docs/经验库/
  2026-11-03-活网关调和器迁边-计数断言要离库测.md` 与 deliver 事件
  note 逐字一致。
- 范围核：PASS——`git diff master...HEAD --stat` 四件：
  cmd/meta/main.go +69/−1、cmd/meta/main_test.go +134、实证档 83 行、
  坑档 28 行；调和器/失败表/通知/proto/schema 零触碰；范围外顺手改零
  （唯一 −1 行为删除一空行）。
- 暗债对账：PASS——diff 全文 grep `TODO|FIXME|XXX|HACK|绕过|半成品|待补`
  零命中；交付摘要「下一步」申报四条与 diff 实状一致（下节逐列），
  无瞒报。

## 偏差/债项

- 「服务相位」落义为 `svc=up|down`（Ping 拨验）——票面未钦定字段名，
  施工已在遗留申报（若另有指属重命名级修正）；语义合理且宕机降级不拒
  合运维令本性，记档不否决。
- 版本戳＝本二进制构建戳，非运行中网关版本——AdminGateway 无该读径
  （实跑 `dev+38f53fd`＝支构建戳，运行版可暂异）；已申报，proto 面缺口
  属范围外，记档。
- `--socket`→`*.sock→*.db` 推导依赖 serve 双旗同位约定（`--db` 显式
  可覆盖，库缺失报错不建空库）——已申报，记档。
- 活席计数＝seat_run dead=0 全量含 adjunct 席——已申报，票面未分
  口径，记档。
- 机器记录未落独立 `/tmp/t0051*.machine-record.txt`，以实证档「机器
  记录」节载（票验证命令「四件＋实跑输出实证」未钦定文件落点）——
  本判以审计侧独立复算为准，记档不否决。
- 可沉淀打法候选附言：**可沉淀**——「运维只读 CLI＝--socket 双职
  （Ping 拨验＋`*.sock→*.db` 库径推导）＋--db 覆盖＋库缺失拒建空库＋
  拨验失败降级不拒」是干净的可复用范式；「活网关调和器迁边——计数
  断言要离库测」坑档已入经验库，双面拆测法（连通面挂活网关/计数面挂
  独立库）同样值得沉淀。

## 独立复算

- `go build ./...` → rc=0（worktree @38f53fd）
- `go vet ./...` → rc=0
- `gofmt -l cmd/meta/main.go cmd/meta/main_test.go` → 零命中
- `bash scripts/selfcheck.sh` → rc=0（Ticker/Sleep/出 adapter 的
  encoding/json 全零命中）
- `go test ./...` → 全绿（cmd/meta 0.578s、internal/seat 18.443s、余包
  cached ok，无一失败行）
- `go test ./cmd/meta/ -run TestStatusCLI -v` → PASS（0.23s）
- 交付支构建二进制实跑生产 socket：`meta dev+38f53fd svc=up 在途=1
  已立=0 设计中=0 已派=0 施工中=0 待审=0 审计中=1 待部署=0 部署中=0
  blocked=1 活席=2`，rc=0，严格单行
- `git diff master...HEAD | grep -nE 'TODO|FIXME|XXX|HACK|绕过|半成品|
  待补'` → 零命中（暗债对账零瞒报）
- 事件史：deliver(38f53fd, exit=0)→audit_start 链完整；施工 sentinel.log
  原文在案
