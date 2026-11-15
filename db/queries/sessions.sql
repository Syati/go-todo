-- name: FindSessionAdmin :one
SELECT a.* FROM sessions s JOIN admins a ON a.id=s.admin_id
WHERE s.token_hash=$1 AND s.expires_at>now() AND a.deleted_at IS NULL;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at<=now();

-- name: CreateSession :exec
INSERT INTO sessions(token_hash,admin_id,expires_at)
VALUES($1,$2,now()+interval '12 hours');

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash=$1;

-- name: DeleteAdminSessions :exec
DELETE FROM sessions WHERE admin_id=$1;
