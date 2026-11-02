# T18（外派·凭据残面两修）判决：通过（CEO 终端形态·决策 97，2026-11-02）

- 交付：支 `t/t18-secrets-harden` @ `630e30d`（worktree 合规）。独立复算：
  build/vet RC=0、`go test ./...` **270 绿/16 包**（与申报一致）、selfcheck RC=0。

## 逐项对账

| 项 | 判 |
|---|---|
| ①真只读：目录 0555/文件 0444；写径临时松 0700→写→回落 | 绿——同 uid open(O_WRONLY)/新建俱 EACCES（锚实测）；**chmod 二步升级洞已主动申报**（E6 合规）：收敛非根绝，真根绝＝uid 隔离＝T21（队列在位） |
| ②孤儿收尸连收：wipeSecretsDir 回权限位再 RemoveAll；onRunDead/孤儿两径共用 | 绿——只读目录不可删的副作用一并实修；锚＝孤儿死后 secrets 目录空 |
| 测试侧 unlinkat 闸自清 | 绿（细节到位） |

## 观察项（非缺陷）

同 run 内同名 SecretReq 重写（已存在 0444 件）会 EACCES——scoped 模型
（每 run 新目录）下不构成现径；若未来加轮换语义须先松文件位。

**销号**：D-84、D-85。

**结论：通过。合入 master，随下一班部署生效。**
