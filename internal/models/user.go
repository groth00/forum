package models

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int
	Name      string
	Email     string
	Password  Password
	Created   time.Time
	Activated bool
	Admin     bool
	Version   int
}

type UserModel struct {
	DB *sql.DB
}

func (m *UserModel) Get(user_id int) (*User, error) {
	query := "SELECT id, name, email, password_hash, created_at, activated, version FROM users WHERE id = $1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user := &User{}
	if err := m.DB.QueryRowContext(ctx, query, user_id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password.Hash,
		&user.Created,
		&user.Activated,
		&user.Version,
	); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoRecordFound
	} else if err != nil {
		return nil, err
	}

	return user, nil
}

func (m *UserModel) GetSavedComments(user_id int) ([]*Comment, error) {
	query := `
    SELECT c.id, c.user_id, c.username, c.post_id, c.likes, c.created, c.last_updated, c.content 
    FROM comments_saved AS s JOIN comments AS c ON s.comment_id = c.id 
    WHERE s.user_id = $1
  `

	comments := []*Comment{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, user_id)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		comment := &Comment{}
		if err := rows.Scan(
			&comment.ID,
			&comment.UserID,
			&comment.Username,
			&comment.PostID,
			&comment.Likes,
			&comment.Created,
			&comment.LastUpdated,
			&comment.Content,
		); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}

	if len(comments) == 0 {
		return nil, ErrNoRecordFound
	}

	return comments, nil
}

func (m *UserModel) GetSavedPosts(user_id int) ([]*Post, error) {
	query := `
    SELECT p.id, p.topic_id, p.user_id, p.username, p.likes, p.created, p.last_updated, p.title, p.content, p.num_comments
    FROM posts_saved AS s JOIN posts AS p ON s.post_id = p.id 
    WHERE s.user_id = $1
  `

	posts := []*Post{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, user_id)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		post := &Post{}
		if err := rows.Scan(
			&post.ID,
			&post.TopicID,
			&post.UserID,
			&post.Username,
			&post.Likes,
			&post.Created,
			&post.LastUpdated,
			&post.Title,
			&post.Content,
			&post.NumComments,
		); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	if len(posts) == 0 {
		return nil, ErrNoRecordFound
	}

	return posts, nil
}

func (m *UserModel) GetLikedComments(user_id int) ([]*Comment, error) {
	query := `
    SELECT c.id, c.user_id, c.username, c.post_id, c.likes, c.created, c.last_updated, c.content 
    FROM comments_liked AS l JOIN comments AS c ON l.comment_id = c.id 
    WHERE l.user_id = $1 AND l.score = 1
  `

	comments := []*Comment{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, user_id)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		comment := &Comment{}
		if err := rows.Scan(
			&comment.ID,
			&comment.UserID,
			&comment.Username,
			&comment.PostID,
			&comment.Likes,
			&comment.Created,
			&comment.LastUpdated,
			&comment.Content,
		); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}

	if len(comments) == 0 {
		return nil, ErrNoRecordFound
	}

	return comments, nil
}

func (m *UserModel) GetLikedPosts(user_id int) ([]*Post, error) {
	query := `
    SELECT p.id, p.topic_id, p.user_id, p.username, p.likes, p.created, p.last_updated, p.title, p.content, p.num_comments
    FROM posts_liked AS l JOIN posts AS p ON l.post_id = p.id 
    WHERE l.user_id = $1 AND l.score = 1
  `

	posts := []*Post{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, user_id)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		post := &Post{}
		if err := rows.Scan(
			&post.ID,
			&post.TopicID,
			&post.UserID,
			&post.Username,
			&post.Likes,
			&post.Created,
			&post.LastUpdated,
			&post.Title,
			&post.Content,
			&post.NumComments,
		); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	if len(posts) == 0 {
		return nil, ErrNoRecordFound
	}

	return posts, nil
}

func (m *UserModel) Insert(name, email, password string) (int, error) {
	query := "INSERT INTO users(name, email, password_hash, activated) VALUES($1, $2, $3, false) RETURNING id"

	hashed_password, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return -1, err
	}

	var user_id int

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = m.DB.QueryRowContext(ctx, query, name, email, hashed_password).Scan(&user_id)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return -1, ErrDuplicateEmail
		case err.Error() == `pq: duplicate key value violates unique constraint "users_name_key"`:
			return -1, ErrDuplicateUsername
		default:
			return -1, err
		}
	}
	return user_id, nil
}

func (m *UserModel) Delete(user_id int) error {
	query := "DELETE FROM users WHERE id = $1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, user_id)
	if err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); rowsAffected == 0 {
		return ErrNoRecordFound
	} else {
		return err
	}
}

func (m *UserModel) List() ([]*User, error) {
	query := "SELECT id, name, email, created FROM users ORDER BY id LIMIT 10"

	users := []*User{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		user := &User{}
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Created); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (m *UserModel) Update(user *User) error {
	query := `
		UPDATE users
		SET name = $1, email = $2, password_hash = $3, activated = $4, version = version + 1
		WHERE id = $5 AND version = $6
		RETURNING version
	`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []interface{}{user.Name, user.Email, user.Password.Hash, user.Activated, user.ID, user.Version}
	result, err := m.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); rowsAffected == 0 {
		return ErrConcurrencyControl
	} else {
		return err
	}
}

func (m *UserModel) Authenticate(email, password string) (int, error) {
	query := "SELECT id, password_hash FROM users WHERE email = $1"

	var id int
	var hashed_password []byte

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, email).Scan(&id, &hashed_password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, ErrInvalidCredentials
		}
		return -1, err
	}

	err = bcrypt.CompareHashAndPassword(hashed_password, []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return -1, ErrInvalidCredentials
		}
		return -1, err
	}

	return id, nil
}

func (m *UserModel) Exists(id int) (bool, error) {
	query := "SELECT EXISTS(SELECT true FROM users WHERE id = $1)"

	var exists bool

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(&exists)
	return exists, err
}

func (m *UserModel) GetByEmail(email string) (*User, error) {
	query := "SELECT id, name, email, password_hash, created_at, activated, version FROM users WHERE email = $1"

	user := &User{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password.Hash,
		&user.Created,
		&user.Activated,
		&user.Version,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRecordFound
		} else {
			return nil, err
		}
	}
	return user, nil
}

func (m *UserModel) GetByToken(token, scope string) (*User, error) {
	hash := sha256.Sum256([]byte(token))

	query := `
    SELECT u.id, u.name, u.email, u.password_hash, u.created_at, u.activated, u.version
    FROM users AS u JOIN tokens AS t on u.id = t.user_id
    WHERE t.hash = $1 AND t.scope = $2 AND t.expiration > $3
  `

	user := &User{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, hash[:], scope, time.Now()).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password.Hash,
		&user.Created,
		&user.Activated,
		&user.Version,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRecordFound
		} else {
			return nil, err
		}
	}
	return user, nil
}

func (m *UserModel) UpdatePassword(user *User) error {
	query := "UPDATE users SET password_hash = $1 WHERE id = $2"

	new_hash, err := bcrypt.GenerateFromPassword([]byte(*user.Password.Plaintext), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	result, err := m.DB.Exec(query, new_hash, user.ID)
	if err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); rowsAffected == 0 {
		return ErrNoRecordFound
	} else {
		return err
	}
}
