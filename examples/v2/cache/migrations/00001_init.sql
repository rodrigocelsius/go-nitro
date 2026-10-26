-- +goose Up
CREATE TABLE drivers (
  id TEXT PRIMARY KEY NOT NULL,
  name TEXT NOT NULL,
  license TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);

-- +goose Down
DROP TABLE drivers;
