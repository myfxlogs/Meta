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

## 引用
票0009 施工过程；commit t0009。
