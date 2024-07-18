package models

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Topic struct {
	ID             int
	Name           string
	CreatedAt      time.Time
	NumSubscribers int
	NumPosts       int
	Moderators     []string
}

type TopicModel struct {
	DB *sql.DB
}

func (t *TopicModel) Get(topic_id int) (*Topic, error) {
	//  TODO: fix: if there are no moderators, this query returns an empty set

	//  SELECT t.id, t.topic_name, t.created, t.num_subscribers, t.num_posts, array_agg(m.username) AS moderators
	//  FROM topics AS t JOIN topic_moderators AS m ON t.id = topic_moderators.topic_id
	//  WHERE t.id = $1
	//  GROUP BY t.id

	metadata := `
    SELECT t.id, t.topic_name, t.created, t.num_subscribers, t.num_posts
    FROM topics AS t
    WHERE t.id = $1
  `

	topic := &Topic{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := t.DB.QueryRowContext(ctx, metadata, topic_id).Scan(
		&topic.ID,
		&topic.Name,
		&topic.CreatedAt,
		&topic.NumSubscribers,
		&topic.NumPosts,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNoRecordFound
		default:
			return nil, err
		}
	}

	return topic, nil
}

func (t *TopicModel) List(limit int) ([]*Topic, error) {
	metadata := `
    SELECT t.id, t.topic_name, t.created, t.num_subscribers, t.num_posts
    FROM topics AS t
    ORDER BY t.id
    LIMIT $1
  `

	limit = max(10, limit)

	var topics []*Topic

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := t.DB.QueryContext(ctx, metadata, limit)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		topic := &Topic{}
		if err := rows.Scan(
			&topic.ID,
			&topic.Name,
			&topic.CreatedAt,
			&topic.NumSubscribers,
			&topic.NumPosts,
		); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	return topics, nil
}

func (t *TopicModel) Insert(name string) (int, error) {
	query := "INSERT INTO topics(name) VALUES($1) RETURNING id"

	var topic_id int

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := t.DB.QueryRowContext(ctx, query, name).Scan(&topic_id)
	if err != nil {
		return -1, err
	}

	return topic_id, nil
}

func (t *TopicModel) Delete(topic_id int) error {
	query := "DELETE FROM topics WHERE topic_id = $1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := t.DB.ExecContext(ctx, query, topic_id)
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

func (t *TopicModel) Update(topic *Topic) error {
	query := "UPDATE topics SET name = $1 WHERE id = $2"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := t.DB.ExecContext(ctx, query, topic.Name, topic.ID)
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

func (t *TopicModel) AddModerator(topic_id, user_id int, username string) error {
	query := "INSERT INTO topic_moderators(topic_id, user_id, username) VALUES($1, $2, $3)"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := t.DB.ExecContext(ctx, query, topic_id, user_id, username)
	if err != nil {
		return err
	}

	return nil
}

func (t *TopicModel) RemoveModerator(topic_id, user_id int) error {
	query := "DELETE FROM topic_moderators WHERE topic_id = $1 AND user_id = $2"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := t.DB.ExecContext(ctx, query, topic_id, user_id)
	if err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); rowsAffected == 0 {
		return ErrNoRecordFound
	} else {
		return err
	}
}

func (t *TopicModel) Subscribe(topic_id, user_id int) error {
	insert := "INSERT INTO topic_subscription(topic_id, user_id) VALUES($1, $2)"
	increment := "UPDATE topics SET num_subscribers = num_subscribers + 1 WHERE id = $1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := t.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, insert, topic_id, user_id); err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, increment, topic_id); err != nil {
		return err
	}

	return tx.Commit()
}

func (t *TopicModel) Unsubscribe(topic_id, user_id int) error {
	remove := "DELETE FROM topic_subscription WHERE topic_id = $1 AND user_id = $2"
	decrement := "UPDATE topics SET num_subscribers = num_subscribers - 1 WHERE id = $1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := t.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, remove, topic_id, user_id); err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, decrement, topic_id); err != nil {
		return err
	}

	return tx.Commit()
}
