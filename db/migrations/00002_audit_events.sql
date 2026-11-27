-- +goose Up
CREATE TABLE audit_events (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  actor_admin_id BIGINT NOT NULL REFERENCES admins(id) ON DELETE RESTRICT,
  action TEXT NOT NULL CHECK (action IN ('create', 'update', 'delete')),
  target_type TEXT NOT NULL CHECK (target_type IN ('admin', 'product', 'sku')),
  target_id BIGINT NOT NULL CHECK (target_id > 0),
  previous_role TEXT CHECK (previous_role IN ('operator', 'super_admin')),
  new_role TEXT CHECK (new_role IN ('operator', 'super_admin')),
  CHECK (
    (previous_role IS NULL AND new_role IS NULL)
    OR (target_type = 'admin' AND action = 'update' AND previous_role IS NOT NULL
        AND new_role IS NOT NULL AND previous_role <> new_role)
  )
);
CREATE INDEX audit_events_actor_idx ON audit_events(actor_admin_id);
CREATE INDEX audit_events_time_idx ON audit_events(occurred_at, id);

-- +goose StatementBegin
CREATE FUNCTION record_admin_audit_event() RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  actor BIGINT := nullif(current_setting('go_todo.audit_actor_id', true), '')::bigint;
  event_action TEXT := CASE WHEN TG_OP = 'INSERT' THEN 'create' ELSE lower(TG_OP) END;
  event_target_id BIGINT;
  old_role TEXT;
  next_role TEXT;
BEGIN
  -- A transaction may belong to migration, seed, or another CLI operation.
  IF actor IS NULL THEN
    RETURN NULL;
  END IF;

  IF TG_OP = 'DELETE' THEN
    event_target_id := OLD.id;
  ELSE
    event_target_id := NEW.id;
  END IF;

  IF TG_ARGV[0] = 'admin' AND TG_OP = 'UPDATE' THEN
    IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
      event_action := 'delete';
    ELSIF OLD.deleted_at IS NOT NULL THEN
      RETURN NULL;
    ELSIF OLD.role IS DISTINCT FROM NEW.role THEN
      old_role := OLD.role;
      next_role := NEW.role;
    END IF;
  END IF;

  INSERT INTO audit_events(actor_admin_id, action, target_type, target_id, previous_role, new_role)
  VALUES (actor, event_action, TG_ARGV[0], event_target_id, old_role, next_role);
  RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_admins
AFTER INSERT OR UPDATE ON admins
FOR EACH ROW EXECUTE FUNCTION record_admin_audit_event('admin');

CREATE TRIGGER audit_products
AFTER UPDATE OR DELETE ON catalog_products
FOR EACH ROW EXECUTE FUNCTION record_admin_audit_event('product');

CREATE TRIGGER audit_skus
AFTER UPDATE OR DELETE ON catalog_skus
FOR EACH ROW EXECUTE FUNCTION record_admin_audit_event('sku');

-- +goose Down
DROP TRIGGER audit_skus ON catalog_skus;
DROP TRIGGER audit_products ON catalog_products;
DROP TRIGGER audit_admins ON admins;
DROP FUNCTION record_admin_audit_event();
DROP TABLE audit_events;
