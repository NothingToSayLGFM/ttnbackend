package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"ttnflow-api/internal/domain"
)

type SubscriptionRepo struct {
	db *pgxpool.Pool
}

func NewSubscriptionRepo(db *pgxpool.Pool) *SubscriptionRepo {
	return &SubscriptionRepo{db: db}
}

func (r *SubscriptionRepo) Create(ctx context.Context, s *domain.Subscription) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO subscriptions (user_id, granted_by, starts_at, ends_at, note)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		s.UserID, s.GrantedBy, s.StartsAt, s.EndsAt, s.Note,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}
	return nil
}

func (r *SubscriptionRepo) FindActiveByUserID(ctx context.Context, userID string) (*domain.Subscription, error) {
	s := &domain.Subscription{}
	now := time.Now()
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, granted_by, starts_at, ends_at, COALESCE(note,''), created_at
		 FROM subscriptions
		 WHERE user_id=$1 AND starts_at <= $2 AND ends_at >= $2
		 ORDER BY ends_at DESC LIMIT 1`,
		userID, now,
	).Scan(&s.ID, &s.UserID, &s.GrantedBy, &s.StartsAt, &s.EndsAt, &s.Note, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *SubscriptionRepo) ListByUserID(ctx context.Context, userID string) ([]*domain.Subscription, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, granted_by, starts_at, ends_at, COALESCE(note,''), created_at
		 FROM subscriptions WHERE user_id=$1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*domain.Subscription
	for rows.Next() {
		s := &domain.Subscription{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.GrantedBy, &s.StartsAt, &s.EndsAt, &s.Note, &s.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, nil
}

func (r *SubscriptionRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM subscriptions WHERE id=$1`, id)
	return err
}
