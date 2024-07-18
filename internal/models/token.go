package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"time"
)

const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
)

type Token struct {
	Plaintext  string
	Hash       []byte
	UserID     int
	Expiration time.Time
	Scope      string
}

type TokenModel struct {
	DB *sql.DB
}

func generateToken(user_id int, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID:     user_id,
		Expiration: time.Now().Add(ttl),
		Scope:      scope,
	}

	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes) // calls io.ReadFull to write into randomBytes
	if err != nil {
		return nil, err
	}

	plaintext := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes) // len is 52
	hash := sha256.Sum256([]byte(plaintext))

	token.Plaintext = plaintext
	token.Hash = hash[:]
	return token, nil
}

func (m *TokenModel) New(user_id int, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(user_id, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = m.Insert(token)
	return token, err
}

func (m *TokenModel) Insert(token *Token) error {
	query := `
    INSERT INTO tokens(hash, user_id, expiration, scope) 
    VALUES($1, $2, $3, $4)
    ON CONFLICT(hash)
    DO NOTHING
  `

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, token.Hash, token.UserID, token.Expiration, token.Scope)
	return err
}

func (m *TokenModel) DeleteAllForUser(user_id int, scope string) error {
	query := "DELETE FROM tokens WHERE user_id = $1 AND scope = $2"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, user_id, scope)
	if err != nil {
		return err
	}

	if rowsAffected, err := result.RowsAffected(); rowsAffected == 0 {
		return ErrNoRecordFound
	} else {
		return err
	}
}
