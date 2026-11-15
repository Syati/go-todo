-- name: CountProducts :one
SELECT count(*) FROM catalog_products;

-- name: ListProducts :many
SELECT * FROM catalog_products
WHERE strpos(lower(name),lower(sqlc.arg(search)::text))>0
AND (sqlc.arg(status_filter)::text='' OR status=sqlc.arg(status_filter)::text)
ORDER BY CASE WHEN sqlc.arg(ascending)::boolean THEN id END ASC,
         CASE WHEN NOT sqlc.arg(ascending)::boolean THEN id END DESC
LIMIT 31 OFFSET sqlc.arg(page_offset)::int;

-- name: FindProduct :one
SELECT * FROM catalog_products WHERE id=$1;

-- name: CreateProduct :one
INSERT INTO catalog_products(name,description,status,tax_rate) VALUES($1,$2,$3,$4) RETURNING id;

-- name: UpdateProduct :exec
UPDATE catalog_products SET name=$1,description=$2,status=$3,tax_rate=$4,updated_at=now() WHERE id=$5;

-- name: DeleteProduct :exec
DELETE FROM catalog_products WHERE id=$1;

-- name: ListSKUs :many
SELECT * FROM catalog_skus WHERE product_id=$1 ORDER BY id;

-- name: FindSKU :one
SELECT * FROM catalog_skus WHERE id=$1 AND product_id=$2;

-- name: CreateSKU :exec
INSERT INTO catalog_skus(product_id,code,size,color,price_yen,stock_quantity) VALUES($1,$2,$3,$4,$5,$6);

-- name: UpdateSKU :exec
UPDATE catalog_skus SET code=$1,size=$2,color=$3,price_yen=$4,stock_quantity=$5 WHERE id=$6 AND product_id=$7;

-- name: DeleteSKU :exec
DELETE FROM catalog_skus WHERE id=$1 AND product_id=$2;
