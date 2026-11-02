# 容器笼 `--user uid:gid` 下 /etc/passwd 无条目——身份判定真源是 os.Getuid() 非名解析

日期：2026-11-02
席位：施工（票0034-p1）
现象：check 步笼内跑测试，`user.Current()`/`user.Lookup(runAs)` 双 err→
sameUser 恒 false→全部步落 legacy-sudo deny；`me, _ := user.Current()` 吞
err 裸解 `me.Uid` 直接 panic（0034 部署三连败堵全部待部署票）
一句话教训：**uid 是身份真源，名解析只是显示层**——凡「是不是同一用户」的
判定一律数字比 os.Getuid()，名只作 Lookup 输入；吞 err 后必判空。

## 环境
docker run --user $(id -u):$(id -g) golang:1.27（票0029 笼化形制）：进程
uid 真实存在但镜像 /etc/passwd 无条目——cgo getpwuid 与纯 Go 解析同败。
宿主 mluser 直跑全绿，笼内同树同测试三红，极易误判成码退化。

## 复现
```
docker run --rm --network meta-sandbox --user $(id -u):$(id -g) \
  -v <repo>:/work -w /work -v $HOME/go/pkg/mod:/gomod:ro \
  -e GOMODCACHE=/gomod -e GOCACHE=/work/.gocache -e HOME=/work \
  -e GOFLAGS=-buildvcs=false golang:1.27 \
  bash -lc "export PATH=/usr/local/go/bin:\$PATH && go test -count=1 ./internal/gateway/"
```

## 解法
- `sameUser(runAs)`：数字 run_as 直比 `os.Getuid()`；名 run_as 经
  `user.Lookup` 解出 uid 后同样数字比——自身名解析从判定径上摘除。
- `selfIDs()`：passwd 可解走 `user.Current()`，不可解回退
  `os.Getuid()/Getgid()`——笼 argv --user 不再静默落 "0:0"（那会把
  root 落盘产物写进宿主机）。
- 测试夹具：`stubSandboxSpec` 名解析失败时落 `run_as: "<uid>"` 数字串；
  断言处 `me, _ := user.Current()` 一律判空＋Getuid 回退。
- 排坑副产：笼内 HOME=/work 会把 `.gocache`、`.config/go/telemetry` 落进
  挂载目录——复现跑后记得清，防误提交。

## 引用
票0034-p1；commit 76fba94；exec.go selfIDs/sameUser；
deploy_test.go TestExec_NumericRunAs（变异验证：删数字直比→红）。
