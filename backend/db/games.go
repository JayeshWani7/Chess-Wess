package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GetActiveTimelineID returns the active timeline for a game, if set.
func GetActiveTimelineID(ctx context.Context, pool *pgxpool.Pool, gameID string) (*string, error) {
	var activeTimelineID *string
	if err := pool.QueryRow(ctx,
		`SELECT active_timeline_id FROM games WHERE id = $1`,
		gameID,
	).Scan(&activeTimelineID); err != nil {
		return nil, fmt.Errorf("GetActiveTimelineID: %w", err)
	}
	return activeTimelineID, nil
}

// SetActiveTimelineID sets the active timeline for a game.
// It only succeeds if the timeline belongs to the game.
func SetActiveTimelineID(ctx context.Context, pool *pgxpool.Pool, gameID, timelineID string) error {
	ct, err := pool.Exec(ctx,
		`UPDATE games
		 SET active_timeline_id = $2, updated_at = NOW()
		 WHERE id = $1
		   AND EXISTS (SELECT 1 FROM timelines WHERE id = $2 AND game_id = $1)`,
		gameID, timelineID,
	)
	if err != nil {
		return fmt.Errorf("SetActiveTimelineID: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("SetActiveTimelineID: timeline not found for game")
	}
	return nil
}
