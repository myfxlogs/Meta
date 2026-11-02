日期：2026-11-02
席位：施工（票0023）
现象：验收脚本失败退场后 scratch `meta serve` 仍活着、TMP 未删——trap 清场
只跑了第一条 kill（client 已死→kill 退非零→set -e 把 trap 函数就地正法）
一句话教训：**set -e 脚本的 trap 里每条 kill/删库语句都要 `|| true` 兜底**——
清场函数死在半路＝泄漏比不清还隐蔽（进程在、口占着、tmp 留着）

## 环境

bash set -euo pipefail ＋ `trap cleanup EXIT` ＋ 后台子进程三枚（serve/client/REPL）

## 复现

```
cleanup() {
  [ -n "$CLIENT_PID" ] && kill "$CLIENT_PID" 2>/dev/null   # client 已退→kill 退 1
  [ -n "$SERVE_PID" ] && kill "$SERVE_PID" 2>/dev/null     # ← 永远执行不到
  rm -rf "$TMP"                                            # ← 同上
}
```
反锚径（client 秒死）一跑：serve 存活、socket 占用、TMP 残留——「红」得
不彻底，还坑下一跑。

## 解法/绕法

- 每行 `… && kill … 2>/dev/null || true`；删目录同款兜底
- 检验法：故意跑失败径后 `pgrep -f <tmp路径>`＋`ls -d $TMP`——两者皆空才算
  trap 真的全程走完（票0023 A3 锚就是这么红的）

## 引用

scripts/board-verify.sh cleanup（首跑泄漏实证→修后全径清场）；同类：
2026-11-01-变异验证回退-git-checkout吃未提交改动.md（脚本半路死＝状态烂在半路）
