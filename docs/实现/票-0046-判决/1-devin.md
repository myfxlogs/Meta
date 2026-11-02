# 票-0046 判决（配方行真源归一——启动回种覆写消双源漂移＋rollback_cmd 修齐）——审计

支 `t/0046-recipe-resync`｜deliver_ref=`9c7d4a6977c125eed3f3b37bb830f37a4a740583`｜基 master=`16a9374`

## 判决：通过

## 逐锚对账（每条只信 diff 与机器记录）

- **锚1 回种覆写测**：PASS。`internal/gateway/t0046_test.go:77-95` `TestReseedOverwrite`——预置全字段异值行→`New()`→`GetProject` 逐字对 `seedRow`。atif step33 机器记录三锚测全 PASS；**对抗提名实证**＝atif step34 变异记录：回种语义改回「行在即不动」后同一测试 FAIL（`got: RollbackCmd:x ... want: RollbackCmd:cmd-r`），变异源即锚面钦定行。
- **锚2 漂移事件测**：PASS。`t0046_test.go:99-150` 双子案——异值预置→`Events` 命中 `project_recipe_sync` 且 note 含全部八字段名（`repo_url…sandbox_spec`）；同文预置→零事件。atif step33 PASS；step34 stderr 实录 `gateway: project_recipe_sync 项目 P 配方漂移字段: repo_url,default_branch,integrate_cmd,check_cmd,deploy_cmd,probe_cmd,rollback_cmd,sandbox_spec`——emit/stderr 径同步实证。`gateway.go:321-326` `AppendEvent("","project_recipe_sync",…)` 空票径与 `ceo.go:683` ceo_say 同款；`len(drift)>0` 才产事件（零漂零事件防每启噪音）——diff 行实证。
- **锚3 空种子保行测**：PASS。`t0046_test.go:155-168` `TestReseedEmptyKeepsRows`——`cfg.Projects` 空＋预置行→行存且 `RollbackCmd="x"` 原值。atif step33 PASS。
- **锚4 build/vet/test/gofmt**：PASS（注记）。本席交付树复算：`go build ./...` rc=0、`go vet ./...` rc=0、`go test -count=1 ./...`＝**296 passed/16 pkg**（与施工 atif step38 机器记录 296/16 一致）。`gofmt -l internal/ cmd/` 交付树＝16 件——与 master 基线同款输出逐行 diff 比对：**master 17 件含 `internal/gateway/gateway.go`，支内净减 1 件、零增量**（16 件全为存量：adapter/claude.go、caster/caster.go、db/msg.go、db/phase_test.go、db/t0036_test.go、db/ticket.go、feishu 三件、feishu_card.go、inject.go、t_t18_test.go、t_t2_test.go、route/route.go、upgrade/t_p3_test.go、cmd/meta/main.go）——字面零命中不达系 master 存量脏区，修之即范围蔓延，照票-0010/0011/0034/0044 先例判绿记观察。
- **锚5 复演**：PASS。atif step41 机器记录原文：scratch 仓 `merge rc=1` 冲突态→`rev-parse MERGE_HEAD` 得 `1eb354c…`＋`UU f`→实跑 config:40 文面输出 `rollback: 半合并态已弃净`→跑后 `MERGE_HEAD rc=1`（解析失败）＋`UU 数: 0`；clean 仓输出 `rollback: 无 MERGE_HEAD 残留——显式告警，本部署无可回状态`——非「本部署无破坏可回」旧 stub 文。对账档 §6 记录与机器输出逐字一致。本席亲验 `/opt/meta/config/gateway.textproto:40` 文面＝MERGE_HEAD 分径版，与复演所跑同源。
- **锚6 对账档在仓**：PASS。`docs/实现/配方漂移对账-0046.md` §1 五字段对照表（rollback_cmd 独漂、余四零漂、sandbox_spec 串形漂语义同值附记）＋§2 处置记＋§3 真源归一明记段＋§4 同族残险记（subscription/shell 插而不改面、proto 注释残影）＋§5 脏树三连证＋§6 复演录——六节齐，与本席亲验（生产 DB `rollback_cmd` 仍为旧 stub 原文、主仓 `MERGE_HEAD rc=1`＋UU×0）逐条对得上。
- **锚7 部署后自证**：记档项。票面钦定「审计不阻塞」——部署 restart→`New()` 回种即绿，机制已锚 1/2 覆写＋事件径实证；生产库现状（rollback_cmd 旧 stub 滞留、零 `project_recipe_sync` 事件）与本票未部署态自洽，归 CTO-B/巡检取证。

## 范围核

diff 四件：`internal/db/project.go`（SeedProject 废止→ReseedProject＋ProjectDrift）、`internal/gateway/gateway.go`（回种环＋事件）、`internal/gateway/t0046_test.go`（三锚测）、`docs/实现/配方漂移对账-0046.md`——全落票面 §3 钦定域。零 proto/RPC/CLI 面变更（proto/ 零 diff）；config:40 文面未动；生产 DB 仅 SELECT 取证。`gateway.go` 内 FilterSpec/runState/recon 字段/reconSilentKinds 六处对齐整版系 `gofmt -w` 对存量脏文件的就地归一（master 该文件即脏），在触碰文件内、零语义差——不构范围蔓延。

## 偏差/债项（瞒报对账——摘要「下一步」逐项对账）

- gofmt 存量：摘要报「15 件」实测 **16 件**（计数微差，「存量非本票带入」口径经基线 diff 零增量亲核成立——票-0044 同款处理）。
- proto 注释残影：摘要报 `meta.proto:528/617`，实测 `proto/meta/v1/meta.proto:529`（「缺 id 行才落」）＋`:618`（「已存在 id 行不动」）——行号各偏一、残影实存，零 proto 禁线下不修正确，随下个 proto 面票顺带正。
- sandbox_spec 串形漂：atif step43 marshal 对比实证（DB 多行排版 vs prototext 单行紧凑，语义同值）——一次性良性申报属实，首启回种将报一次 `sandbox_spec` 漂移并覆写归一。
- 同族残险（subscription `INSERT OR IGNORE`／shell `UpsertShell` 只回 priority 列）：对账档 §4 记档列下一步，设计 §5/§6 钦定不修——申报属实。
- diff 扫 TODO/FIXME/XXX/HACK/绕过/临时/fmt.Print 零命中；`SeedProject` 全库零残留调用（grep 实证仅 `ReseedProject`/`ProjectDrift` 新名）——无死码、无调试残渣、**瞒报零**。

## 独立复算

交付树（9c7d4a6）：`go build ./...`→BUILD_OK；`go vet ./...`→VET_OK；`go test -count=1 ./...`→**296 passed in 16 packages**；`gofmt -l internal/ cmd/`→16 件（master 基线 17 件、diff 零增量净减 1）；`git diff master...HEAD | grep -cE 'TODO|FIXME|绕过|fmt\.Print'`→0。生产只读取证：`sqlite3 run/meta.db "SELECT rollback_cmd FROM project WHERE id='meta'"`→旧 stub 原文（待部署回种修齐，锚 7 候）；`project_recipe_sync` 事件零行（新码未装，自洽）。atif 施工实录 step21（库值快照）/30（build+vet+fmt 单链）/33（三锚测）/34（变异回红）/38（296/16）/41（复演双径）/43（sandbox 序列化对比）逐项互证一致。

## 质量三问

最优解：`ReseedProject`＝`GetProject`＋`UpsertProject`（既有 ON CONFLICT 全列覆写）＋纯函数 `ProjectDrift`——零新依赖、零逆向依赖（db 不引 gateway）；`SeedProject` 更名重签语义诚实（返回漂移清单），优于在原名上改义。第一性：直击「种子只生效一次」病根，运行时读径（exec.go GetProject）一行未动——漂移消灭在启动边界，非 workaround 摞补丁。洁净：旧函数零残留、无重复实现、无 TODO。说不出更优替代形制——无质量驳回项。

## 可沉淀打法候选

「真源归一三板斧——钦定源文面＋启动投影全列 upsert＋字段级漂移事件（零漂零事件）」——**可沉淀**：形制与对账档 §4 同族残险面（subscription/shell 种子）一一对应，后续票可直接套用到 SeedSubscription/UpsertShell 改种，沉淀建议落 `docs/技能库/`。
