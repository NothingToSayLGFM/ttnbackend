package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"ttnflow-api/internal/domain"
)

type TokenRepo struct {
	db *pgxpool.Pool
}

func NewTokenRepo(db *pgxpool.Pool) *TokenRepo {
	return &TokenRepo{db: db}
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

func (r *TokenRepo) Save(ctx context.Context, userID, token string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, hashToken(token), expiresAt,
	)
	return err
}

func (r *TokenRepo) FindUserID(ctx context.Context, token string) (string, error) {
	var userID string
	var revoked bool
	var expiresAt time.Time
	err := r.db.QueryRow(ctx,
		`SELECT user_id, revoked, expires_at FROM refresh_tokens WHERE token_hash=$1`,
		hashToken(token),
	).Scan(&userID, &revoked, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if revoked || time.Now().After(expiresAt) {
		return "", domain.ErrUnauthorized
	}
	return userID, nil
}

func (r *TokenRepo) Revoke(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked=true WHERE token_hash=$1`,
		hashToken(token),
	)
	return err
}

func (r *TokenRepo) RevokeAll(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked=true WHERE user_id=$1`,
		userID,
	)
	return err
}
