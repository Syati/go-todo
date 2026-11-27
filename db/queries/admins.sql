-- name: LockAdmins :exec
LOCK TABLE admins IN EXCLUSIVE MODE;

-- name: BootstrapAdmin :exec
INSERT INTO admins(email,password_hash,role)
SELECT $1,$2,'super_admin' WHERE NOT EXISTS(SELECT 1 FROM admins);

-- name: FindLoginAdmin :one
SELECT * FROM admins WHERE email=$1 AND deleted_at IS NULL;

-- name: FindAdmin :one
SELECT * FROM admins WHERE id=$1 AND deleted_at IS NULL;

-- name: ListAdmins :many
SELECT * FROM admins
WHERE deleted_at IS NULL AND strpos(lower(email), lower(sqlc.arg(search)::text))>0
ORDER BY CASE WHEN sqlc.arg(ascending)::boolean THEN id END ASC,
         CASE WHEN NOT sqlc.arg(ascending)::boolean THEN id END DESC
LIMIT 31 OFFSET sqlc.arg(page_offset)::int;

-- name: CreateAdmin :one
INSERT INTO admins(email,password_hash) VALUES($1,$2) RETURNING id;

-- name: UpdateAdmin :exec
UPDATE admins SET email=sqlc.arg(email),role=sqlc.arg(role),
password_hash=CASE WHEN sqlc.arg(new_password_hash)::text='' THEN password_hash ELSE sqlc.arg(new_password_hash)::text END
WHERE id=sqlc.arg(id) AND deleted_at IS NULL;

-- name: DiscardAdmin :exec
UPDATE admins SET deleted_at=now() WHERE id=$1;

-- name: LockLoginAdminCredentials :one
SELECT id FROM admins
WHERE id = $1 AND password_hash = $2 AND deleted_at IS NULL
FOR SHARE;
