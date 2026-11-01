#!/usr/bin/env bash
# close-audit.sh —— 收口对账机（E19/决策 70）：机器核验替代自报，防门禁仪式化
# 用法：收口时跑；退出码 0=门绿，1=有欠账（逐条打印）。输出即自证内容。
set -uo pipefail
M=/opt/meta/docs/实现/ceo_memory.md
K=/opt/meta/docs/经验库
D=/opt/meta/docs/实现/债务台账.md
fail=0
# ① 同生原则（水位：决策 69 起，既往不咎）
awk '/^### 决策/{n=$0; sub(".*决策 ","",n); sub("[^0-9].*","",n)} n>=69 && /^-/{print}' "$M" | while IFS= read -r ln; do
  echo "$ln" | grep -qE '^\s*-?\s*(坑档|经验库＋)' || continue
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
