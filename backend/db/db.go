package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Connect opens a pgxpool connection with production-tuned pool settings.
// dsn must be a valid libpq / pgx connection string; an empty string is
// rejected so callers catch misconfiguration early.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.ParseConfig: %w", err)
	}

	// ---- Connection pool tuning ----
	// MaxConns: pgx default is 4 which is far too small.  25 is a safe starting
	// point for a single-instance deployment; raise it if you observe pool
	// exhaustion under load.
	cfg.MaxConns = 25

	// MinConns: keep a handful of connections warm so the first burst of
	// requests does not pay the handshake cost.
	cfg.MinConns = 5

	// MaxConnLifetime: recycle connections periodically to recover from
	// server-side connection limits and transparent load-balancer timeouts.
	cfg.MaxConnLifetime = 30 * time.Minute

	// MaxConnIdleTime: close connections that have been idle for a while so we
	// don't hold more DB resources than we need during quiet periods.
	cfg.MaxConnIdleTime = 5 * time.Minute

	// HealthCheckPeriod: regularly ping idle connections to detect stale ones.
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return pool, nil
}

// ConnectRedis opens a Redis client.  An empty url defaults to localhost.
func ConnectRedis(url string) (*redis.Client, error) {
	if url == "" {
		url = "redis://localhost:6379"
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis.ParseURL: %w", err)
	}
	rdb := redis.NewClient(opts)
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return rdb, nil
}
