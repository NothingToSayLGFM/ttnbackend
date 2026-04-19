package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"ttnflow-api/internal/domain"
)

type SessionRepo struct {
	db *pgxpool.Pool
}

func NewSessionRepo(db *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(ctx context.Context, userID, deviceType string) (*domain.Session, error) {
	s := &domain.Session{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO sessions (user_id, device_type) VALUES ($1, $2)
		 RETURNING id, user_id, device_type, started_at, finished_at, ttn_count, status`,
		userID, deviceType,
	).Scan(&s.ID, &s.UserID, &s.DeviceType, &s.StartedAt, &s.FinishedAt, &s.TTNCount, &s.Status)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return s, nil
}

func (r *SessionRepo) FindByID(ctx context.Context, id string) (*domain.Session, error) {
	s := &domain.Session{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, device_type, started_at, finished_at, ttn_count, status FROM sessions WHERE id=$1`, id,
	).Scan(&s.ID, &s.UserID, &s.DeviceType, &s.StartedAt, &s.FinishedAt, &s.TTNCount, &s.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return s, err
}

func (r *SessionRepo) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Session, int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, device_type, started_at, finished_at, ttn_count, status
		 FROM sessions WHERE user_id=$1 ORDER BY started_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanSessions(rows, r.db, ctx, `SELECT COUNT(*) FROM sessions WHERE user_id=$1`, userID)
}

func (r *SessionRepo) ListAll(ctx context.Context, limit, offset int) ([]*domain.Session, int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT s.id, s.user_id, s.device_type, s.started_at, s.finished_at, s.ttn_count, s.status,
		        u.email, u.name
		 FROM sessions s
		 LEFT JOIN users u ON u.id = s.user_id
		 ORDER BY s.started_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanSessionsAdmin(rows, r.db, ctx)
}

func scanSessions(rows pgx.Rows, db *pgxpool.Pool, ctx context.Context, countQ string, args ...any) ([]*domain.Session, int, error) {
	var sessions []*domain.Session
	for rows.Next() {
		s := &domain.Session{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.DeviceType, &s.StartedAt, &s.FinishedAt, &s.TTNCount, &s.Status); err != nil {
			return nil, 0, err
		}
		sessions = append(sessions, s)
	}
	var total int
	_ = db.QueryRow(ctx, countQ, args...).Scan(&total)
	return sessions, total, nil
}

func scanSessionsAdmin(rows pgx.Rows, db *pgxpool.Pool, ctx context.Context) ([]*domain.Session, int, error) {
	var sessions []*domain.Session
	for rows.Next() {
		s := &domain.Session{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.DeviceType, &s.StartedAt, &s.FinishedAt, &s.TTNCount, &s.Status, &s.UserEmail, &s.UserName); err != nil {
			return nil, 0, err
		}
		sessions = append(sessions, s)
	}
	var total int
	_ = db.QueryRow(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&total)
	return sessions, total, nil
}

// CreateFinished creates a completed session with TTNs in a single transaction.
// Returns the finished session.
func (r *SessionRepo) CreateFinished(ctx context.Context, userID, deviceType string, ttns []*domain.SessionTTN) (*domain.Session, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	s := &domain.Session{}
	err = tx.QueryRow(ctx,
		`INSERT INTO sessions (user_id, device_type, status, finished_at)
		 VALUES ($1, $2, 'done', now())
		 RETURNING id, user_id, device_type, started_at, finished_at, ttn_count, status`,
		userID, deviceType,
	).Scan(&s.ID, &s.UserID, &s.DeviceType, &s.StartedAt, &s.FinishedAt, &s.TTNCount, &s.Status)
	if err != nil {
		return nil, fmt.Errorf("create finished session: %w", err)
	}

	for _, t := range ttns {
		if _, err := tx.Exec(ctx,
			`INSERT INTO session_ttns (session_id, ttn, status, message, registry) VALUES ($1,$2,$3,$4,$5)`,
			s.ID, t.TTN, t.Status, t.Message, t.Registry,
		); err != nil {
			return nil, fmt.Errorf("insert ttn: %w", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE sessions SET ttn_count=(SELECT COUNT(*) FROM session_ttns WHERE session_id=$1) WHERE id=$1`,
		s.ID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.TTNCount = len(ttns)
	return s, nil
}

// AbandonRunning marks all running sessions of a user as done before creating a new one.
func (r *SessionRepo) AbandonRunning(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET status='done', finished_at=now()
		 WHERE user_id=$1 AND status='running'`,
		userID,
	)
	return err
}

func (r *SessionRepo) Finish(ctx context.Context, id string, status domain.SessionStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET status=$1, finished_at=now() WHERE id=$2`,
		status, id,
	)
	return err
}

func (r *SessionRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE id=$1`, id)
	return err
}

// ReplaceTTNs deletes existing TTNs for the session and inserts the new ones.
func (r *SessionRepo) ReplaceTTNs(ctx context.Context, sessionID string, ttns []*domain.SessionTTN) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM session_ttns WHERE session_id=$1`, sessionID); err != nil {
		return err
	}
	for _, t := range ttns {
		if _, err := tx.Exec(ctx,
			`INSERT INTO session_ttns (session_id, ttn, status, message, registry) VALUES ($1,$2,$3,$4,$5)`,
			sessionID, t.TTN, t.Status, t.Message, t.Registry,
		); err != nil {
			return fmt.Errorf("insert ttn: %w", err)
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE sessions SET ttn_count=(SELECT COUNT(*) FROM session_ttns WHERE session_id=$1) WHERE id=$1`,
		sessionID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *SessionRepo) AddTTNs(ctx context.Context, sessionID string, ttns []*domain.SessionTTN) error {
	batch := &pgx.Batch{}
	for _, t := range ttns {
		batch.Queue(
			`INSERT INTO session_ttns (session_id, ttn, status, message, registry)
			 VALUES ($1, $2, $3, $4, $5)`,
			sessionID, t.TTN, t.Status, t.Message, t.Registry,
		)
	}
	batch.Queue(`UPDATE sessions SET ttn_count = (SELECT COUNT(*) FROM session_ttns WHERE session_id=$1) WHERE id=$1`, sessionID)
	br := r.db.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("add ttn batch[%d]: %w", i, err)
		}
	}
	return nil
}

func (r *SessionRepo) ListTTNs(ctx context.Context, sessionID string) ([]*domain.SessionTTN, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, session_id, ttn, status, COALESCE(message,''), COALESCE(registry,''), created_at
		 FROM session_ttns WHERE session_id=$1 ORDER BY id`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ttns []*domain.SessionTTN
	for rows.Next() {
		t := &domain.SessionTTN{}
		if err := rows.Scan(&t.ID, &t.SessionID, &t.TTN, &t.Status, &t.Message, &t.Registry, &t.CreatedAt); err != nil {
			return nil, err
		}
		ttns = append(ttns, t)
	}
	return ttns, nil
}
