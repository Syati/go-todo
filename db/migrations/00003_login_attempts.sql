-- +goose Up
CREATE TABLE login_attempts (
  identity_hash TEXT PRIMARY KEY,
  attempts INTEGER NOT NULL CHECK (attempts BETWEEN 1 AND 5),
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX login_attempts_expires_at_idx ON login_attempts(expires_at);

-- +goose Down
DROP TABLE login_attempts;
