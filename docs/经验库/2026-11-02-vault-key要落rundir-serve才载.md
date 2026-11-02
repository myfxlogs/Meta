日期：2026-11-02
席位：施工（票0023）
现象：scratch 验收脚本 `secret put --key $TMP/vault.key` 落种后，`meta board` 恒
Unauthenticated「token 无效或行缺」——密文明明在库、token 明明对
一句话教训：**vault.key 必须落在 serve 的 rundir 里**（网关只读
`<rundir>/vault.key`，gateway.go:236）；`secret put --key` 指到别处＝种了
一把网关永远不载的钥——token 闸 fail closed 表现为「token 无效」而非「钥缺」

## 环境

scratch 网关（`meta serve --db --socket --rundir`）＋`meta secret put` CLI

## 复现

1. `meta secret put --db $TMP/meta.db --key $TMP/vault.key secret.board.token <<< tok`
2. `meta serve --db … --rundir $TMP/run …`（钥在 $TMP/，rundir 是 $TMP/run/）
3. `meta board --connect … --token tok` → Unauthenticated（CheckBoardToken：
   `g.vaultKey == nil` → 直接 false，解密根本没走到）

## 解法/绕法

- `--key` 与 `--rundir` 对齐：`KEY=$RUND/vault.key`（票0023 首跑实证修后即绿）
- 判据：secret put 与 serve 各有各的钥路径约定（put 缺省 `run/vault.key` 恰与
  rundir 缺省重合——自定路径一拆就分家）；诊断时先查网关侧钥路径，别盯 token 值

## 引用

internal/gateway/gateway.go:236（RunDir 拼 vault.key，缺文件＝vaultKey nil）；
internal/gateway/board.go CheckBoardToken（fail closed 链）；scripts/board-verify.sh ①
