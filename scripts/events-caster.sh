#!/usr/bin/env bash
# events-caster.sh —— 网关事件播报员（自举期 stopgap，阶段 3 由网关飞书通道接管后退役）
# 消费 observe 流（纯事件驱动，无轮询无定时）；席位起落关键节点→飞书纯中文模板（决策 23 双禁）。
# 原始事件行含英文/黑话，一律不透传——只出票号＋中文定性。
exec /opt/meta/bin/meta observe --socket /opt/meta/run/meta.sock 2>/dev/null | while IFS= read -r line; do
  tid=$(printf '%s' "$line" | awk '{for(i=1;i<=NF;i++) if($i ~ /^[0-9]{4}$/){print $i; exit}}')
  [ -z "$tid" ] && continue
  case "$line" in
    *claim*)            k="开工：干活席位已领票";;
    *deliver*)          k="完工：交付待查验";;
    *shells_exhausted*) k="⚠️ 换壳链走完仍失败（全壳耗尽）";;
    *budget_frozen*)    k="⚠️ 预算冻结止血";;
    *stall_suspect*)    k="⚠️ 疑似卡死，待裁决";;
    *dispatch*)         k="已派单";;
    *) continue;;
  esac
  /opt/meta/scripts/feishu-notify.sh "【公司】票务动态（第${tid}号票）：${k}。不用你操作。" || true
done
