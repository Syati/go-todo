-- +goose Up
CREATE TABLE admin_recovery_events (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  admin_id BIGINT NOT NULL REFERENCES admins(id) ON DELETE RESTRICT,
  operator_ref TEXT NOT NULL CHECK (length(operator_ref) BETWEEN 1 AND 200)
);
CREATE INDEX admin_recovery_events_admin_idx ON admin_recovery_events(admin_id);

-- +goose Down
DROP TABLE admin_recovery_events;
