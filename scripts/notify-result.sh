#!/usr/bin/env bash
# notify-result.sh <票号> <环节> <判决/结果文件> <退出码>
# 业主通知的中文模板器——**绝不透传席位原文**（英文/黑话止于此）。
# 环节 ∈ 施工|查验；结果文件若含「判决：通过/驳回/票面待裁」则取其行，否则报「结果没出来」。
set -euo pipefail
TICKET="${1:?票号}"; STAGE="${2:?环节}"; FILE="${3:?结果文件}"; RC="${4:-?}"

if [ "$RC" = "0" ]; then RCN="正常结束"; else RCN="中途退出（$RC）"; fi
LINE=$(grep -m1 '^判决：' "$FILE" 2>/dev/null || true)
case "$LINE" in
  *通过*)   RESULT="✅ 查验通过";;
  *驳回*)   RESULT="❌ 查验不通过，需要返工";;
  *票面待裁*) RESULT="⚠️ 任务单本身有问题，CEO 正在处理";;
  *)        if [ "$STAGE" = "施工" ]; then RESULT="干完了（$RCN），接下来安排查验"; else RESULT="结果没出来（$RCN），正在处理，下一条通知为准"; fi;;
esac

/opt/meta/scripts/feishu-notify.sh "【Meta】${TICKET}：${RESULT}。"
