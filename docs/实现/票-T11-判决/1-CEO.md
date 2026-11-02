# T11（外派·立票点火洞复工）判决：驳回——返修一项（CEO 终端形态·决策 94，2026-11-02）

- 交付：支 `t/t11-reconcile-wake` @ `f07d1cb`（deliver_ref 亲核），九件
  +266/−62。独立复算：build/vet RC=0、`go test ./...` **230 绿/16 包**、
  selfcheck RC=0。

## 采信面（主体规整，返修不动）

- 静默集 `reconSilentKinds`：机器记账 kind 不武装——防「滞留票
  spawn_needed 每窗重放＝变相轮询」违 R2；`created`/相位类真事件照常武装，
  点火本意保住。设计成立。
- 闹钟形态＝`time.AfterFunc` 重置式一次性（R2 钦定形）；Close 收口在位；
  `reconSilentKinds` 含 `patrol_tick` 不误伤（tick 本身静默，patrol 票由
  `created` 武装）。
- patrol-* 已立行补巡检角色（D-54 同径漏覆写）——与票面锚③对齐。
- config `reconcile_debounce` 默认 30s、≤0 关——与票面一致。

## 驳回项（唯一）

**`armRecon` 的 `reconBusy` 早退丢事件＝偶现点火洞**：

```
if g.closed || g.reconBusy { return }   // 对账在飞期事件被丢
```

对账运行的瞬间若有新票落地（`created` 到达恰逢 busy），该事件既不续命也
无脏标；若那轮对账扫描又未读到该票（先扫后落），此票掉进无人点火缝——
**必现病修成偶现病，更难抓**。启动对账（网关 New 内 reconcile）窗口最
危险：重启风暴期 patrol/化身立票恰在此窗。票面锚①「裸立票→≤去抖窗+5s
spawn_needed 必发」在该竞态下不成立（测试未覆盖此窗）。

**返修范围（只此一项，其余零改动）**：busy 期到达的非静默事件置
`reconDirty`；对账收尾（busy=false 处）查脏标——脏则重置闹钟再醒一轮。
补一锚：对账在飞期注入 created → 收尾后必再醒一轮（竞态窗闭合实证）。

**结论：驳回。返修后本席复审，只验驳回项。**
