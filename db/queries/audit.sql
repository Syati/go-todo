-- name: SetAuditActor :exec
SELECT set_config('go_todo.audit_actor_id', sqlc.arg(actor_admin_id)::bigint::text, true);
