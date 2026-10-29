// Package db opens the Postgres pool and applies the embedded goose migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedded embed.FS

var migrationsFS = func() fs.FS {
	sub, err := fs.Sub(embedded, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}()

// Open connects to Postgres and migrates the schema to the latest version.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Migrate applies every pending migration. It is safe to run on every boot.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn := stdlib.OpenDBFromPool(pool)
	defer conn.Close()
	return migrate(ctx, conn)
}

func migrate(ctx context.Context, conn *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, conn, migrationsFS)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
