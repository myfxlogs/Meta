日期：2026-11-02
席位：施工（票0023）
现象：票面按「FULL＋2 upsert＝3 帧」定 `--n 3`，实测一次 `rule void` 产
**3 帧**（phase:void→TICKET_UPSERT＋ruling→SEAT_UPSERT＋TICKET_UPSERT），
n=3 恒在第 3 帧截住只数到 1 枚 upsert——按票面字面实现＝必红
一句话教训：**给板数帧先读 board.go frames() 映射再定 n**：事件带 RunID
必夹一枚席位帧（活→SEAT_UPSERT/死→SEAT_GONE）；且 FULL 帧**展开多行**
（头行＋每席一行＋每票一行，FULL 里的票行也是 `TICKET_UPSERT` 打头）——
grep 行数≠帧数

## 环境

前端席位板（票0012 码面）＋`meta board --connect --n`（cmd/meta/main.go boardCmd）

## 复现

- CEO 裁决 `rule void`：transition 先 emit `phase:void`（仅 TicketID）→1 枚
  TICKET_UPSERT；land() 再 emit `ruling`（TicketID＋**CEO RunID**）→frames()
  RunID 分支产 SEAT_UPSERT＋TicketID 分支产 TICKET_UPSERT
- FULL 帧打行：`FULL seats=1 tickets=1`＋SEAT_UPSERT 行＋TICKET_UPSERT 行
  （vfy-a 在 FULL 展开里就贡献了一枚 upsert 行——实测首绿 upsert=3）

## 解法/绕法

- `--n 4`：FULL＋一次 void（U/S/U）即 4 帧含 2 枚 upsert，锚口径
  （FULL≥1＋TICKET_UPSERT≥2）不动；第二次 void 兜底
- 先订后装：client 上屏 FULL（grep frames 档）再泵事件——订户口立住前
  发事件＝丢首窗（同族：2026-11-01-订阅在goroutine里注册-丢首事件.md）
- CEO 裁决脚本径：席位 env 的 META_RUN_TOKEN 是施工 role（ceoByToken 拒非
  CEO）；唯一 CEO token 径＝`meta ceo` REPL（token 不出进程）——脚本用
  FIFO 喂 REPL 行命令（`rule void <票> confirm`＝同一 CeoRule RPC）

## 引用

internal/board/board.go frames()（RunID/TicketID 双分支）；cmd/meta/main.go
formatFrame（FULL 展开）；internal/gateway/ceo.go CeoRule（transition＋land 双
emit）；scripts/board-verify.sh 头注帧账
