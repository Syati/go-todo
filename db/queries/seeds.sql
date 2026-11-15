-- name: LockSeeds :exec
SELECT pg_advisory_xact_lock(78390215);

-- name: FindSeedAdmin :one
SELECT id FROM admins WHERE email=$1;

-- name: CreateSeedAdmin :exec
INSERT INTO admins(email,password_hash,role,deleted_at)
VALUES(sqlc.arg(email),sqlc.arg(password_hash),sqlc.arg(role),
       CASE WHEN sqlc.arg(deleted)::boolean THEN now() ELSE NULL END)
ON CONFLICT(email) DO NOTHING;

-- name: FindSeedProduct :one
SELECT id FROM catalog_products WHERE name=$1 ORDER BY id LIMIT 1;

-- name: CreateSeedSKU :exec
INSERT INTO catalog_skus(product_id,code,size,color,price_yen,stock_quantity)
VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(code) DO NOTHING;
