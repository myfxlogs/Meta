# 0052 `meta tickets` 在途票列表——实证档（施工留档）

分支：t/0052-tickets-list｜锚测＝cmd/meta/main_test.go `TestTicketsCLI`

## 改动面

- `cmd/meta/main.go`：`tickets` 子命令＋usage 行＋`ticketsCmd`＋`clipTitle`。
  - 列表：`NonTerminalTickets`（phase NOT IN 完成/作废——blocked 系非终态
    照列，同 0050/0051 口径）按票号字典序重排输出；既有读径零新 SQL。
  - 行式：`<票号>\t<相位>\t<标题>`——标题 `clipTitle` 截 40 rune＋「…」
    （按 rune 不切多字节）；`\n/\r/\t`→空格一行一票格式自保（决策67
    同族净化）。
  - 空板：`len(ts)==0` 显式「无在途票」单行。
  - 库径：`--socket` 推导 `*.sock→*.db`（同 0050/同 serve 双旗布局），
    `--db` 显式覆盖；库文件缺失即报错不建空库。纯读零写（db.Open
    既有只读用法同 blocked/catchrate）。

## 输出形

```
0050	blocked	meta blocked 列表命令（v2运维件·试车票）
0051	blocked	meta status 一行状态命令（复测试车）
0052	施工中	meta tickets 在途票列表（白纸试车）
```

一行一票：票号 TAB 相位 TAB 标题截断；票号升序；终态（完成/作废）不出现。

## 机器记录（worktree 内）

- `go build ./...` OK；`go vet ./cmd/meta/` OK；`gofmt -l` 触碰件零命中。
- `go test ./cmd/meta/` 全绿；`bash scripts/selfcheck.sh` 零命中
  （无周期件/无 JSON/无第二写者）。
- `go test ./cmd/meta/ -run TestTicketsCLI` PASS——空板单行「无在途票」／
  反序立票按票号序（T04<T07<T18<T30）／终态 T12/T25 零出现／blocked
  系非终态照列／长题 40rune+…恰 41 rune、短题原样／毒标题 \n\t 消毒
  不裂行／--socket 推导径同 --db 输出／库缺失报错不建空库——八面一测。
  断言全在无网关独立库上做（0051 坑档口径：活网关调和器迁边，相位
  断言离库测）。

## 实跑输出实证

生产库 `/opt/meta/run/meta.sock`（本票施工中实态——0052 自身席施工中，
0050/0051 blocked 系非终态照列）：

```
$ meta tickets --socket /opt/meta/run/meta.sock
0050	blocked	meta blocked 列表命令（v2运维件·试车票）
0051	blocked	meta status 一行状态命令（复测试车）
0052	施工中	meta tickets 在途票列表（白纸试车）
```

空板显式「无在途票」（空 scratch 库）：

```
$ meta tickets --socket /tmp/empty-0052/meta.sock
无在途票
```

库缺失报错不建空库：`meta: tickets: 库不可读 /tmp/none-0052/meta.db: stat
… no such file or directory`，退码 1，`/tmp/none-0052/meta.db` 未被创建
（实证）。

## 范围外确认

未动调和器/失败表/通知面（票面 R2′ 禁线）；无新周期件零写径；无 proto/
网关/schema 改动——cmd/meta 单文件＋测试件＋本档三面。

## 遗留申报

- 标题截断长度票面未钦定——本交付落 40 rune＋「…」（一瞥门面口径，
  长题截断指示可辨）。若钦定另有宽度/省略记号属常数级修正。
- 「按票号序」落为 id 字典序——零填充票号（0050/0051）自然序；混合
  id 形（req-*/T*）按字典序排，非数字值比较。
- blocked 票（非终态）照列——票面「全部非终态」直读口径；0051 status
  的「在途」计数以 blocked 单列剔除系另一口径，本票锚按「非终态」。
- `--socket` 推导 `*.sock→*.db` 依赖 serve 双旗同位约定（同 0050/0051
  申报）；异布局可 `--db` 显式指。
- 本席系接替场景：接手时支上已有前任壳未提交的实现主体（子命令＋
  测试骨架，`git status` 两 M 零 commit）。经复核沿用，本席增补标题
  消毒面（\n/\t 防裂行）＋对应用例与本档。
