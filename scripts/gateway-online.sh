#!/usr/bin/env bash
# gateway-online.sh —— 网关上线报到（ExecStartPost；复活计数>0 即附带死亡信号）
N=$(systemctl --user show meta-gateway.service -p NRestarts --value)
if [ "${N:-0}" -gt 0 ]; then
  /opt/meta/scripts/feishu-notify.sh "【公司】网关已上线（这是崩溃后第 ${N} 次复活——此前出过事但已自愈，无需操作）。"
else
  /opt/meta/scripts/feishu-notify.sh "【公司】网关已上线（systemd 守护：异常自动重启＋熔断警报）。不用你操作。"
fi
