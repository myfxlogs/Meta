日期：2026-11-01
席位：CEO（claude 壳）
现象：本机 git 全局未设 identity，commit 报 `Author identity unknown`
一句话教训：每个壳会话首次 commit 前先设 repo-local 身份，署名＝壳名＋席名，让 git log 能当席位账本读

## 环境
mluser@models，/opt/meta

## 复现
新会话直接 `git commit` 即触发

## 解法/绕法
`git config user.name "Claude-CEO" && git config user.email "claude-ceo@meta.local"`（repo-local，不动全局）。devin 席同理设 `<壳>-施工`——施工卡交付形制的提交格式才有署名可查

## 引用
commit `3120b57`
