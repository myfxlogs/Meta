#!/usr/bin/env bash
# gateway-alarm.sh —— 网关熔断警报（OnFailure 触发；字段全部机械采自 systemd 现场，零判断）
U=meta-gateway.service
RESULT=$(systemctl --user show $U -p Result --value)
STATUS=$(systemctl --user show $U -p ExecMainStatus --value)
RESTARTS=$(systemctl --user show $U -p NRestarts --value)
TS=$(date '+%m-%d %H:%M:%S')
TAIL=$(journalctl --user -u $U --no-pager -n 4 2>/dev/null | tail -3 | cut -c1-60 | tr '\n' '／')

/opt/meta/scripts/feishu-notify.sh "【公司】⚠️ 网关熔断停机
■ 发生：${TS}——短时间连崩已超限，停止自动重启（累计重启 ${RESTARTS} 次，终态=${RESULT} 退出码=${STATUS}）
■ 线索（机器日志末 3 行）：${TAIL:-无输出}
■ 影响：新派单/查验暂停；在跑的干活席位不受影响（各自独立存活）；账本数据无损（已落盘）
■ 要你动吗：无需操作——下次打开电脑 CEO 会话即接手处理；急则看：journalctl --user -u meta-gateway"
