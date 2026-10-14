-- name: ListProducts :many
SELECT id, name, price::float8 AS price, active, created_at
FROM products
WHERE ($1::boolean IS NULL OR active = $1)
  AND ($2::float8 IS NULL OR price >= $2)
  AND ($3::text IS NULL OR name ILIKE '%' || $3 || '%')
ORDER BY created_at DESC
LIMIT 50;

-- name: GetProduct :one
SELECT id, name, price::float8 AS price, active, created_at
FROM products
WHERE id = $1;

-- name: CreateProduct :one
INSERT INTO products (name, price, active)
VALUES ($1, $2, $3)
RETURNING id, name, price::float8 AS price, active, created_at;

-- name: UpdateProduct :one
UPDATE products
SET name = $2, price = $3, active = $4
WHERE id = $1
RETURNING id, name, price::float8 AS price, active, created_at;

-- name: DeleteProduct :execrows
DELETE FROM products WHERE id = $1;
