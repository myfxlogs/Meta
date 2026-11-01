日期：2026-11-01
席位：CEO（手工派发径）
现象：devin `--sandbox` 无头模式下，**Write 工具调用仍要求确认**→被拒——审计席取完全证、推理完成，判决最后一步写不出来（EXIT=0 假成功）
一句话教训：无头席位的产出出口＝**stdout 重定向落档**，别依赖它的文件写权限

## 环境
devin 3000.11.234，`-p --sandbox`，bwrap+socat 已装齐

## 复现
审计席全流程跑完，日志尾 `warning: rejected a tool call that requires confirmation`——正是写判决文件那一步

## 解法/绕法
- 同会话续跑＋判决走 stdout：`devin -p -c --sandbox --prompt-file <续跑指令>` ＋ `> 判决文件` 重定向——判决仍出自审计实例，CEO 只提供管道不代笔
- **固化**：无头席位提示词的产出条款一律写「输出到 stdout」，调用方重定向落档；文件写仅当 sandbox 明确放行才作为路径

## 引用
`t0001.devin-audit.log`；ceo_memory 决策 10
