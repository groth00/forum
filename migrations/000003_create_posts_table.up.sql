CREATE TABLE IF NOT EXISTS posts (
  id serial PRIMARY KEY,
  topic_id int REFERENCES topics(id) NOT NULL,
  user_id int REFERENCES users(id) NOT NULL,
  username text NOT NULL,
  likes int DEFAULT 1,
  created timestamp(0) with time zone default now(),
  last_updated timestamp(0) with time zone default now(),
  title varchar(256) NOT NULL,
  content text NOT NULL,
  num_comments int DEFAULT 0
);

CREATE TABLE IF NOT EXISTS posts_liked (
  id serial PRIMARY KEY,
  post_id int REFERENCES posts(id),
  user_id int REFERENCES users(id),
  score int,
  created timestamp(0) with time zone default now()
);

CREATE TABLE IF NOT EXISTS posts_saved (
  id serial PRIMARY KEY,
  post_id int REFERENCES posts(id),
  user_id int REFERENCES users(id),
  created timestamp(0) with time zone default now()
);
