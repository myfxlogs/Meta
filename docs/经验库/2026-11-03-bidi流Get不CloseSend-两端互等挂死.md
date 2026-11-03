# 经验：双向流 Get 后客户端不 CloseSend——两端互等挂死（meta memory 读径）

> 令#E1 一坑一文。发现：CEO 三班接手跑 close-audit.sh 挂死实证——
> 前班交接 commit 误诊为「组权限未激活」，真因是流协议两端互等。

## 症状链

1. `CeoMemory` 是**双向流**（`internal/gateway/ceo.go:681`）：服务端
   `for { stream.Recv() }` 每轮等一个 op，答完 Get 回环等下一 op。
2. 客户端 `memoryGet`（`cmd/meta/main.go:1297`）Send 一个 Get 后
   `for { st.Recv() }` 干等服务端 EOF——服务端却在等客户端下一 op。
   两端互等＝永久挂死：行全打完进程不退。
3. `--put` 径正常是因为 `memoryPut` 末尾有 `st.CloseSend()`
   （main.go:1294）——客户端说「我说完了」，服务端 Recv 拿 EOF 退，
   客户端 Recv 才拿 EOF。**读侧漏写同一行**。
4. 受害面：一切 `meta memory` 读调用——close-audit.sh:12 挂死
   （对账机自证断）、交互 CEO 每次读账泄一个进程＋一条服务端流。

## 教训/核查口径

- 双向流客户端「只发一帧就等收」时，发完必须 `CloseSend()`——
  否则服务端不知道对话结束。核查口径：凡 `st.Recv()` 等 io.EOF
  的客户端循环，向上找有没有 CloseSend；没有＝挂死预定。
- 「命令打印完不退出」≠权限/网络问题——先 `pgrep` 看进程是否
  活着，活着就是协议没走完，别猜环境（前班组权限误诊实证）。
- 服务端亦可防御：Get 是单发单答语义的话，答完主动 `return nil`
  关流，比指望客户端礼仪稳。

## 本坑修法

- 运维面已落（close-audit.sh）：`timeout 60` 截流取全量——行先到
  全量后挂死，截流不丢数据。
- 真修（债 D-91）：`memoryGet` Send 后补 `st.CloseSend()`（一行）；
  或服务端 Get 答毕关流。走票修，禁CEO直改产品码。
- 次生断档（债 D-92）：对账机①②仍按旧 md 格式（`### `头/Content
  内坑档名）扫，DB-TSV 读径（无 Content 列）下两闸近乎空转——
  核验口径需随真源改。
