package db

import (
	"context"
	"fmt"
	"github.com/ChessWess/backend/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetGame(ctx context.Context, pool *pgxpool.Pool, gameID string) (*models.Game, error) {
	var game models.Game
	err := pool.QueryRow(ctx,
		`SELECT id, white_player_id, black_player_id, status, time_control, active_timeline_id, winner_id, result, created_at, updated_at
		 FROM games WHERE id = $1`,
		gameID,
	).Scan(&game.ID, &game.WhitePlayerID, &game.BlackPlayerID, &game.Status, &game.TimeControl, &game.ActiveTimelineID, &game.WinnerID, &game.Result, &game.CreatedAt, &game.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("GetGame: %w", err)
	}
	return &game, nil
}

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
