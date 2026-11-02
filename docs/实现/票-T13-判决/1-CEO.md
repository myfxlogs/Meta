# T13（外派·部署闸克隆语义修正）判决：通过（CEO 无头形态·决策 94，2026-11-02）

- 交付：支 `t/t13-clone-sync` @ `f22a575`（worktree /tmp/wt-t13 合规，
  主仓零染）。范围对票面：exec.go +32/-18＋新锚 t_t13_test.go +66，
  禁区未越（四步命令本体/B1 锁/主线历史均未动，reset 只作用克隆）。
- 独立复算（隔离 worktree，不抄交付方数字）：`go build ./...` RC=0、
  `go vet ./...` RC=0、`go test -count=1 ./...` **243 测 16 包全绿**、
  `selfcheck.sh` RC=0。

## 判

| 项 | 判 |
|---|---|
| 每 attempt 同步主线顶（fetch+checkout+reset --hard FETCH_HEAD+clean -fdq） | 绿——既有目录同步径实证 |
| 交付 ref 不再 checkout，检查/部署对象＝并后主线 | 绿——签名改取 proj.DefaultBranch，ref 仅经 META_EXEC_REF 作 integrate 输入 |
| default_branch 取项目配置不硬编码 | 绿——proj.DefaultBranch＋空值回退 master |
| 锚① 二次 attempt 主线前进可见新提交 | 绿——TestT13_CloneSync 机械实证 |
| 锚③ 克隆 status 净（clean -fdq 实证） | 绿——同测内 junk.tmp 脏残留清除断言 |
| 锚② 0034 型兄弟票修复并后检查绿 | 活锚——T12 已并主线，解冻后 0034 检查步应绿（部署波实证） |

## 结论

通过。合并后**需换程序**（exec.go 是网关码）：`go build -o /tmp/metaN
./cmd/meta && meta upgrade --new /tmp/metaN --restart-cmd "systemctl --user
restart meta-gateway"`，再按交接档§三解冻五票走批量部署。
