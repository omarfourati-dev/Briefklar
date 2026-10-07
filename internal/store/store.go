// Package store keeps accounts and daily usage counters in Postgres. There are no letter tables on purpose.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

var (
	ErrNotFound   = errors.New("not found")
	ErrEmailTaken = errors.New("email already used")
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	Enabled      bool      `json:"enabled"`
	DailyLimit   int       `json:"dailyLimit"`
	UsedToday    int       `json:"usedToday"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close()                         { s.pool.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		version, err := strconv.Atoi(strings.SplitN(strings.TrimPrefix(name, "migrations/"), "_", 2)[0])
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		sql, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
			// the advisory lock keeps two starting containers from migrating at the same time
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4711)`); err != nil {
				return err
			}
			var done bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&done); err != nil || done {
				return err
			}
			// simple protocol: a migration file may contain several statements
			if _, err := tx.Conn().PgConn().Exec(ctx, string(sql)).ReadAll(); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version)
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

const userCols = `id::text, email, password_hash, display_name, role, enabled, daily_limit, created_at`

func scanUser(row pgx.Row, extra ...any) (User, error) {
	var u User
	dest := append([]any{&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.Enabled, &u.DailyLimit, &u.CreatedAt}, extra...)
	err := row.Scan(dest...)
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return User{}, ErrNotFound
	case errors.As(err, &pgErr) && pgErr.Code == "22P02": // invalid uuid text
		return User{}, ErrNotFound
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		return User{}, ErrEmailTaken
	}
	return u, err
}

func (s *Store) CreateUser(ctx context.Context, email, hash, name, role string, dailyLimit int) (User, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`INSERT INTO app_user (email, password_hash, display_name, role, daily_limit)
		 VALUES (lower($1), $2, $3, $4, $5) RETURNING `+userCols, email, hash, name, role, dailyLimit))
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM app_user WHERE email = lower($1)`, email))
}

func (s *Store) UserByID(ctx context.Context, id string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM app_user WHERE id = $1::uuid`, id))
}

func (s *Store) ListUsers(ctx context.Context, day time.Time) ([]User, error) {
	rows, err := s.pool.Query(ctx, `SELECT a.id::text, a.email, a.password_hash, a.display_name, a.role, a.enabled,
	        a.daily_limit, a.created_at, COALESCE(u.count, 0)
	   FROM app_user a LEFT JOIN usage_day u ON u.user_id = a.id AND u.day = $1::date
	  ORDER BY a.created_at`, day.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var used int
		u, err := scanUser(rows, &used)
		if err != nil {
			return nil, err
		}
		u.UsedToday = used
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) SetEnabled(ctx context.Context, id string, enabled bool) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `UPDATE app_user SET enabled = $2 WHERE id = $1::uuid RETURNING `+userCols, id, enabled))
}

func (s *Store) SetPassword(ctx context.Context, id, hash string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE app_user SET password_hash = $2 WHERE id = $1::uuid`, id, hash)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// IsActive is asked on every authenticated request, so a disabled account loses access at once.
func (s *Store) IsActive(ctx context.Context, id string) (bool, error) {
	u, err := s.UserByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil && u.Enabled, err
}

// TakeQuota counts one explanation for the day. It refuses atomically once daily_limit is reached.
func (s *Store) TakeQuota(ctx context.Context, userID string, day time.Time) (int, bool, error) {
	var used int
	err := s.pool.QueryRow(ctx, `
		INSERT INTO usage_day (user_id, day, count)
		SELECT id, $2::date, 1 FROM app_user WHERE id = $1::uuid AND daily_limit > 0
		ON CONFLICT (user_id, day) DO UPDATE SET count = usage_day.count + 1
		 WHERE usage_day.count < (SELECT daily_limit FROM app_user WHERE id = $1::uuid)
		RETURNING count`, userID, day.Format("2006-01-02")).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return used, true, nil
}
