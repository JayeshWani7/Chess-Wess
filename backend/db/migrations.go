package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		createUsersTable,
		alterUsersAddBotColumns,
		createGamesTable,
		createGameMovesTable,
		createTimelinesTable,
		alterTimelinesAddName,
		alterGamesAddActiveTimeline,
		createGameNodesTable,
		alterGameNodesForSnapshots,
		createNodeChildrenTable,
		createPlayerEnergyTable,
		createEnergyTransactionsTable,
		createTimelineMetadataTable,
		alterTimelinesAddLocking,
		fixNullActiveTimeline,
		createNodeMergesTable,
		createNodeAnnotationsTable,
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
  time_control    INT NOT NULL DEFAULT 600,
  winner_id       UUID REFERENCES users(id),
  result          VARCHAR(16),
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
  move_san    VARCHAR(10) NOT NULL,
  move_uci    VARCHAR(6)  NOT NULL,
  fen_after   TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id, move_number)
);

CREATE INDEX IF NOT EXISTS idx_game_moves_game_id ON game_moves(game_id);
`

const createTimelinesTable = `
CREATE TABLE IF NOT EXISTS timelines (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  game_id       UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  root_node_id  UUID,
  timeline_name VARCHAR(64) NOT NULL DEFAULT 'Timeline',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by_user UUID REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_timelines_game_id ON timelines(game_id);
`

const alterTimelinesAddName = `
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='timelines' AND column_name='timeline_name') THEN
    ALTER TABLE timelines ADD COLUMN timeline_name VARCHAR(64) NOT NULL DEFAULT 'Timeline';
  END IF;
END $$;
`

const alterGamesAddActiveTimeline = `
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='games' AND column_name='active_timeline_id') THEN
    ALTER TABLE games ADD COLUMN active_timeline_id UUID;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'games_active_timeline_fkey'
  ) THEN
    ALTER TABLE games
      ADD CONSTRAINT games_active_timeline_fkey
      FOREIGN KEY (active_timeline_id) REFERENCES timelines(id);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_games_active_timeline_id ON games(active_timeline_id);
`

const createGameNodesTable = `
CREATE TABLE IF NOT EXISTS game_nodes (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  game_id          UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  timeline_id      UUID NOT NULL REFERENCES timelines(id) ON DELETE CASCADE,
  parent_node_id   UUID REFERENCES game_nodes(id),
  move_uci         VARCHAR(6),
  move_san         VARCHAR(10),
  move_promotion   VARCHAR(1),
  board_state      TEXT,
  is_snapshot      BOOLEAN NOT NULL DEFAULT FALSE,
  turn_number      INT NOT NULL,
  created_by_user  UUID NOT NULL REFERENCES users(id),

  is_check         BOOLEAN NOT NULL DEFAULT FALSE,
  is_checkmate     BOOLEAN NOT NULL DEFAULT FALSE,
  is_stalemate     BOOLEAN NOT NULL DEFAULT FALSE,
  evaluation       INT,
  captured_piece   VARCHAR(1),

  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (timeline_id, turn_number)
);

CREATE INDEX IF NOT EXISTS idx_game_nodes_game_id ON game_nodes(game_id);
CREATE INDEX IF NOT EXISTS idx_game_nodes_timeline_id ON game_nodes(timeline_id);
CREATE INDEX IF NOT EXISTS idx_game_nodes_parent_id ON game_nodes(parent_node_id);
CREATE INDEX IF NOT EXISTS idx_game_nodes_turn ON game_nodes(timeline_id, turn_number);
`

const alterGameNodesForSnapshots = `
DO $$ BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name='game_nodes' AND column_name='board_state' AND is_nullable='NO'
  ) THEN
    ALTER TABLE game_nodes ALTER COLUMN board_state DROP NOT NULL;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name='game_nodes' AND column_name='is_snapshot'
  ) THEN
    ALTER TABLE game_nodes ADD COLUMN is_snapshot BOOLEAN NOT NULL DEFAULT FALSE;
  END IF;
END $$;
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

const createPlayerEnergyTable = `
CREATE TABLE IF NOT EXISTS player_energy (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  game_id           UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  player_id         UUID NOT NULL REFERENCES users(id),
  energy_remaining  INT NOT NULL DEFAULT 15,
  energy_spent      INT NOT NULL DEFAULT 0,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id, player_id)
);

CREATE INDEX IF NOT EXISTS idx_player_energy_game_id ON player_energy(game_id);
CREATE INDEX IF NOT EXISTS idx_player_energy_player_id ON player_energy(player_id);
`

const createEnergyTransactionsTable = `
CREATE TABLE IF NOT EXISTS energy_transactions (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  game_id    UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  player_id  UUID NOT NULL REFERENCES users(id),
  amount     INT NOT NULL,
  action     VARCHAR(32) NOT NULL,
  details    TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_energy_transactions_game_id ON energy_transactions(game_id);
CREATE INDEX IF NOT EXISTS idx_energy_transactions_player_id ON energy_transactions(player_id);
`

const createTimelineMetadataTable = `
CREATE TABLE IF NOT EXISTS timeline_metadata (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  timeline_id           UUID NOT NULL REFERENCES timelines(id) ON DELETE CASCADE,
  game_id               UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  locked_by_player_id   UUID REFERENCES users(id),
  is_locked             BOOLEAN NOT NULL DEFAULT FALSE,
  stability_score       INT NOT NULL DEFAULT 100,
  energy_cost_to_create INT NOT NULL DEFAULT 0,
  paradox_count         INT NOT NULL DEFAULT 0,
  is_collapsed          BOOLEAN NOT NULL DEFAULT FALSE,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (timeline_id)
);

CREATE INDEX IF NOT EXISTS idx_timeline_metadata_game_id ON timeline_metadata(game_id);
CREATE INDEX IF NOT EXISTS idx_timeline_metadata_timeline_id ON timeline_metadata(timeline_id);
CREATE INDEX IF NOT EXISTS idx_timeline_metadata_locked ON timeline_metadata(is_locked);
CREATE INDEX IF NOT EXISTS idx_timeline_metadata_collapsed ON timeline_metadata(is_collapsed);
`

const alterTimelinesAddLocking = `
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='player_energy' AND column_name='energy_remaining') THEN
    CREATE TABLE IF NOT EXISTS player_energy (
      id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      game_id           UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
      player_id         UUID NOT NULL REFERENCES users(id),
      energy_remaining  INT NOT NULL DEFAULT 15,
      energy_spent      INT NOT NULL DEFAULT 0,
      created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      UNIQUE (game_id, player_id)
    );
  END IF;

  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='timeline_metadata' AND column_name='is_locked') THEN
    CREATE TABLE IF NOT EXISTS timeline_metadata (
      id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      timeline_id           UUID NOT NULL REFERENCES timelines(id) ON DELETE CASCADE,
      game_id               UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
      locked_by_player_id   UUID REFERENCES users(id),
      is_locked             BOOLEAN NOT NULL DEFAULT FALSE,
      stability_score       INT NOT NULL DEFAULT 100,
      energy_cost_to_create INT NOT NULL DEFAULT 0,
      paradox_count         INT NOT NULL DEFAULT 0,
      is_collapsed          BOOLEAN NOT NULL DEFAULT FALSE,
      created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      UNIQUE (timeline_id)
    );
  END IF;
END $$;
`

const fixNullActiveTimeline = `
UPDATE games g
SET active_timeline_id = (
  SELECT id FROM timelines t
  WHERE t.game_id = g.id
  ORDER BY created_at ASC
  LIMIT 1
)
WHERE active_timeline_id IS NULL;
`

const createNodeMergesTable = `
CREATE TABLE IF NOT EXISTS node_merges (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  game_id        UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  source_node_id UUID NOT NULL REFERENCES game_nodes(id) ON DELETE CASCADE,
  target_node_id UUID NOT NULL REFERENCES game_nodes(id) ON DELETE CASCADE,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id, source_node_id, target_node_id)
);

CREATE INDEX IF NOT EXISTS idx_node_merges_game_id ON node_merges(game_id);
`

const createNodeAnnotationsTable = `
CREATE TABLE IF NOT EXISTS node_annotations (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  node_id        UUID NOT NULL REFERENCES game_nodes(id) ON DELETE CASCADE,
  user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  annotation     TEXT NOT NULL,
  label_tag      VARCHAR(32),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (node_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_node_annotations_node_id ON node_annotations(node_id);
`

