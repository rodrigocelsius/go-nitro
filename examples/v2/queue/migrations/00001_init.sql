-- +goose Up
CREATE TABLE orders (
  id TEXT PRIMARY KEY NOT NULL,
  customer_email TEXT NOT NULL,
  total REAL NOT NULL,
  status TEXT NOT NULL DEFAULT 'placed',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);

-- +goose Down
DROP TABLE orders;
