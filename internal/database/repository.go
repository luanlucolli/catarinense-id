package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luanlucolli/auth-catarinense/internal/models"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrDuplicateUser   = errors.New("duplicate user")
	ErrSessionNotFound = errors.New("session not found")
)

const (
	stmtGetUserByUsername      = "get_user_by_username"
	stmtUpdateUserSessionUUID  = "update_user_session_uuid"
	stmtGetActiveUserBySession = "get_active_user_by_session_uuid"
	stmtCreateUser             = "create_user"
)

type UserStore interface {
	GetUserByUsername(ctx context.Context, username string) (models.User, error)
	UpdateSessionUUID(ctx context.Context, userID int32, sessionUUID string) error
	GetActiveUserBySessionUUID(ctx context.Context, sessionUUID string) (models.User, error)
	CreateUser(ctx context.Context, params CreateUserParams) (models.User, error)
}

type CreateUserParams struct {
	Username     string
	PasswordHash string
	IsAdmin      bool
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, databaseURL string) (*Repository, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return prepareStatements(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	repo := &Repository{pool: pool}
	if err := repo.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return repo, nil
}

func (r *Repository) Close() {
	r.pool.Close()
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return models.User{}, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	user, err := scanUser(conn.QueryRow(ctx, stmtGetUserByUsername, username))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrUserNotFound
		}

		return models.User{}, fmt.Errorf("query user by username: %w", err)
	}

	return user, nil
}

func (r *Repository) UpdateSessionUUID(ctx context.Context, userID int32, sessionUUID string) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	commandTag, err := conn.Exec(ctx, stmtUpdateUserSessionUUID, sessionUUID, userID)
	if err != nil {
		return fmt.Errorf("update session uuid: %w", err)
	}

	if commandTag.RowsAffected() != 1 {
		return ErrUserNotFound
	}

	return nil
}

func (r *Repository) GetActiveUserBySessionUUID(ctx context.Context, sessionUUID string) (models.User, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return models.User{}, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	user, err := scanUser(conn.QueryRow(ctx, stmtGetActiveUserBySession, sessionUUID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrSessionNotFound
		}

		return models.User{}, fmt.Errorf("query active user by session: %w", err)
	}

	return user, nil
}

func (r *Repository) CreateUser(ctx context.Context, params CreateUserParams) (models.User, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return models.User{}, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	user, err := scanUser(conn.QueryRow(ctx, stmtCreateUser, params.Username, params.PasswordHash, params.IsAdmin))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return models.User{}, ErrDuplicateUser
		}

		return models.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func prepareStatements(ctx context.Context, conn *pgx.Conn) error {
	statements := map[string]string{
		stmtGetUserByUsername: `
			SELECT id, username, password_hash, is_admin, active, session_uuid, created_at
			FROM users
			WHERE username = $1
		`,
		stmtUpdateUserSessionUUID: `
			UPDATE users
			SET session_uuid = $1
			WHERE id = $2
		`,
		stmtGetActiveUserBySession: `
			SELECT id, username, password_hash, is_admin, active, session_uuid, created_at
			FROM users
			WHERE session_uuid = $1
			  AND active = true
		`,
		stmtCreateUser: `
			INSERT INTO users (username, password_hash, is_admin)
			VALUES ($1, $2, $3)
			RETURNING id, username, password_hash, is_admin, active, session_uuid, created_at
		`,
	}

	for name, sql := range statements {
		if _, err := conn.Prepare(ctx, name, sql); err != nil {
			return fmt.Errorf("prepare statement %q: %w", name, err)
		}
	}

	return nil
}

func scanUser(row pgx.Row) (models.User, error) {
	var user models.User

	if err := row.Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.Active,
		&user.SessionUUID,
		&user.CreatedAt,
	); err != nil {
		return models.User{}, err
	}

	return user, nil
}
