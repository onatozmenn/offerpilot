package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/onatozmenn/offerpilot/internal/config"
	"github.com/onatozmenn/offerpilot/migrations"
	"github.com/pressly/goose/v3"
)

const initialMigrationVersion int64 = 1

var errStoreClosed = errors.New("postgres store is closed")

type Store struct {
	pool      *pgxpool.Pool
	sqlDB     *sql.DB
	provider  *goose.Provider
	closeOnce sync.Once
	closeErr  error
	closed    atomic.Bool
}

func Open(ctx context.Context, databaseConfig config.DatabaseConfig) (*Store, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseConfig.URL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL configuration: %w", err)
	}
	if databaseConfig.MaxConns < 1 {
		return nil, fmt.Errorf("database max connections must be positive")
	}
	if databaseConfig.MinConns < 0 || databaseConfig.MinConns > databaseConfig.MaxConns {
		return nil, fmt.Errorf("database min connections must be between zero and max connections")
	}
	if databaseConfig.MaxConnLifetime <= 0 || databaseConfig.MaxConnIdleTime <= 0 || databaseConfig.HealthCheckPeriod <= 0 {
		return nil, fmt.Errorf("database pool durations must be positive")
	}

	poolConfig.MaxConns = databaseConfig.MaxConns
	poolConfig.MinConns = databaseConfig.MinConns
	poolConfig.MaxConnLifetime = databaseConfig.MaxConnLifetime
	poolConfig.MaxConnIdleTime = databaseConfig.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = databaseConfig.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	provider, err := newMigrationProvider(sqlDB)
	if err != nil {
		closeErr := sqlDB.Close()
		pool.Close()
		return nil, errors.Join(fmt.Errorf("create migration provider: %w", err), closeErr)
	}

	return &Store{
		pool:     pool,
		sqlDB:    sqlDB,
		provider: provider,
	}, nil
}

func (store *Store) Migrate(ctx context.Context) error {
	if err := store.ensureOpen(); err != nil {
		return err
	}
	if _, err := store.provider.Up(ctx); err != nil {
		return fmt.Errorf("apply PostgreSQL migrations: %w", err)
	}
	version, err := store.provider.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("read PostgreSQL migration version: %w", err)
	}
	if version != initialMigrationVersion {
		return fmt.Errorf("unexpected PostgreSQL migration version %d", version)
	}

	return nil
}

func (store *Store) Ping(ctx context.Context) error {
	if err := store.ensureOpen(); err != nil {
		return err
	}
	if err := store.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return nil
}

func (store *Store) Close() error {
	if store == nil {
		return nil
	}
	store.closeOnce.Do(func() {
		store.closed.Store(true)
		if store.sqlDB != nil {
			store.closeErr = store.sqlDB.Close()
		}
		if store.pool != nil {
			store.pool.Close()
		}
	})

	return store.closeErr
}

func (store *Store) withTx(ctx context.Context, operation func(pgx.Tx) error) (returnErr error) {
	if err := store.ensureOpen(); err != nil {
		return err
	}
	if operation == nil {
		return fmt.Errorf("transaction operation is required")
	}

	transaction, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		rollbackErr := transaction.Rollback(ctx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			if returnErr == nil {
				returnErr = fmt.Errorf("rollback transaction: %w", rollbackErr)
			} else {
				returnErr = errors.Join(returnErr, fmt.Errorf("rollback transaction: %w", rollbackErr))
			}
		}
	}()

	if err := operation(transaction); err != nil {
		return fmt.Errorf("run transaction: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (store *Store) ensureOpen() error {
	if store == nil || store.pool == nil || store.provider == nil || store.closed.Load() {
		return errStoreClosed
	}

	return nil
}

func newMigrationProvider(database *sql.DB) (*goose.Provider, error) {
	upSQL, err := migrations.FS.ReadFile("000001_initial.up.sql")
	if err != nil {
		return nil, fmt.Errorf("read embedded up migration: %w", err)
	}
	downSQL, err := migrations.FS.ReadFile("000001_initial.down.sql")
	if err != nil {
		return nil, fmt.Errorf("read embedded down migration: %w", err)
	}

	migration := goose.NewGoMigration(
		initialMigrationVersion,
		migrationFunction("up", upSQL),
		migrationFunction("down", downSQL),
	)

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		database,
		nil,
		goose.WithDisableGlobalRegistry(true),
		goose.WithGoMigrations(migration),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize Goose provider: %w", err)
	}

	return provider, nil
}

func migrationFunction(direction string, contents []byte) *goose.GoFunc {
	script := string(contents)
	return &goose.GoFunc{
		RunTx: func(ctx context.Context, transaction *sql.Tx) error {
			if _, err := transaction.ExecContext(ctx, script); err != nil {
				return fmt.Errorf("execute embedded %s migration: %w", direction, err)
			}
			return nil
		},
	}
}
