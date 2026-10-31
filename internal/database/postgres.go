// Package database contains helpers for connecting to PostgreSQL.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Registers the "pgx" driver with database/sql.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open connects to PostgreSQL using the given connection string and verifies
// the connection with a ping. It retries for a short while so the API can be
// started right after the database container.
func Open(ctx context.Context, url string) (*sql.DB, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	var pingErr error
	for attempt := 1; attempt <= 10; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		pingErr = db.PingContext(pingCtx)
		cancel()
		if pingErr == nil {
			return db, nil
		}
		select {
		case <-ctx.Done():
			db.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}

	db.Close()
	return nil, fmt.Errorf("ping database: %w", pingErr)
}
