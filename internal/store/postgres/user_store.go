package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
)

type User struct {
	ID           int
	Nickname     string
	PasswordHash string
}

type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

func (s *UserStore) Create(ctx context.Context, nickname, passwordHash string) error {
	_, err := s.pool.Exec(ctx,
		"INSERT INTO users (nickname, password_hash) VALUES ($1, $2)",
		nickname, passwordHash,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrUserAlreadyExists
		}
		return err
	}
	return nil
}

func (s *UserStore) GetByNickname(ctx context.Context, nickname string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		"SELECT id, nickname, password_hash FROM users WHERE nickname = $1",
		nickname,
	).Scan(&u.ID, &u.Nickname, &u.PasswordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func isUniqueViolation(err error) bool {
	// Postgres unique violation = SQLSTATE 23505
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
