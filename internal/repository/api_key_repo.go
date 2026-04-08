package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"ttnflow-api/internal/domain"
)

type APIKeyRepo struct {
	db *pgxpool.Pool
}

func NewAPIKeyRepo(db *pgxpool.Pool) *APIKeyRepo {
	return &APIKeyRepo{db: db}
}

func (r *APIKeyRepo) Create(ctx context.Context, userID, label, apiKey string) (*domain.NPAPIKey, error) {
	k := &domain.NPAPIKey{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO np_api_keys (user_id, label, api_key, is_active)
		 VALUES ($1, $2, $3, (SELECT COUNT(*) = 0 FROM np_api_keys WHERE user_id = $1))
		 RETURNING id, user_id, label, api_key, is_active, created_at`,
		userID, label, apiKey,
	).Scan(&k.ID, &k.UserID, &k.Label, &k.APIKey, &k.IsActive, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}
	return k, nil
}

func (r *APIKeyRepo) ListByUserID(ctx context.Context, userID string) ([]*domain.NPAPIKey, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, label, api_key, is_active, created_at
		 FROM np_api_keys WHERE user_id = $1 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*domain.NPAPIKey
	for rows.Next() {
		k := &domain.NPAPIKey{}
		if err := rows.Scan(&k.ID, &k.UserID, &k.Label, &k.APIKey, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (r *APIKeyRepo) FindActiveByUserID(ctx context.Context, userID string) (*domain.NPAPIKey, error) {
	k := &domain.NPAPIKey{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, label, api_key, is_active, created_at
		 FROM np_api_keys WHERE user_id = $1 AND is_active = true LIMIT 1`,
		userID,
	).Scan(&k.ID, &k.UserID, &k.Label, &k.APIKey, &k.IsActive, &k.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (r *APIKeyRepo) Activate(ctx context.Context, id, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `UPDATE np_api_keys SET is_active = false WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE np_api_keys SET is_active = true WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (r *APIKeyRepo) Delete(ctx context.Context, id, userID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM np_api_keys WHERE id = $1 AND user_id = $2`, id, userID,
	)
	return err
}
