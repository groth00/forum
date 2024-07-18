CREATE TABLE IF NOT EXISTS topics (
  id serial PRIMARY KEY,
  topic_name varchar(64) NOT NULL,
  created timestamp(0) with time zone default now(),
  num_subscribers int DEFAULT 0,
  num_posts int DEFAULT 0
);

INSERT INTO topics(topic_name) VALUES
  ('Technology'),
  ('News'),
  ('Gaming'),
  ('Fashion'),
  ('Food'),
  ('Travel');

CREATE TABLE IF NOT EXISTS topic_moderators (
  id serial PRIMARY KEY,
  topic_id int REFERENCES topics(id),
  user_id int REFERENCES users(id),
  username text NOT NULL,
  created timestamp(0) with time zone default now(),
  UNIQUE(topic_id, user_id)
);

INSERT INTO topic_moderators(topic_id, user_id, username) VALUES
  (1, 1, 'admin'),
  (2, 1, 'admin'),
  (3, 1, 'admin'),
  (4, 1, 'admin'),
  (5, 1, 'admin'),
  (6, 1, 'admin');
 
CREATE TABLE IF NOT EXISTS topic_subscription (
  id serial PRIMARY KEY,
  topic_id int REFERENCES topics(id) NOT NULL,
  user_id int REFERENCES users(id) NOT NULL,
  created timestamp(0) with time zone default now(),
  UNIQUE(topic_id, user_id)
);
