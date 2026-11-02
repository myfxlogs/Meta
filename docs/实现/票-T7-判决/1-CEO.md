# 票-T7 判决（0032 席位凭据注入与 HOME）——CEO

支 t/t7-seat-secrets｜deliver_ref=d6c180d｜基 c18ecf4（master 基全量续建）

## 判决：通过（并入主线）

## 三锚实证
- 锚1 放行落盘：handleSecret ACL 放行支内写 run/secrets/<runid>/<name>（0700/0600），dispatch 预建＋META_SECRETS_DIR＋HOME/seat-secrets 符号链——TestT7 断言权限与可读
- 锚2 fail-closed：resp 缺省 Denied:true（ACL miss/库错/vaultKey 空全归拒）→secret_denied 事件＋emit；拒件不落盘（写径仅在放行支内，测试断言 IsNotExist）
- 锚3 死席回收：onRunDead RemoveAll run/secrets/<runid>，测试实证

## 偏差（记债不卡收）
- D-84：「只读挂载」为符号链近似——同 uid 席可写自身凭据；真 RO 归 req-f88 容器笼域
- D-85：孤儿席（reconcile killRunGroup 径）不走 onRunDead→secrets 目录残留（0700 防护在）

## 独立复算
build/vet 绿；248 测全绿/16 包（含 TestT7_ScopedSecrets 三锚）。交付方 selfcheck 项实为 usage 输出（无此子命令），check 配方口径＝build+vet+test，已核。
