# 0050 `meta blocked` 列表命令——实证档（施工留档）

分支：t/0050-blocked-list｜锚测＝cmd/meta/main_test.go `TestBlockedCLI`

## 改动面

- `internal/db/ticket.go`：`BlockedRow`＋`BlockedRows(ctx)`——`phase='blocked'`
  票 LEFT JOIN 各票最近一条 `kind='blocked' AND to_phase='blocked'` 迁移边
  事件（note＝机器原因原文；其后的「票 blocked：…可选动作」附言行
  to_phase 为空天然不命中）。纯读零写，票号升序。
- `cmd/meta/main.go`：`blocked` 子命令＋usage 行。`--socket` 推导库径
  （`*.sock→*.db`，同 serve 双旗布局）；`--db` 显式覆盖；库文件缺失即
  报错不建空库。

## 机器记录（worktree 内，GOCACHE/GOPATH 工作区重定向）

- `go build ./...` OK；`go vet ./...` OK；`gofmt -l` 触碰件零命中
  （顺手清掉 1441 行存量多空行——非本票引入）。
- `go test ./...`：除 `internal/gateway` 外全绿；gateway 包三跑各挂
  不同测（TestCeoRule_RedirectPrompt→TestDispatch_PromptFile覆盖/
  TestT30→TestBudget_ThreeGates/TestT30），**master 干净检同挂
  TestBudget_ThreeGates**（e069874 detached 复算 FAIL）＝存量环境抖动，
  与本票读面无关；挂测单跑全 PASS。`scripts/selfcheck.sh` 零命中。
- `go test ./cmd/meta/ -run TestBlockedCLI` PASS（空表/一票带因/多票序/
  非 blocked 零出现/socket 推导库径五面一测）。

## 实跑输出实证（/tmp/blocked-demo 对照库＋生产库）

对照库造两 blocked 票（B900 含「blocked 附言行」对照、B901）＋一施工中
票 N100：

```
$ meta blocked --socket /tmp/blocked-demo/meta.sock
B900	试车对照票·部署连败	待部署→blocked
	末因: 失败表到限 kind=spawn_failed 计数=4>cap=3：next_shell
	动作: 放行/重试 meta rule --action redirect --ticket B900｜作废 meta rule --action void --ticket B900
B901	试车对照票·席秒退	施工中→blocked
	末因: 未知/禁动失败 kind=seat_dead——人工裁定
	动作: 放行/重试 meta rule --action redirect --ticket B901｜作废 meta rule --action void --ticket B901
```

- 末因取迁移边行原文（非附言「票 blocked：…可选动作」行）——B900 两行
  同 kind 实照选对。
- N100（施工中）零出现；全表作废后输出「无 blocked 票」。
- 生产库 `/opt/meta/run/meta.sock`→`meta.db` 实跑：当前零 blocked 票，
  输出「无 blocked 票」；直读校验 `COUNT(*)=0` 相符。

## 范围外确认

未动 rule/调和器/失败表；零通知面；零写径（db.Open 既有只读用法同
catchrate，库存在性先 Stat 防误建）；无新周期件（自检口令过）。

## 遗留申报

- 「放行」语义在网关是 AdminGateway.Resume RPC（回 blocked 边记的原相位）
  ——票面钦定提示写 `meta rule redirect`，目前 redirect 对 blocked 票
  会被闸拒（非 req_pending/待部署）。提示文字按票面原样落，redirect 对
  blocked 放行语义属网关面，本票范围外——建议 CTO 另立票补 `meta resume`
  CLI 或扩 redirect 受理面。
- `--socket` 推导 `*.sock→*.db` 依赖 serve 双旗同位约定；异布局可 `--db`
  显式指。
