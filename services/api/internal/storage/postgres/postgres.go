// Package postgres holds the durable stores behind the domain interfaces declared
// in internal/platform and internal/auth.
//
// Everything moderation-critical is resolved here from server-owned rows, never
// from a request body, so that the trust boundary described in
// docs/architecture/overview.md holds even against a modified client.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolationCode = "23505"

// Connect opens a pool and verifies it, so a misconfigured DATABASE_URL fails at
// startup rather than on the first user request.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingContext); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == uniqueViolationCode
}

// isUUID guards lookups whose parameters come from a request path or body.
//
// Postgres rejects a malformed uuid literal with an error, which would surface as a
// 500 for what is really a client mistake. Checking the shape first lets callers
// answer "not found" instead.
func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}

	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			isDigit := character >= '0' && character <= '9'
			isLowerHex := character >= 'a' && character <= 'f'
			isUpperHex := character >= 'A' && character <= 'F'
			if !isDigit && !isLowerHex && !isUpperHex {
				return false
			}
		}
	}

	return true
}
