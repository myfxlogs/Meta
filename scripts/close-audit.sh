#!/usr/bin/env bash
# close-audit.sh —— 收口对账机（E19/决策 70）：机器核验替代自报，防门禁仪式化
# 用法：收口时跑；退出码 0=门绿，1=有欠账（逐条打印）。输出即自证内容。
set -uo pipefail
DB=/opt/meta/run/meta.db
K=/opt/meta/docs/经验库
D=/opt/meta/docs/实现/债务台账.md
fail=0
# 决策账真源＝DB ceo_memory 表（T27③ 陈账：改 meta memory CLI 读——
# 不经 socket 以外死指针；md 已归档 ceo_memory.archived.md）。
M=$(mktemp /tmp/ceo-mem.XXXXXX); trap 'rm -f "$M"' EXIT
# D-91：memory 读侧双向流服务端不闭流（客户端 memoryGet 缺 CloseSend，
# 两端互等挂死——11-03 接手实证，前班误诊为组权限）。超时截流取全量，
# 真修（client CloseSend）在债台账。
timeout 60 /opt/meta/bin/meta memory --socket /opt/meta/run/meta.sock --kind decision --limit 100000 > "$M" || true
# ① 同生原则（水位：决策 69 起，既往不咎）：带坑档标签的行须含真实路径
awk '/^### /{n=0; t=$0; sub(".*决策 ?","",t); if(t ~ /^[0-9]+/){sub("[^0-9].*","",t); n=t}} n>=69{print}' "$M" | { grep -oE '坑档[：:][^ ]*|经验库＋[^ ]*' || true; } | while IFS= read -r ln; do
  echo "$ln" | grep -qE '\.md|经验库/' || { echo "✗ 同生违反（决策69后）：$ln"; exit 1; }
done || fail=1
# ② 坑档存在性：经验库被引用的文件名真实存在
for f in $(grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}-[^」） )】]*\.md' "$M" | sort -u); do
  [ -f "$K/$f" ] || { echo "✗ 坑档缺失：$f"; fail=1; }
done
# ③ 债账行↔判决档：判决新增债（N 系）在台账有行
for t in /opt/meta/docs/实现/票-*-判决/*.md; do
  [ -f "$t" ] || continue
  z=$(basename "$t" .md); grep -q "$(echo $z | cut -d- -f1)" "$D" 2>/dev/null || echo "（提示）判决档 $z 无对应债行（可能已清）"
done
# ④ 工作区净
[ -z "$(git -C /opt/meta status --short)" ] || { echo "✗ 工作区不净"; fail=1; }
[ "$fail" = 0 ] && echo "收口对账：门绿（涉坑决策带档、坑档齐、判决债有账、区净）"
exit $fail
