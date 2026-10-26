-- +goose Up
CREATE TABLE orders (
  id TEXT PRIMARY KEY NOT NULL,
  customer_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  total REAL NOT NULL DEFAULT 0,
  note TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);
CREATE TABLE order_events (
  id TEXT PRIMARY KEY NOT NULL,
  order_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  actor TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);

-- +goose Down
DROP TABLE order_events;
DROP TABLE orders;
