package models

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Post struct {
	ID          int
	TopicID     int
	UserID      int
	Username    string
	Likes       int
	Created     time.Time
	LastUpdated time.Time
	Title       string
	Content     string
	NumComments int
}

type PostModel struct {
	DB *sql.DB
}

func (m *PostModel) Get(post_id int) (*Post, error) {
	query := `
    SELECT p.id, u.name, p.likes, p.created, p.last_updated, p.title, p.content 
    FROM posts AS p JOIN users AS u ON p.user_id = u.id
    WHERE p.id = $1
  `

	post := &Post{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, post_id).Scan(
		&post.ID,
		&post.Username,
		&post.Likes,
		&post.Created,
		&post.LastUpdated,
		&post.Title,
		&post.Content,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRecordFound
		} else {
			return nil, err
		}
	}
	return post, nil
}

func (m *PostModel) GetByTopic(topic_id int) ([]*Post, error) {
	query := `
    SELECT p.id, p.user_id, p.username, p.likes, p.created, p.last_updated, p.title, p.num_comments
    FROM posts AS p
    WHERE p.topic_id = $1
  `

	var posts []*Post

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, topic_id)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		post := &Post{}
		if err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.Username,
			&post.Likes,
			&post.Created,
			&post.LastUpdated,
			&post.Title,
			&post.NumComments,
		); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func (m *PostModel) List(limit int) ([]*Post, error) {
	query := `
    SELECT p.user_id, p.likes, p.created, p.last_updated, p.title, p.content, u.name
    FROM (
      SELECT user_id, likes, created, last_updated, title, content, name
      FROM posts 
      LIMIT $1
    ) AS p JOIN users AS u ON p.user_id = u.id
  `
	limit = max(10, limit)

	posts := []*Post{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		row := &Post{}
		if err := rows.Scan(
			&row.UserID,
			&row.Likes,
			&row.Created,
			&row.LastUpdated,
			&row.Title,
			&row.Content,
			&row.Username); err != nil {
			return nil, err
		}
		posts = append(posts, row)
	}

	return posts, nil
}

func (m *PostModel) Insert(user_id, topic_id int, username, title, content string) (int, error) {
	query := "INSERT INTO posts(user_id, topic_id, username, title, content) VALUES($1, $2, $3, $4, $5) RETURNING id"
	increment := "UPDATE topics SET num_posts = num_posts + 1 WHERE id = $1"
	like := "INSERT INTO posts_liked(post_id, user_id, score) VALUES($1, $2, 1)"

	var id int

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return -1, err
	}

	err = tx.QueryRowContext(ctx, query, user_id, topic_id, username, title, content).Scan(&id)
	if err != nil {
		return -1, err
	}

	_, err = tx.ExecContext(ctx, increment, topic_id)
	if err != nil {
		return -1, err
	}

	// user likes their own post
	_, err = tx.ExecContext(ctx, like, id, user_id)
	if err != nil {
		return -1, err
	}

	err = tx.Commit()
	if err != nil {
		return -1, err
	}

	return id, nil
}

func (m *PostModel) Delete(user_id, post_id, topic_id int) error {
	unliked := "DELETE FROM posts_liked WHERE user_id = $1 AND post_id = $2"
	decrement := "UPDATE topics SET num_posts = num_posts - 1 WHERE topic_id = $1"
	remove := "DELETE FROM posts WHERE post_id = $1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, unliked, user_id, post_id)
	if err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); rowsAffected == 0 {
		return ErrNoRecordFound
	} else if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, decrement, topic_id)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, remove, post_id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (m *PostModel) Update(post *Post) error {
	query := "UPDATE posts SET title = $1, content = $2 WHERE post_id = $3 RETURNING id"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, post.Title, post.Content, post.ID)
	if err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); rowsAffected == 0 {
		return ErrNoRecordFound
	} else if err != nil {
		return err
	}

	return nil
}

func (m *PostModel) Like(user_id, post_id int) error {
	exists := "SELECT score FROM posts_liked WHERE user_id = $1 AND post_id = $2"
	insert := "INSERT INTO posts_liked(user_id, post_id, score) VALUES($1, $2, 1)"
	increment_on_insert := "UPDATE posts SET likes = likes + 1 WHERE id = $1"

	// from dislike to like
	update := "UPDATE posts_liked SET score = 1 WHERE user_id = $1 AND post_id = $2"
	increment_on_like := "UPDATE posts SET likes = likes + 2 WHERE id = $1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	var score int
	err = tx.QueryRowContext(ctx, exists, user_id, post_id).Scan(&score)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := tx.ExecContext(ctx, insert, user_id, post_id); err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, increment_on_insert, post_id); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if score == -1 {
		if _, err := tx.ExecContext(ctx, update, user_id, post_id); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, increment_on_like, post_id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (m *PostModel) Dislike(user_id, post_id int) error {
	exists := "SELECT score FROM posts_liked WHERE user_id = $1 AND post_id = $2"
	insert := "INSERT INTO posts_liked(user_id, post_id, score) VALUES($1, $2, -1)"
	decrement_on_insert := "UPDATE posts SET likes = likes - 1 WHERE id = $1"

	update := "UPDATE posts_liked SET score = -1 WHERE user_id = $1 AND post_id = $2"
	decrement_on_dislike := "UPDATE posts SET likes = likes - 2 WHERE id = $1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	var score int
	err = tx.QueryRowContext(ctx, exists, user_id, post_id).Scan(&score)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if _, err = tx.ExecContext(ctx, insert, user_id, post_id); err != nil {
				return err
			}

			if _, err = tx.ExecContext(ctx, decrement_on_insert, post_id); err != nil {
				return err
			}
		}
	}

	if score == 1 {
		if _, err := tx.ExecContext(ctx, update, user_id, post_id); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, decrement_on_dislike, post_id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (m *PostModel) Save(user_id, post_id int) error {
	exists := "SELECT EXISTS(SELECT true FROM posts_saved WHERE user_id = $1 AND post_id = $2)"
	insert := "INSERT INTO posts_saved(user_id, post_id) VALUES($1, $2)"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	var already_saved bool
	if err := tx.QueryRowContext(ctx, exists, user_id, post_id).Scan(&already_saved); err != nil {
		return err
	}

	if !already_saved {
		if _, err := tx.ExecContext(ctx, insert, user_id, post_id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (m *PostModel) Unsave(user_id, post_id int) error {
	exists := "SELECT EXISTS(SELECT true FROM posts_saved WHERE user_id = $1 AND post_id = $2)"
	remove := "DELETE FROM posts_saved WHERE user_id = $1 AND post_id = $2"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	var already_saved bool
	if err := tx.QueryRowContext(ctx, exists, user_id, post_id).Scan(&already_saved); err != nil {
		return err
	}

	if already_saved {
		if _, err := tx.ExecContext(ctx, remove, user_id, post_id); err != nil {
			return err
		}
	}

	return tx.Commit()
}
