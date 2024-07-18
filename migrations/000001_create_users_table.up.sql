CREATE TABLE IF NOT EXISTS users (
  id serial PRIMARY KEY,
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  name text UNIQUE NOT NULL,
  email text UNIQUE NOT NULL,
  password_hash bytea NOT NULL,
  activated bool NOT NULL DEFAULT false,
  admin bool NOT NULL DEFAULT false,
  version integer NOT NULL DEFAULT 1
);

INSERT INTO users(name, email, password_hash, activated, admin) VALUES
  ('admin', 'admin@example.com', 'passwordtodo', true, true);
