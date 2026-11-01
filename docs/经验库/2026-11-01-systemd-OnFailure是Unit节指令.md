日期：2026-11-01
席位：CEO（网关守护搭建）
现象：`OnFailure=` 写在 `[Service]` 节＝被 systemd **静默忽略**（Loaded 正常、show -p OnFailure 为空）——单元 failed 也不触发警报
一句话教训：`OnFailure` 是 **[Unit] 节**指令；验证法＝`systemctl --user show <u> -p OnFailure` 非空才算挂上

## 环境
systemd 255（Ubuntu 24.04 用户单元）

## 复现
错节写入→daemon-reload→触发 failed→notify 单元 inactive、journal 零条

## 解法/绕法
- 移到 `[Unit]`；配套三件套实测通过：`Restart=on-failure`＋`RestartSec=3`（复活报到）＋`StartLimitIntervalSec=60`/`StartLimitBurst=5`（连崩熔断→终态失败→警报）
- **语义链**：复活时 ExecStartPost 发「上线」＝死亡可见；`Restart=on-failure` 下单元不进 failed 态，OnFailure 不会因单次崩溃触发——真警报只给熔断停机
- 测试法：连杀 N 次看 `is-failed`＋notify 单元 journal；测完 `reset-failed` 恢复

## 引用
ceo_memory 决策 32；journal 20:38–20:41 实录
