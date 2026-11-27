-- name: ReserveLoginAttempt :one
INSERT INTO login_attempts (identity_hash, attempts, expires_at)
VALUES ($1, 1, statement_timestamp() + interval '15 minutes')
ON CONFLICT (identity_hash) DO UPDATE SET
  attempts = CASE WHEN login_attempts.expires_at <= statement_timestamp() THEN 1 ELSE login_attempts.attempts + 1 END,
  expires_at = CASE WHEN login_attempts.expires_at <= statement_timestamp() THEN excluded.expires_at ELSE login_attempts.expires_at END
WHERE login_attempts.expires_at <= statement_timestamp() OR login_attempts.attempts < 5
RETURNING expires_at;

-- name: ResetLoginAttempts :exec
DELETE FROM login_attempts WHERE identity_hash = $1 AND expires_at = $2;

-- name: DeleteExpiredLoginAttempts :exec
DELETE FROM login_attempts WHERE expires_at <= statement_timestamp();
