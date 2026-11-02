# worktree 票面改动误落主检——edit 路径手滑，git status 即捕

> 2026-11-02 · 票 0038-p1 · devin 施工席

## 坑

施工规定「只在 worktree/独立克隆里改」。实际改 `internal/db/ticket.go`
时，edit 工具填了凭记忆敲的 `/opt/meta/internal/db/ticket.go`——主检
（部署树·master 检出）而非 `~/meta-worktrees/t0038-p1/`。edit 成功、
无声无息：两棵树文件同构，工具不报错。

## 捕法

下一步 `git status` 例行核对发现主检出现未暂存改动 → `git checkout`
还原 → 同改重落在 worktree 路径。损失＝一次 edit 往返。

## 防御

- **edit/write 前用 `git status` 或 `pwd` 锚定当前树**——尤其同仓多
  worktree 时，`/opt/meta`（主检）与 `~/meta-worktrees/<支>` 路径前缀
  相似，肌肉记忆会选错。
- 主检出现任何未提交改动＝红旗：施工支的工作永远不该在那里。
- 本坑属「碰部署树」未遂——令行靠自查，不靠工具拦。

## 同族

`2026-11-02-docs票交付口径在master-支交付不算交.md`（master/支落点
混淆的另一面——那边是「交错了树」，这边是「改错了树」）。
