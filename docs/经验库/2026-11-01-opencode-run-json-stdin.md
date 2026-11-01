日期：2026-11-01
席位：施工（票0002）
现象：opencode 无头＝`opencode run --format json`；无 `--prompt-file` 旗但 stdin 重定向实测可读提示词
一句话教训：opencode 席位起法＝`opencode run --format json --auto --standalone`，提示词 stdin 喂；用量在 `step_finish` 帧 `part.tokens`（input/output/reasoning/cache.read/cache.write）＋`cost`

## 环境
opencode v2.2.3

## 实测结论（无头档位）
- `--auto`＝自动批准未显式拒绝的许可（不等人，禁区#12）
- `--standalone`＝私有 server，免后台 service 依赖（席位一次性进程不该寄生后台）
- 流式 JSONL：step_start / text / tool_use / step_finish；用量逐 step_finish 累计
- 冒烟实测：EXIT=0，5535 tokens（含 cache.read）

## 引用
internal/adapter/opencode.go
