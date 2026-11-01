#!/usr/bin/env bash
# feishu-notify <文本> —— CEO 手工径单向通知（自举 stopgap）
#
# 钦定宿主是网关外部面（总设计 §8、禁区#16「通道归网关」）；本件是网关立起前的
# 替身：**只出站、只发 FEISHU_MANAGE_CHAT_ID、无守护、无轮询**——调用即发，发完即退。
# 调用点＝事件退出钩子（进程退出/判决落档时），不做任何定时。
#
# 凭证：读迁移源 env（凭证清单钦定路径），600 校验；JSON 止于本件——
# 本件即 R3 所指「适配层」，阶段 3 网关接管后随票退役。
set -euo pipefail
ENV_FILE="${FEISHU_MANAGE_ENV:-/opt/meta-archive/feishu-manage.env}"
MSG="${1:?用法: feishu-notify <文本>}"

[ "$(stat -c %a "$ENV_FILE")" = "600" ] || { echo "env 权限非 600，拒读" >&2; exit 2; }
APP_ID=$(grep -oP '^FEISHU_MANAGE_APP_ID=\K.*' "$ENV_FILE")
APP_SECRET=$(grep -oP '^FEISHU_MANAGE_APP_SECRET=\K.*' "$ENV_FILE")
CHAT_ID=$(grep -oP '^FEISHU_MANAGE_CHAT_ID=\K.*' "$ENV_FILE")
[ -n "$APP_ID" ] && [ -n "$APP_SECRET" ] && [ -n "$CHAT_ID" ] || { echo "env 键缺" >&2; exit 2; }

TOKEN=$(curl -s -m 10 https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal \
  -H 'Content-Type: application/json' \
  -d "{\"app_id\":\"$APP_ID\",\"app_secret\":\"$APP_SECRET\"}" \
  | grep -o '"tenant_access_token":"[^"]*"' | cut -d'"' -f4)
[ -n "$TOKEN" ] || { echo "token 获取失败" >&2; exit 3; }

BODY=$(python3 -c 'import json,sys
s=sys.argv[2].encode("utf-8","replace").decode("utf-8","replace")
print(json.dumps({"receive_id":sys.argv[1],"msg_type":"text","content":json.dumps({"text":s},ensure_ascii=False)},ensure_ascii=False))' "$CHAT_ID" "$MSG")
[ -n "${META_NOTIFY_DEBUG:-}" ] && printf '%s' "$BODY" > /tmp/notify-body.json
curl -s -m 10 "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "$BODY" \
  | grep -o '"code":[0-9]*' || true
echo
