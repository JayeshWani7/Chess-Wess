package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations applies all schema migrations in order.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		createUsersTable,
		alterUsersAddBotColumns,
		createGamesTable,
		createGameMovesTable,
	}

	for i, m := range migrations {
		if _, err := pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}
	return nil
}

const createUsersTable = `
CREATE TABLE IF NOT EXISTS users (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username    VARCHAR(32) UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  is_bot      BOOLEAN NOT NULL DEFAULT FALSE,
  rating      INT NOT NULL DEFAULT 1200,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const alterUsersAddBotColumns = `
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='is_bot') THEN
    ALTER TABLE users ADD COLUMN is_bot BOOLEAN NOT NULL DEFAULT FALSE;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='rating') THEN
    ALTER TABLE users ADD COLUMN rating INT NOT NULL DEFAULT 1200;
  END IF;
END $$;
`

const createGamesTable = `
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'game_status') THEN
    CREATE TYPE game_status AS ENUM ('pending', 'active', 'completed', 'abandoned');
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS games (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  white_player_id UUID REFERENCES users(id),
  black_player_id UUID REFERENCES users(id),
  status          game_status NOT NULL DEFAULT 'pending',
  time_control    INT NOT NULL DEFAULT 600,  -- seconds per player (0 = unlimited)
  winner_id       UUID REFERENCES users(id),
  result          VARCHAR(16),               -- 'checkmate','stalemate','timeout','resign','draw'
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const createGameMovesTable = `
CREATE TABLE IF NOT EXISTS game_moves (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  game_id     UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  player_id   UUID NOT NULL REFERENCES users(id),
  move_number INT NOT NULL,
  move_san    VARCHAR(10) NOT NULL,   -- Standard Algebraic Notation e.g. "e4", "Nf3"
  move_uci    VARCHAR(6)  NOT NULL,   -- UCI format e.g. "e2e4"
  fen_after   TEXT NOT NULL,          -- Board state after this move
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id, move_number)
);

CREATE INDEX IF NOT EXISTS idx_game_moves_game_id ON game_moves(game_id);
`
