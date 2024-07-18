CREATE TABLE IF NOT EXISTS comments (
  id serial PRIMARY KEY,
  user_id int REFERENCES users(id) ON DELETE CASCADE,
  username text NOT NULL,
  post_id int REFERENCES posts(id) ON DELETE CASCADE,
  likes int DEFAULT 1,
  created timestamp(0) with time zone default now(),
  last_updated timestamp(0) with time zone default now(),
  content text NOT NULL
);

CREATE TABLE IF NOT EXISTS comments_paths (
  ancestor int REFERENCES comments(id),
  descendant int REFERENCES comments(id),
  path_length int NOT NULL,
  PRIMARY KEY(ancestor, descendant)
);

CREATE TABLE IF NOT EXISTS comments_liked (
  id serial PRIMARY KEY,
  comment_id int REFERENCES comments(id) ON DELETE CASCADE,
  user_id int REFERENCES users(id) ON DELETE CASCADE,
  score int DEFAULT 1,
  created timestamp(0) with time zone default now()
);

CREATE TABLE IF NOT EXISTS comments_saved (
  id serial PRIMARY KEY,
  comment_id int REFERENCES comments(id) ON DELETE CASCADE,
  user_id int REFERENCES users(id) ON DELETE CASCADE,
  created timestamp(0) with time zone default now()
);
