package db

// schema 建全九表（总设计 §3.2）：ticket/ticket_event/seat/seat_run/shell/
// project/ceo_memory/budget/secret。proto 二进制存 BLOB，需查询字段另立列。
const schema = `
CREATE TABLE IF NOT EXISTS ticket (
  id            TEXT PRIMARY KEY,
  title         TEXT NOT NULL DEFAULT '',
  phase         TEXT NOT NULL,
  spec          BLOB,
  project       TEXT NOT NULL DEFAULT '',
  dedupe_key    TEXT NOT NULL DEFAULT '',
  budget_frozen INTEGER NOT NULL DEFAULT 0,
  reject_count  INTEGER NOT NULL DEFAULT 0,
  defer_count   INTEGER NOT NULL DEFAULT 0,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS ticket_event (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  ticket_id   TEXT NOT NULL,
  kind        TEXT NOT NULL,
  from_phase  TEXT NOT NULL DEFAULT '',
  to_phase    TEXT NOT NULL DEFAULT '',
  actor       TEXT NOT NULL DEFAULT '',
  note        TEXT NOT NULL DEFAULT '',
  provenance  TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ticket_event_ticket ON ticket_event(ticket_id, id);

CREATE TABLE IF NOT EXISTS seat (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  role       TEXT NOT NULL,
  shell      TEXT NOT NULL DEFAULT '',
  status     TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS seat_run (
  run_id        TEXT PRIMARY KEY,
  seat_id       INTEGER NOT NULL DEFAULT 0,
  ticket_id     TEXT NOT NULL,
  role          TEXT NOT NULL,
  shell         TEXT NOT NULL DEFAULT '',
  pid           INTEGER NOT NULL DEFAULT 0,
  run_token     TEXT NOT NULL UNIQUE,
  socket_path   TEXT NOT NULL,
  log_path      TEXT NOT NULL DEFAULT '',
  started_at    INTEGER NOT NULL,
  attached_at   INTEGER NOT NULL DEFAULT 0,
  adopted_at    INTEGER NOT NULL DEFAULT 0,
  exited_at     INTEGER NOT NULL DEFAULT 0,
  exit_code     INTEGER NOT NULL DEFAULT -1,
  stream_closed INTEGER NOT NULL DEFAULT 0,
  dead          INTEGER NOT NULL DEFAULT 0,
  adjunct       INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_seat_run_ticket ON seat_run(ticket_id);
CREATE INDEX IF NOT EXISTS idx_seat_run_token ON seat_run(run_token);

CREATE TABLE IF NOT EXISTS shell (
  name      TEXT NOT NULL,
  model     TEXT NOT NULL DEFAULT '',
  role      TEXT NOT NULL DEFAULT '',
  priority  INTEGER NOT NULL DEFAULT 0,
  health    TEXT NOT NULL DEFAULT 'unknown',
  PRIMARY KEY (name, model, role)
);

CREATE TABLE IF NOT EXISTS project (
  id                        TEXT PRIMARY KEY,
  repo_url                  TEXT NOT NULL DEFAULT '',
  default_branch            TEXT NOT NULL DEFAULT '',
  check_cmd                 TEXT NOT NULL DEFAULT '',
  deploy_cmd                TEXT NOT NULL DEFAULT '',
  probe_cmd                 TEXT NOT NULL DEFAULT '',
  rollback_cmd              TEXT NOT NULL DEFAULT '',
  discipline_docs           TEXT NOT NULL DEFAULT '',
  budget                    INTEGER NOT NULL DEFAULT 0,
  sandbox_spec              TEXT NOT NULL DEFAULT '',
  auto_ticket_enabled       INTEGER NOT NULL DEFAULT 0,
  auto_ticket_budget        INTEGER NOT NULL DEFAULT 0,
  dedupe_rule               TEXT NOT NULL DEFAULT '',
  auto_deploy_requires_ceo  INTEGER NOT NULL DEFAULT 0,
  confirm_rulings           TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS ceo_memory (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  kind        TEXT NOT NULL,
  title       TEXT NOT NULL DEFAULT '',
  content     BLOB,
  event_ref   TEXT NOT NULL DEFAULT '',
  provenance  TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS budget (
  ticket_id  TEXT PRIMARY KEY,
  cap        INTEGER NOT NULL DEFAULT 0,
  used       INTEGER NOT NULL DEFAULT 0,
  hard_cap   INTEGER NOT NULL DEFAULT 0,
  used_total INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS secret (
  name       TEXT PRIMARY KEY,
  value      BLOB NOT NULL,
  role_acl   TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  rotated_at INTEGER NOT NULL DEFAULT 0
);
`
