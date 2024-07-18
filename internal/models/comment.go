package models

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/emirpasic/gods/stacks/arraystack"
	"github.com/lib/pq"
)

type Comment struct {
	ID          int
	UserID      int
	Username    string
	PostID      int
	Likes       int
	Created     time.Time
	LastUpdated time.Time
	Content     string
}

type CommentNode struct {
	ID           int
	PostID       int
	UserID       int
	Username     string
	Likes        int
	Created      time.Time
	LastUpdated  time.Time
	Content      string
	PathLength   int
	Ancestor     int
	Descendant   int
	Breadcrumbs  string
	CommentNodes []*CommentNode
}

type CommentModel struct {
	DB *sql.DB
}

func (m *CommentModel) Get(comment_id int) (*Comment, error) {
	query := "SELECT user_id, post_id, likes, created, last_updated, content FROM comments WHERE id = $1"

	comment := &Comment{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, comment_id).Scan(
		&comment.UserID,
		&comment.PostID,
		&comment.Likes,
		&comment.Created,
		&comment.LastUpdated,
		&comment.Content,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRecordFound
		}
		return nil, err
	}

	return comment, nil
}

func (m *CommentModel) GetForPost(post_id int) ([]*CommentNode, error) {
	tlc := `
    SELECT c.id
    FROM comments AS c
    WHERE c.post_id = $1 AND NOT EXISTS (
      SELECT 1
      FROM comments_paths AS p
      WHERE p.descendant = c.id AND p.path_length > 0
    )
  `
	all_comments := `
    SELECT
      c.id, c.post_id, c.user_id, c.username, c.likes, c.created, c.last_updated, c.content,
      p.path_length, p.ancestor, p.descendant, string_agg(crumbs.ancestor::varchar, ',') AS breadcrumbs
    FROM comments AS c
    JOIN comments_paths AS p ON c.id = p.descendant
    JOIN comments_paths AS crumbs ON crumbs.descendant = p.descendant
    WHERE p.ancestor = ANY($1)
    GROUP BY c.id, p.path_length, p.ancestor, p.descendant
    ORDER BY breadcrumbs;
  `

	tlc_ids := []int{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, tlc, post_id)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		tlc_ids = append(tlc_ids, id)
	}

	// check for no comments on the post
	if len(tlc_ids) == 0 {
		return nil, ErrNoCommentsForPost
	}

	rows, err = tx.QueryContext(ctx, all_comments, pq.Array(tlc_ids))
	if err != nil {
		return nil, err
	}

	topLevelComments, err := deserialize(rows)
	if err != nil {
		return nil, err
	}

	return topLevelComments, nil
}

func (m *CommentModel) GetForUser(user_id int) ([]*Comment, error) {
	query := `
    SELECT c.id, c.post_id, c.likes, c.created, c.last_updated, c.content
    FROM comments AS c
    WHERE c.id = $1
  `

	comments := []*Comment{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, user_id)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		row := &Comment{}
		if err := rows.Scan(
			&row.ID,
			&row.UserID,
			&row.PostID,
			&row.Likes,
			&row.Created,
			&row.LastUpdated,
			&row.Content); err != nil {
			return nil, err
		}
		comments = append(comments, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return comments, nil
}

func (m *CommentModel) Insert(user_id, post_id, parent_id int, username, content string) (int, error) {
	insert_comment := "INSERT INTO comments(user_id, username, post_id, content) VALUES($1, $2, $3, $4) RETURNING id"
	insert_path := `
    INSERT INTO comments_paths(ancestor, descendant, path_length)
      SELECT p.ancestor, $1::int, p.path_length + 1
      FROM comments_paths AS p
      WHERE p.descendant = $2::int
      UNION ALL
      SELECT $1::int, $1::int, 0
  `
	like := "INSERT INTO comments_liked(comment_id, user_id, score) VALUES($1, $2, 1)"
	increment := "UPDATE posts SET num_comments = num_comments + 1 WHERE id = $1"

	var comment_id int

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return -1, err
	}

	err = tx.QueryRowContext(ctx, insert_comment, user_id, username, post_id, content).Scan(&comment_id)
	if err != nil {
		return -1, err
	}

	_, err = tx.ExecContext(ctx, insert_path, comment_id, parent_id)
	if err != nil {
		var pqerror *pq.Error
		if errors.As(err, &pqerror) {
			log.Println(pqerror.Code, pqerror.Error())
		}
		return -1, err
	}

	// user likes their own comment
	_, err = tx.ExecContext(ctx, like, comment_id, user_id)
	if err != nil {
		return -1, err
	}

	_, err = tx.ExecContext(ctx, increment, post_id)
	if err != nil {
		return -1, err
	}

	err = tx.Commit()
	if err != nil {
		return -1, err
	}

	return comment_id, nil
}

func (m *CommentModel) Delete(comment_id int) error {
	post := "SELECT post_id FROM comments WHERE id = $1"
	remove_path := `
    DELETE t1 FROM comments_paths AS t1
    JOIN comments_paths AS t2 ON t1.descendant = t2.descendant
    WHERE t2.ancestor = $1
  `
	decrement := "UPDATE posts SET num_comments = num_comments - 1 WHERE id = $1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	var post_id int
	if err := tx.QueryRowContext(ctx, post, comment_id).Scan(&post_id); errors.Is(err, sql.ErrNoRows) {
		return ErrNoRecordFound
	} else if err != nil {
		return err
	} else {
		_, err := tx.ExecContext(ctx, remove_path, comment_id)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, decrement, post_id)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *CommentModel) Update(comment *Comment) error {
	query := "UPDATE comments SET content = $1 WHERE id = $2"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, comment.Content, comment.ID)
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

func (m *CommentModel) Like(user_id, comment_id int) error {
	exists := "SELECT score FROM comments_liked WHERE user_id = $1 AND comment_id = $2"
	like := "INSERT INTO comments_liked(user_id, comment_id, score) VALUES($1, $2, 1)"
	increment_on_insert := "UPDATE comments SET likes = likes + 1 WHERE id = $1"

	// from dislike to like
	update := "UPDATE comments_liked SET score = 1 WHERE user_id = $1 AND comment_id = $2"
	increment_on_update := "UPDATE comments SET likes = likes + 2 WHERE id = $1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	var score int
	err = tx.QueryRowContext(ctx, exists, user_id, comment_id).Scan(&score)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := tx.ExecContext(ctx, like, user_id, comment_id); err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, increment_on_insert, comment_id); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// user has currently disliked the comment
	if score == -1 {
		if _, err := tx.ExecContext(ctx, update, user_id, comment_id); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, increment_on_update, comment_id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (m *CommentModel) Dislike(user_id, comment_id int) error {
	exists := "SELECT score FROM comments_liked WHERE user_id = $1 AND comment_id = $2"
	dislike := "INSERT INTO comments_liked(user_id, comment_id, score) VALUES($1, $2, -1)"
	decrement_on_insert := "UPDATE comments SET likes = likes - 1 WHERE id = $1"

	// from like to dislike
	update := "UPDATE comments_liked SET score = -1 WHERE user_id = $1 AND comment_id = $2"
	decrement_on_dislike := "UPDATE comments SET likes = likes - 2 WHERE id = $1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	var score int
	err = tx.QueryRowContext(ctx, exists, user_id, comment_id).Scan(&score)
	if err != nil {
		// user has not liked or disliked the comment before
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := tx.ExecContext(ctx, dislike, user_id, comment_id); err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, decrement_on_insert, comment_id); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// user has currently liked the comment
	if score == 1 {
		if _, err := tx.ExecContext(ctx, update, user_id, comment_id); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, decrement_on_dislike, comment_id); err != nil {
			return err
		}
	}

	// if the user has currently disliked the comment, no action needed
	return tx.Commit()
}

func (m *CommentModel) Save(user_id, comment_id int) error {
	exists := "SELECT EXISTS(SELECT true FROM comments_saved WHERE user_id = $1 AND comment_id = $2)"
	insert := "INSERT INTO comments_saved(user_id, comment_id) VALUES($1, $2)"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	var already_saved bool
	if err = tx.QueryRowContext(ctx, exists, user_id, comment_id).Scan(&already_saved); err != nil {
		return err
	}

	if !already_saved {
		if _, err := tx.ExecContext(ctx, insert, user_id, comment_id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (m *CommentModel) Unsave(user_id, comment_id int) error {
	exists := "SELECT EXISTS(SELECT true FROM comments_saved WHERE user_id = $1 AND comment_id = $2)"
	remove := "DELETE FROM comments_saved WHERE user_id = $1 AND comment_id = $2"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	var already_saved bool
	if err := tx.QueryRowContext(ctx, exists, user_id, comment_id).Scan(&already_saved); err != nil {
		return err
	}

	if already_saved {
		if _, err := tx.ExecContext(ctx, remove, user_id, comment_id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func deserialize(rows *sql.Rows) ([]*CommentNode, error) {
	topLevelComments := []*CommentNode{}
	stack := arraystack.New()
	var currentPathLength int

	for rows.Next() {
		row := &CommentNode{}
		if err := rows.Scan(
			&row.ID,
			&row.PostID,
			&row.UserID,
			&row.Username,
			&row.Likes,
			&row.Created,
			&row.LastUpdated,
			&row.Content,
			&row.PathLength,
			&row.Ancestor,
			&row.Descendant,
			&row.Breadcrumbs,
		); err != nil {
			log.Fatal(err)
		}

		// pathLength of 0 means it is a root comment
		if row.PathLength == 0 {
			currentPathLength = 0
			topLevelComments = append(topLevelComments, row)

			// the SQL query returns a root then all of its children, then another root and its children, etc.
			// clear the stack upon reaching a new root
			stack.Clear()
			stack.Push(row)

			// new sibling after sibling
		} else if row.PathLength == currentPathLength {
			// pop the current node, which is the sibling
			temp, _ := stack.Pop()
			parent := temp.(*CommentNode)
			parent.CommentNodes = append(parent.CommentNodes, row)
			stack.Push(row)
			// children
		} else if row.PathLength > currentPathLength {
			currentPathLength += 1
			temp, _ := stack.Peek()
			current := temp.(*CommentNode)
			current.CommentNodes = append(current.CommentNodes, row)
			stack.Push(row)
			// new parent after child
		} else if row.PathLength < currentPathLength {
			currentPathLength -= 1
			// pop the sibling's child and the sibling
			_, _ = stack.Pop()
			_, _ = stack.Pop()
			temp, _ := stack.Peek()
			ancestor := temp.(*CommentNode)
			ancestor.CommentNodes = append(ancestor.CommentNodes, row)
			stack.Push(row)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return topLevelComments, nil
}
