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
