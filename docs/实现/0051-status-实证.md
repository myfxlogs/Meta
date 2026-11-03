# 0051 `meta status` 一行状态命令——实证档（施工留档）

分支：t/0051-status-cmd｜锚测＝cmd/meta/main_test.go `TestStatusCLI`

## 改动面

- `cmd/meta/main.go`：`status` 子命令＋usage 行＋`statusCmd`。
  - 版本戳：`buildinfo.Version(buildStamp, buildinfo.Short())`——同
    `meta version`（T38/票0048 版本戳径）。
  - svc（票面「服务相位」落义）：既有 `AdminGateway.Ping` 拨验（票0010
    健康核验径）——答则 `up` 不答则 `down`；拨不通不拒，降级照出库
    计数（网关宕时本令恰是最需要的一瞥）。`--timeout` 默认 3s。
  - 在途/相位计数/blocked/活席：DB 纯读拼装——`NonTerminalTickets`
    （phase NOT IN 完成/作废）按相位分桶，blocked（§3.3-B 人工态）单列，
    `ActiveRuns`（seat_run dead=0）计活席。零新 SQL 面零写径。
  - 库径：`--socket` 推导 `*.sock→*.db`（同 0050/同 serve 双旗布局），
    `--db` 显式覆盖；库文件缺失即报错不建空库。

## 输出形（恒单行）

```
meta <版本戳> svc=<up|down> 在途=<N> 已立=n 设计中=n 已派=n 施工中=n 待审=n 审计中=n 待部署=n 部署中=n blocked=n 活席=n
```

- 在途＝非终态非 blocked 合计（八个相位逐项即分明细）；blocked 单列
  不计入在途——票面三要素各一值，无歧义。
- 空板所有计数显式零（锚：空板显式零）。

## 机器记录（worktree 内）

- `go build ./...` OK；`go vet ./...` OK；`gofmt -l` 触碰件零命中
  （internal/ 下存量未格式化件系 master 原有，非本票触碰，不动）。
- `go test ./...` 全绿（gateway 38.8s / seat 18.4s 一跑全过）。
- `scripts/selfcheck.sh` 零命中（无周期件/无 JSON/无第二写者）。
- `go test ./cmd/meta/ -run TestStatusCLI` PASS——真网关 socket 上
  svc=up＋推导库径／空板逐项零／满板八相位各一＋blocked 单列＋终态不
  计／活席 dead=0 计 dead=1 不计／死 socket svc=down 降级照出／库缺失
  报错／严格单行六面一测。

## 实跑输出实证

生产库 `/opt/meta/run/meta.sock`（本票施工中实态——施工中=1 即 0051
自身席，blocked=1 即 0050，活席=2）：

```
$ meta status --socket /opt/meta/run/meta.sock
meta dev+5c7a50d svc=up 在途=1 已立=0 设计中=0 已派=0 施工中=1 待审=0 审计中=0 待部署=0 部署中=0 blocked=1 活席=2
```

死 socket＋--db 降级径（svc=down 不拒，计数照出）：

```
$ meta status --socket /tmp/nonexistent.sock --db /opt/meta/run/meta.db
meta dev+5c7a50d svc=down 在途=1 已立=0 设计中=0 已派=0 施工中=1 待审=0 审计中=0 待部署=0 部署中=0 blocked=1 活席=2
```

空板显式零（空 scratch 库）：

```
$ meta status --socket /tmp/empty-board/meta.sock
meta dev+5c7a50d svc=down 在途=0 已立=0 设计中=0 已派=0 施工中=0 待审=0 审计中=0 待部署=0 部署中=0 blocked=0 活席=0
```

库缺失报错不建空库：`meta: status: 库不可读 /tmp/x/none.db: stat …`，
退码 1，`/tmp/x/none.db` 未被创建（实证）。

## 范围外确认

未动调和器/失败表/通知面（票面 R2′ 禁线）；无新周期件零写径；AdminGateway
只用既有 Ping；DB 只调既有 NonTerminalTickets/ActiveRuns——无 proto/网关/
schema 改动（cmd/meta 单文件＋测试件＋本档三面）。

## 遗留申报

- 「服务相位」票面未给钦定字段名——本交付落为 `svc=up|down`（Ping 拨验
  语义）。若钦定另有指（如 systemd 双件相位），属字段重命名级修正。
- 版本戳为**本二进制**构建戳（与 `meta version` 同源同径）。AdminGateway
  现有 RPC 无「运行中网关版本」读径——若网关未随换装重启，运行版与二
  进制版可能暂异，属 RPC 面缺口（加字段须动 proto，范围外）。
- `--socket` 推导 `*.sock→*.db` 依赖 serve 双旗同位约定（同 0050 申报）；
  异布局可 `--db` 显式指。
- 活席计数＝seat_run dead=0 全量（含 adjunct 席）——口径随库真源，未做
   adjunct 剔除（票面未分口径）。
