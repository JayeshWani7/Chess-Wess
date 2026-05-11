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
		createTimelinesTable,
		createGameNodesTable,
		createNodeChildrenTable,
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

const createTimelinesTable = `
CREATE TABLE IF NOT EXISTS timelines (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  game_id       UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  root_node_id  UUID,  -- Will be set after root node is created
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by_user UUID REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_timelines_game_id ON timelines(game_id);
`

const createGameNodesTable = `
CREATE TABLE IF NOT EXISTS game_nodes (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  game_id          UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  timeline_id      UUID NOT NULL REFERENCES timelines(id) ON DELETE CASCADE,
  parent_node_id   UUID REFERENCES game_nodes(id),
  move_uci         VARCHAR(6),                     -- e2e4 format; NULL if root
  move_san         VARCHAR(10),                    -- e4 format; NULL if root
  move_promotion   VARCHAR(1),                     -- q, r, b, n; NULL if no promotion
  board_state      TEXT NOT NULL,                  -- FEN string
  turn_number      INT NOT NULL,                   -- 0-based move count
  created_by_user  UUID NOT NULL REFERENCES users(id),
  
  -- Metadata
  is_check         BOOLEAN NOT NULL DEFAULT FALSE,
  is_checkmate     BOOLEAN NOT NULL DEFAULT FALSE,
  is_stalemate     BOOLEAN NOT NULL DEFAULT FALSE,
  evaluation       INT,                            -- Stockfish score in centipawns (Phase 7)
  captured_piece   VARCHAR(1),                     -- q, r, b, n, p, k; NULL if no capture
  
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (timeline_id, turn_number)
);

CREATE INDEX IF NOT EXISTS idx_game_nodes_game_id ON game_nodes(game_id);
CREATE INDEX IF NOT EXISTS idx_game_nodes_timeline_id ON game_nodes(timeline_id);
CREATE INDEX IF NOT EXISTS idx_game_nodes_parent_id ON game_nodes(parent_node_id);
CREATE INDEX IF NOT EXISTS idx_game_nodes_turn ON game_nodes(timeline_id, turn_number);
`

const createNodeChildrenTable = `
CREATE TABLE IF NOT EXISTS node_children (
  parent_node_id UUID NOT NULL REFERENCES game_nodes(id) ON DELETE CASCADE,
  child_node_id  UUID NOT NULL REFERENCES game_nodes(id) ON DELETE CASCADE,
  PRIMARY KEY (parent_node_id, child_node_id)
);

CREATE INDEX IF NOT EXISTS idx_node_children_parent ON node_children(parent_node_id);
CREATE INDEX IF NOT EXISTS idx_node_children_child ON node_children(child_node_id);
`
