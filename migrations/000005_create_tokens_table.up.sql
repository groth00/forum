CREATE TABLE IF NOT EXISTS tokens (
    hash bytea PRIMARY KEY,
    user_id int NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expiration timestamp(0) with time zone NOT NULL,
    scope text NOT NULL
);
