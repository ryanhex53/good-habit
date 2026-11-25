-- 0001_init.sql：初始 schema（设计文档 §7 原样）。
-- 时间一律 Unix 毫秒；余额列 users.balance 是账单 SUM 的冗余，由写协程同事务维护。

CREATE TABLE families (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  single_draw_cost INTEGER NOT NULL DEFAULT 5,
  double_draw_cost INTEGER NOT NULL DEFAULT 7,
  created_at INTEGER NOT NULL
);

CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  family_id INTEGER NOT NULL REFERENCES families(id),
  role TEXT NOT NULL CHECK (role IN ('parent','child')),
  username TEXT NOT NULL UNIQUE,          -- 小写存储
  password_hash TEXT NOT NULL,            -- bcrypt
  display_name TEXT NOT NULL,
  balance INTEGER NOT NULL DEFAULT 0,     -- 仅 child；账单为准的冗余列
  active INTEGER NOT NULL DEFAULT 1,      -- 软删除
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_users_family ON users(family_id, role, active);

CREATE TABLE reward_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  family_id INTEGER NOT NULL REFERENCES families(id),
  title TEXT NOT NULL,
  points INTEGER NOT NULL CHECK (points > 0),
  active INTEGER NOT NULL DEFAULT 1,
  sort INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_reward_items_family ON reward_items(family_id, active, sort);

CREATE TABLE prizes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  family_id INTEGER NOT NULL REFERENCES families(id),
  name TEXT NOT NULL,
  emoji TEXT NOT NULL DEFAULT '🎁',
  active INTEGER NOT NULL DEFAULT 1,
  sort INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_prizes_family ON prizes(family_id, active, sort);

CREATE TABLE draws (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uuid TEXT NOT NULL UNIQUE,
  family_id INTEGER NOT NULL REFERENCES families(id),
  child_id INTEGER NOT NULL REFERENCES users(id),
  mode TEXT NOT NULL CHECK (mode IN ('single','double')),
  cost INTEGER NOT NULL,
  prize0_id INTEGER NOT NULL REFERENCES prizes(id),
  prize1_id INTEGER REFERENCES prizes(id),   -- 双抽第二个结果
  picked INTEGER,                            -- 0/1；单抽恒 0
  state TEXT NOT NULL CHECK (state IN ('pending_choice','completed')),
  fulfilled_at INTEGER,                      -- 家长标记发放时间
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_draws_family ON draws(family_id, state, fulfilled_at);
CREATE INDEX idx_draws_child ON draws(child_id, id);

CREATE TABLE ledger (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uuid TEXT NOT NULL UNIQUE,
  family_id INTEGER NOT NULL REFERENCES families(id),
  child_id INTEGER NOT NULL REFERENCES users(id),
  delta INTEGER NOT NULL,                    -- ±
  balance_after INTEGER NOT NULL,            -- 当时余额快照
  kind TEXT NOT NULL CHECK (kind IN ('earn','draw','adjust')),
  ref_reward_item_id INTEGER REFERENCES reward_items(id),
  ref_draw_id INTEGER REFERENCES draws(id),
  note TEXT,                                 -- 手动调整原因/备注
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_ledger_child ON ledger(child_id, id);
CREATE INDEX idx_ledger_family ON ledger(family_id, id);

CREATE TABLE sessions (
  token_hash TEXT PRIMARY KEY,               -- sha256(token) hex
  user_id INTEGER NOT NULL REFERENCES users(id),
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at INTEGER NOT NULL
);
