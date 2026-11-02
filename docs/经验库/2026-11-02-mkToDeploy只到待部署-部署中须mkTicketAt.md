# mkToDeploy 只推到「待部署」——部署中相位 fixture 须 mkTicketAt 直推

- 日期：2026-11-02；坑主票：0044；症状：gateway 测试造「部署中席死」fixture，
  用 `mkToDeploy(t,g,id,"P")` 后起 onRunDead/对账，断言 deploy_failed/
  deploy_done 全不落——票相位停在 待部署，席死分支命中 `case PhaseToDeploy`
  早退（放锁+spawn_needed），waitPhase(待部署) 反而秒过，假绿假象。
- 根因：mkToDeploy 内部 `mkTicketAt(..., db.PhaseToDeploy)`——它是部署支
  **派发入口** fixture（等 CTO claim 走 deploy_start 步行到部署中），
  不是相位快造件。要直达部署中：`mkTicketAt(t,g,id,db.PhaseDeploying)`
  ＋`SetTicketProject`。
- 修法：fixture 命名按相位区分——待部署派发径用 mkToDeploy；相位直推用
  mkTicketAt。0044 抽出 `mkDeploying` helper（t0044_test.go）。
- 另坑：`CreateRun` 不填 RunToken 多行撞 `seat_run.run_token` UNIQUE——
  空串也算重复；fixture 随手给唯一值。
