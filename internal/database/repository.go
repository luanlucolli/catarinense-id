package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luanlucolli/auth-catarinense/internal/models"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrDuplicateUser   = errors.New("duplicate user")
	ErrAppNotFound     = errors.New("app not found")
	ErrSessionNotFound = errors.New("session not found")
)

const (
	stmtGetUserByUsername             = "get_user_by_username"
	stmtGetActiveAppByAPIKey          = "get_active_app_by_api_key"
	stmtUpsertSession                 = "upsert_session"
	stmtGetAuthContextBySessionAndApp = "get_auth_context_by_session_and_app"
	stmtDeleteSessionByID             = "delete_session_by_id"
	stmtCreateUser                    = "create_user"
)

const (
	defaultDBMaxConns              int32 = 4
	defaultDBMinConns              int32 = 0
	defaultDBMaxConnIdleTime             = 5 * time.Minute
	defaultDBMaxConnLifetime             = 30 * time.Minute
	defaultDBMaxConnLifetimeJitter       = 5 * time.Minute
	defaultDBHealthCheckPeriod           = time.Minute
	defaultDBConnectTimeout              = 10 * time.Second
	defaultSessionTouchInterval          = 5 * time.Minute
)

type UserStore interface {
	GetUserByUsername(ctx context.Context, username string) (models.User, error)
	GetActiveAppByAPIKey(ctx context.Context, apiKey string) (models.App, error)
	UpsertSession(ctx context.Context, params UpsertSessionParams) (models.Session, error)
	GetAuthContextBySessionAndAppKey(ctx context.Context, sessionUUID, apiKey string) (models.AuthContext, error)
	DeleteSessionByID(ctx context.Context, sessionID int32) error
	CreateUser(ctx context.Context, params CreateUserParams) (models.User, error)
}

type UpsertSessionParams struct {
	UserID      int32
	AppID       int32
	SessionUUID string
}

type CreateUserParams struct {
	Username     string
	PasswordHash string
	IsAdmin      bool
}

type Repository struct {
	pool                 *pgxpool.Pool
	sessionTouchInterval time.Duration
}

type PoolStats struct {
	MaxConns          int32
	TotalConns        int32
	AcquiredConns     int32
	IdleConns         int32
	EmptyAcquireCount int64
}

func NewRepository(ctx context.Context, databaseURL string) (*Repository, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	config.MaxConns = defaultDBMaxConns
	config.MinConns = defaultDBMinConns
	if config.MinConns > config.MaxConns {
		config.MinConns = defaultDBMinConns
	}
	config.MaxConnIdleTime = defaultDBMaxConnIdleTime
	config.MaxConnLifetime = defaultDBMaxConnLifetime
	config.MaxConnLifetimeJitter = defaultDBMaxConnLifetimeJitter
	config.HealthCheckPeriod = defaultDBHealthCheckPeriod
	config.ConnConfig.ConnectTimeout = defaultDBConnectTimeout

	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return prepareStatements(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	repo := &Repository{
		pool:                 pool,
		sessionTouchInterval: defaultSessionTouchInterval,
	}
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

func (r *Repository) Stats() PoolStats {
	stats := r.pool.Stat()

	return PoolStats{
		MaxConns:          stats.MaxConns(),
		TotalConns:        stats.TotalConns(),
		AcquiredConns:     stats.AcquiredConns(),
		IdleConns:         stats.IdleConns(),
		EmptyAcquireCount: stats.EmptyAcquireCount(),
	}
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

func (r *Repository) GetActiveAppByAPIKey(ctx context.Context, apiKey string) (models.App, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return models.App{}, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	app, err := scanApp(conn.QueryRow(ctx, stmtGetActiveAppByAPIKey, apiKey))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.App{}, ErrAppNotFound
		}

		return models.App{}, fmt.Errorf("query active app by api key: %w", err)
	}

	return app, nil
}

func (r *Repository) UpsertSession(ctx context.Context, params UpsertSessionParams) (models.Session, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return models.Session{}, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	session, err := scanSession(conn.QueryRow(
		ctx,
		stmtUpsertSession,
		params.UserID,
		params.AppID,
		params.SessionUUID,
	))
	if err != nil {
		return models.Session{}, fmt.Errorf("upsert session: %w", err)
	}

	return session, nil
}

func (r *Repository) GetAuthContextBySessionAndAppKey(ctx context.Context, sessionUUID, apiKey string) (models.AuthContext, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return models.AuthContext{}, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	authContext, err := scanAuthContext(
		conn.QueryRow(
			ctx,
			stmtGetAuthContextBySessionAndApp,
			sessionUUID,
			apiKey,
			int32(max(1, int(r.sessionTouchInterval/time.Second))),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.AuthContext{}, ErrSessionNotFound
		}

		return models.AuthContext{}, fmt.Errorf("query auth context by session and app: %w", err)
	}

	return authContext, nil
}

func (r *Repository) DeleteSessionByID(ctx context.Context, sessionID int32) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	result, err := conn.Exec(ctx, stmtDeleteSessionByID, sessionID)
	if err != nil {
		return fmt.Errorf("delete session by id: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrSessionNotFound
	}

	return nil
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
			SELECT id, username, password_hash, is_admin, active, created_at
			FROM users
			WHERE username = $1
		`,
		stmtGetActiveAppByAPIKey: `
			SELECT id, name, api_key, active, created_at
			FROM apps
			WHERE api_key = $1
			  AND active = true
		`,
		stmtUpsertSession: `
			INSERT INTO sessions (user_id, app_id, session_uuid, last_active)
			VALUES ($1, $2, $3::uuid, now())
			ON CONFLICT (user_id, app_id)
			DO UPDATE SET
				session_uuid = EXCLUDED.session_uuid,
				last_active = now(),
				created_at = now()
			RETURNING id, user_id, app_id, session_uuid::text, last_active, created_at
		`,
		stmtGetAuthContextBySessionAndApp: `
				WITH matched AS (
					SELECT
					u.id AS user_id,
					u.username,
					u.password_hash,
					u.is_admin,
					u.active AS user_active,
					u.created_at AS user_created_at,
					a.id AS app_id,
					a.name AS app_name,
					a.api_key,
					a.active AS app_active,
						a.created_at AS app_created_at,
						s.id AS session_id,
						s.user_id AS session_user_id,
						s.app_id AS session_app_id,
						s.session_uuid::text AS session_uuid,
						s.last_active AS session_last_active,
						s.created_at AS session_created_at
					FROM sessions s
					JOIN users u ON u.id = s.user_id
					JOIN apps a ON a.id = s.app_id
					WHERE s.session_uuid = $1::uuid
					  AND a.api_key = $2
					  AND u.active = true
					  AND a.active = true
				),
				touched AS (
					UPDATE sessions s
					SET last_active = now()
					FROM matched m
					WHERE s.id = m.session_id
					  AND s.last_active < now() - make_interval(secs => $3::int)
					RETURNING s.id, s.last_active
				)
				SELECT
				m.user_id,
				m.username,
				m.password_hash,
				m.is_admin,
				m.user_active,
				m.user_created_at,
				m.app_id,
				m.app_name,
				m.api_key,
				m.app_active,
				m.app_created_at,
				m.session_id,
					m.session_user_id,
					m.session_app_id,
					m.session_uuid,
					COALESCE(t.last_active, m.session_last_active),
					m.session_created_at
				FROM matched m
				LEFT JOIN touched t ON t.id = m.session_id
			`,
		stmtDeleteSessionByID: `
			DELETE FROM sessions
			WHERE id = $1
		`,
		stmtCreateUser: `
			INSERT INTO users (username, password_hash, is_admin)
			VALUES ($1, $2, $3)
			RETURNING id, username, password_hash, is_admin, active, created_at
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
		&user.CreatedAt,
	); err != nil {
		return models.User{}, err
	}

	return user, nil
}

func scanApp(row pgx.Row) (models.App, error) {
	var app models.App

	if err := row.Scan(
		&app.ID,
		&app.Name,
		&app.APIKey,
		&app.Active,
		&app.CreatedAt,
	); err != nil {
		return models.App{}, err
	}

	return app, nil
}

func scanSession(row pgx.Row) (models.Session, error) {
	var session models.Session

	if err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.AppID,
		&session.SessionUUID,
		&session.LastActive,
		&session.CreatedAt,
	); err != nil {
		return models.Session{}, err
	}

	return session, nil
}

func scanAuthContext(row pgx.Row) (models.AuthContext, error) {
	var authContext models.AuthContext

	if err := row.Scan(
		&authContext.User.ID,
		&authContext.User.Username,
		&authContext.User.PasswordHash,
		&authContext.User.IsAdmin,
		&authContext.User.Active,
		&authContext.User.CreatedAt,
		&authContext.App.ID,
		&authContext.App.Name,
		&authContext.App.APIKey,
		&authContext.App.Active,
		&authContext.App.CreatedAt,
		&authContext.Session.ID,
		&authContext.Session.UserID,
		&authContext.Session.AppID,
		&authContext.Session.SessionUUID,
		&authContext.Session.LastActive,
		&authContext.Session.CreatedAt,
	); err != nil {
		return models.AuthContext{}, err
	}

	return authContext, nil
}
