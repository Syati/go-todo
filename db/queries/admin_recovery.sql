-- name: RecoverSoleSuperAdmin :one
UPDATE admins AS target
SET password_hash = sqlc.arg(password_hash)
WHERE target.email = sqlc.arg(email) AND target.role = 'super_admin' AND target.deleted_at IS NULL
  AND (SELECT count(*) FROM admins WHERE role = 'super_admin' AND deleted_at IS NULL) = 1
RETURNING target.id;

-- name: RecordAdminRecovery :exec
INSERT INTO admin_recovery_events (admin_id, operator_ref) VALUES ($1, $2);

-- name: ResetRecoveredAdminAttempts :exec
DELETE FROM login_attempts WHERE identity_hash = $1;
