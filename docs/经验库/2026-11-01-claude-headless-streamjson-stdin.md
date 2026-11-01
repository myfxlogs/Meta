日期：2026-11-01
席位：施工（票0002）
现象：claude 无 `--prompt-file` 旗——提示词走 stdin 重定向是等价径（`cat f | claude -p` 实测通）
一句话教训：claude 无头席位起法＝`claude -p --output-format stream-json --verbose --permission-mode acceptEdits --permission-prompts none`，提示词 stdin 喂；用量在 `result` 帧 `usage`（input/output/cache 三段）＋`total_cost_usd`

## 环境
Claude Code 2.1.270（本机默认模型 glm-5.3）

## 实测结论（无头档位）
- 提示词：stdin 重定向（positional 内联也行但一律文件径纪律→stdin 等价）
- 流式：`--output-format stream-json`＝JSONL：system.init / assistant / result
- 许可：`--permission-prompts none`＝会提示的一律自动答拒（禁区#12）；`--dangerously-skip-permissions` 在无审批语境会被安全闸拦（前条经验库）——acceptEdits+none 是干净径
- 用量：result 帧 `usage.{input,output,cache_read,cache_creation}_tokens`＋`total_cost_usd`——口径＝四件全计
- 冒烟实测：EXIT=0，result 帧 15301 tokens / $0.0213

## 引用
internal/adapter/claude.go
