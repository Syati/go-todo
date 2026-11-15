-- +goose Up
CREATE TABLE admins (
  id BIGSERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'operator' CHECK (role IN ('super_admin','operator')),
  deleted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE sessions (
  token_hash TEXT PRIMARY KEY,
  admin_id BIGINT NOT NULL REFERENCES admins(id),
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE catalog_products (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
  description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','on_sale','discontinued')),
  tax_rate INTEGER NOT NULL DEFAULT 10 CHECK (tax_rate IN (8,10)),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE catalog_skus (
  id BIGSERIAL PRIMARY KEY,
  product_id BIGINT NOT NULL REFERENCES catalog_products(id) ON DELETE CASCADE,
  code TEXT NOT NULL UNIQUE CHECK (code ~ '^[A-Z0-9][A-Z0-9-]{0,49}$'),
  size TEXT NOT NULL DEFAULT '' CHECK (char_length(size) <= 30),
  color TEXT NOT NULL DEFAULT '' CHECK (char_length(color) <= 30),
  price_yen BIGINT NOT NULL CHECK (price_yen BETWEEN 0 AND 10000000),
  stock_quantity BIGINT NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),
  UNIQUE (product_id, size, color)
);
CREATE INDEX catalog_products_status_idx ON catalog_products(status);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

-- +goose Down
DROP TABLE catalog_skus;
DROP TABLE catalog_products;
DROP TABLE sessions;
DROP TABLE admins;
