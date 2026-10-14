-- Migration: create_products
-- Generated for examples/v1/db-blinq (+ handlers / db-linq)
-- Padrão Nitro: UUID v7 (app AutoID) + timestamps + soft delete

-- +goose Up

CREATE TABLE IF NOT EXISTS products (
    id         UUID PRIMARY KEY,
    name       TEXT           NOT NULL,
    price      NUMERIC(12, 2) NOT NULL DEFAULT 0,
    active     BOOLEAN        NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_products_active ON products (active);
CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products (deleted_at) WHERE deleted_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS products;
