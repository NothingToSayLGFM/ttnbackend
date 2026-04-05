package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"ttnflow-api/internal/domain"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (id, email, name, password_hash, role, np_api_key, created_at, updated_at)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, now(), now())`,
		u.Email, u.Name, u.PasswordHash, u.Role, u.NPAPIKey,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, name, password_hash, role, COALESCE(np_api_key,''), created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.NPAPIKey, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

func (r *UserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, name, password_hash, role, COALESCE(np_api_key,''), created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.NPAPIKey, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

func (r *UserRepo) List(ctx context.Context, limit, offset int) ([]*domain.User, int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, email, name, password_hash, role, COALESCE(np_api_key,''), created_at, updated_at
		 FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.NPAPIKey, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}

	var total int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)
	return users, total, nil
}

func (r *UserRepo) UpdateName(ctx context.Context, id, name string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET name=$1, updated_at=now() WHERE id=$2`, name, id)
	return err
}

func (r *UserRepo) UpdatePasswordHash(ctx context.Context, id, hash string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password_hash=$1, updated_at=now() WHERE id=$2`, hash, id)
	return err
}

func (r *UserRepo) UpdateNPAPIKey(ctx context.Context, id, key string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET np_api_key=$1, updated_at=now() WHERE id=$2`, key, id)
	return err
}

func (r *UserRepo) UpdateRole(ctx context.Context, id string, role domain.Role) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET role=$1, updated_at=now() WHERE id=$2`, role, id)
	return err
}

func (r *UserRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	return err
}
