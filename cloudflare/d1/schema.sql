-- Worker persistence head state
CREATE TABLE IF NOT EXISTS world_heads (
  world_id TEXT PRIMARY KEY,
  last_path TEXT NOT NULL,
  last_status INTEGER NOT NULL,
  request_count INTEGER NOT NULL DEFAULT 0,
  last_request_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_world_heads_updated_at
  ON world_heads(updated_at DESC);

-- Cloud index backend (replaces local sqlite indexdb in Cloudflare runtime)
CREATE TABLE IF NOT EXISTS index_catalogs (
  world_id TEXT NOT NULL,
  name TEXT NOT NULL,
  digest TEXT NOT NULL,
  json TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (world_id, name)
);

CREATE TABLE IF NOT EXISTS index_ticks (
  world_id TEXT NOT NULL,
  tick INTEGER NOT NULL,
  digest TEXT NOT NULL,
  joins_json TEXT NOT NULL,
  leaves_json TEXT NOT NULL,
  actions_json TEXT NOT NULL,
  raw_json TEXT NOT NULL,
  joins_count INTEGER NOT NULL DEFAULT 0,
  leaves_count INTEGER NOT NULL DEFAULT 0,
  actions_count INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (world_id, tick)
);

CREATE TABLE IF NOT EXISTS index_audits (
  world_id TEXT NOT NULL,
  tick INTEGER NOT NULL,
  seq INTEGER NOT NULL,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  x INTEGER NOT NULL,
  y INTEGER NOT NULL,
  z INTEGER NOT NULL,
  from_block INTEGER NOT NULL,
  to_block INTEGER NOT NULL,
  reason TEXT,
  raw_json TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (world_id, tick, seq)
);

CREATE INDEX IF NOT EXISTS idx_index_audits_actor_tick
  ON index_audits(world_id, actor, tick);

CREATE INDEX IF NOT EXISTS idx_index_audits_pos_tick
  ON index_audits(world_id, x, z, y, tick);

CREATE TABLE IF NOT EXISTS index_snapshots (
  world_id TEXT NOT NULL,
  tick INTEGER NOT NULL,
  path TEXT NOT NULL,
  seed INTEGER NOT NULL,
  height INTEGER NOT NULL,
  chunks INTEGER NOT NULL,
  agents INTEGER NOT NULL,
  claims INTEGER NOT NULL,
  containers INTEGER NOT NULL,
  contracts INTEGER NOT NULL,
  laws INTEGER NOT NULL,
  orgs INTEGER NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (world_id, tick)
);

CREATE TABLE IF NOT EXISTS index_snapshot_world (
  world_id TEXT NOT NULL,
  tick INTEGER NOT NULL,
  weather TEXT NOT NULL,
  weather_until_tick INTEGER NOT NULL,
  active_event_id TEXT NOT NULL,
  active_event_start_tick INTEGER NOT NULL,
  active_event_ends_tick INTEGER NOT NULL,
  active_event_center_x INTEGER NOT NULL,
  active_event_center_y INTEGER NOT NULL,
  active_event_center_z INTEGER NOT NULL,
  active_event_radius INTEGER NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (world_id, tick)
);

CREATE TABLE IF NOT EXISTS index_snapshot_agents (
  world_id TEXT NOT NULL,
  tick INTEGER NOT NULL,
  agent_id TEXT NOT NULL,
  name TEXT NOT NULL,
  org_id TEXT,
  x INTEGER NOT NULL,
  y INTEGER NOT NULL,
  z INTEGER NOT NULL,
  yaw INTEGER NOT NULL,
  hp INTEGER NOT NULL,
  hunger INTEGER NOT NULL,
  stamina_milli INTEGER NOT NULL,
  rep_trade INTEGER NOT NULL,
  rep_build INTEGER NOT NULL,
  rep_social INTEGER NOT NULL,
  rep_law INTEGER NOT NULL,
  fun_novelty INTEGER NOT NULL,
  fun_creation INTEGER NOT NULL,
  fun_social INTEGER NOT NULL,
  fun_influence INTEGER NOT NULL,
  fun_narrative INTEGER NOT NULL,
  fun_risk_rescue INTEGER NOT NULL,
  inventory_json TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (world_id, tick, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_index_snapshot_agents_agent_tick
  ON index_snapshot_agents(world_id, agent_id, tick);

CREATE TABLE IF NOT EXISTS index_snapshot_boards (
  world_id TEXT NOT NULL,
  tick INTEGER NOT NULL,
  board_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  x INTEGER,
  y INTEGER,
  z INTEGER,
  posts_count INTEGER NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (world_id, tick, board_id)
);

CREATE TABLE IF NOT EXISTS index_snapshot_board_posts (
  world_id TEXT NOT NULL,
  tick INTEGER NOT NULL,
  board_id TEXT NOT NULL,
  post_id TEXT NOT NULL,
  author TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  post_tick INTEGER NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (world_id, tick, board_id, post_id)
);

CREATE INDEX IF NOT EXISTS idx_index_snapshot_board_posts_author_tick
  ON index_snapshot_board_posts(world_id, author, tick);

CREATE INDEX IF NOT EXISTS idx_index_snapshot_board_posts_board_posttick
  ON index_snapshot_board_posts(world_id, board_id, post_tick);

CREATE TABLE IF NOT EXISTS index_snapshot_trades (
  world_id TEXT NOT NULL,
  tick INTEGER NOT NULL,
  trade_id TEXT NOT NULL,
  from_agent TEXT NOT NULL,
  to_agent TEXT NOT NULL,
  created_tick INTEGER NOT NULL,
  offer_json TEXT NOT NULL,
  request_json TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (world_id, tick, trade_id)
);

CREATE INDEX IF NOT EXISTS idx_index_snapshot_trades_from_tick
  ON index_snapshot_trades(world_id, from_agent, tick);

CREATE INDEX IF NOT EXISTS idx_index_snapshot_trades_to_tick
  ON index_snapshot_trades(world_id, to_agent, tick);

CREATE TABLE IF NOT EXISTS index_seasons (
  world_id TEXT NOT NULL,
  season INTEGER NOT NULL,
  end_tick INTEGER NOT NULL,
  seed INTEGER NOT NULL,
  snapshot_path TEXT NOT NULL,
  recorded_at TEXT NOT NULL,
  PRIMARY KEY (world_id, season)
);

CREATE INDEX IF NOT EXISTS idx_index_seasons_end_tick
  ON index_seasons(world_id, end_tick);
