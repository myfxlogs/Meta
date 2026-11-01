日期：2026-11-01
席位：施工（票0002）
现象：devin `-p` 无头在新目录直接拒跑——`Refusing to run in an untrusted workspace`（EXIT=1）
一句话教训：网关 spawn 的 devin 席位起法＝`devin -p --prompt-file <f> --permission-mode dangerous --respect-workspace-trust false --export <atif>`——trust 旗不绕则哑拒；用量不在 stdout 流内，唯一来源是 ATIF 产物 `final_metrics`

## 环境
devin 3000.11.234，Linux 6.8

## 实测结论（无头档位）
- 提示词：`--prompt-file` 原生旗（内联会撞路径参数，见前条）
- 权限：dangerous（业主点名授权，决策4）；sandbox 径 exec 废（bwrap 每命令失败）
- workspace trust：无头无法答信任框→`--respect-workspace-trust false`
- 用量：stdout 纯文本零用量；`--export` ATIF 每回合重写，`final_metrics.total_{prompt,completion,cached}_tokens`——适配器退出后终读
- 冒烟实测：`SMOKE_OK` prompt 一回合出文、EXIT=0、ATIF 24748 tokens

## 引用
internal/adapter/devin.go；scripts/smoke-shells.sh
