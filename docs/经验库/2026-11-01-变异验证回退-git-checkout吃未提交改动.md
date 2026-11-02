日期：2026-11-01
席位：施工（票0009）
现象：对抗性锚变异验证（sed 删行→跑测试→回退）用 `git checkout -- <file>`
回退——回退恢复的是 **HEAD**，未提交的本票改动整文件丢失（caster.go 的
role 闸、dispatch.go 的空提示闸+reportShell 重构两件重做）。
一句话教训：**变异验证的回退件先 `cp` 备份或 `git stash`，禁 `git checkout`**——它恢复的是 HEAD，不是你五分钟前的编辑态。

## 复现
`sed -i 's|锚行|if false && 锚行|' file.go && go test` 后
`git checkout -- file.go` —— 连本票未提交的全部改动一起回退，编译面被
attach.go 新签名打碎才暴露。

## 解法
```bash
cp file.go /tmp/x.bak && sed -i '锚行变异' file.go && go test -run 锚 ./pkg/ \
  && cp /tmp/x.bak file.go
```
（或 `git stash` 亦可；`git checkout` 只对「确要扔掉的改动」用。）

## 补记（2026-11-02·票0021 接替复验）：变异必须保编译，编译断的红证无效

删 `if !matched { ... }` 整分支做变异 → `matched` 变未用变量 → `[build failed]`——
测试包根本没跑到断言，红的是编译器不是锚。**编译断只证明「删法删不动」，不证明
「锚能抓语义回归」**；语义删除在真实世界常伴随顺手清变量，照样编译、静默坏死。
保编译死分支法：条件改 `if matched && false {`（变量仍被读，Go 不报 unused，
分支必不进）——TestDefaultNotify 立刻出断言级红 `notify=0 预期 2`。

一句话：**变异验证的红必须是 `--- FAIL: <锚测名>` 断言行**；出的若是
`[build failed]`，换保编译的变异重做。

## 引用
票0009 施工过程；commit t0009。票0021 对抗锚①复验（commit a275f9c 接替复跑）。
