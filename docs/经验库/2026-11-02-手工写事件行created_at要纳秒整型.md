# 手工写 ticket_event：created_at 必须纳秒整型

日期：2026-11-02
席位：CEO（devin 上身）
现象：INSERT 事件行后 caster 反复「流断 Scan error on column index 8, name "created_at"」重连，spawn_needed 全停摆
一句话教训：外部直改库只许 UPDATE，事件行一律 meta inject/代码径——手工 INSERT 的 datetime() 字符串会毒死整条事件流

## 环境
SQLite 单库，`ticket_event.created_at` 列型 INTEGER 存 unix 纳秒（`1793586939235427465` 量级）。caster 经 Observe/EventsAfter 扫行驱动全部派发。

## 复现
`INSERT INTO ticket_event (..., created_at) VALUES (..., datetime('now'))` → 行内 created_at 为 `2026-11-02 02:38:41` 字符串 → caster 扫到即 `converting string to int64` 崩流 → 退避重连 1s→1min 循环，水位停摆。

## 解法
UPDATE 该行改纳秒整型（取邻行量级：epoch秒×1e9，保持与同 id 邻行单调）。规程入 D-56：外部直改库只允许 UPDATE 相位/dead 列；事件行永远走 `meta inject` 或 AppendEvent 代码径。

## 引用
D-56 台账行；caster 日志 `Scan error ... created_at`；修复实证 ticket_event id=2673。
