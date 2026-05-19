package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/ChessWess/backend/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InitPlayerEnergy(ctx context.Context, pool *pgxpool.Pool, gameID string, whitePlayerID, blackPlayerID string, initialEnergy int) error {
	query := `
		INSERT INTO player_energy (game_id, player_id, energy_remaining, energy_spent)
		VALUES ($1, $2, $3, 0), ($1, $4, $3, 0)
	`
	_, err := pool.Exec(ctx, query, gameID, whitePlayerID, initialEnergy, blackPlayerID)
	return err
}

func GetPlayerEnergy(ctx context.Context, pool *pgxpool.Pool, gameID, playerID string) (*models.PlayerEnergy, error) {
	var pe models.PlayerEnergy
	query := `SELECT id, game_id, player_id, energy_remaining, energy_spent, created_at, updated_at
	         FROM player_energy WHERE game_id = $1 AND player_id = $2`
	err := pool.QueryRow(ctx, query, gameID, playerID).Scan(
		&pe.ID, &pe.GameID, &pe.PlayerID, &pe.EnergyRemaining, &pe.EnergySpent, &pe.CreatedAt, &pe.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &pe, nil
}

func SpendEnergy(ctx context.Context, pool *pgxpool.Pool, gameID, playerID string, amount int, action, details string) error {
	if amount <= 0 {
		return errors.New("energy amount must be positive")
	}

	pe, err := GetPlayerEnergy(ctx, pool, gameID, playerID)
	if err != nil {
		return fmt.Errorf("failed to get player energy: %w", err)
	}

	if pe.EnergyRemaining < amount {
		return fmt.Errorf("insufficient energy: need %d, have %d", amount, pe.EnergyRemaining)
	}

	updateQuery := `UPDATE player_energy 
	               SET energy_remaining = energy_remaining - $1, 
	                   energy_spent = energy_spent + $1,
	                   updated_at = NOW()
	               WHERE game_id = $2 AND player_id = $3`
	_, err = pool.Exec(ctx, updateQuery, amount, gameID, playerID)
	if err != nil {
		return fmt.Errorf("failed to spend energy: %w", err)
	}

	logQuery := `INSERT INTO energy_transactions (game_id, player_id, amount, action, details, created_at)
	           VALUES ($1, $2, $3, $4, $5, NOW())`
	_, err = pool.Exec(ctx, logQuery, gameID, playerID, amount, action, details)
	return err
}

func RefundEnergy(ctx context.Context, pool *pgxpool.Pool, gameID, playerID string, amount int, reason string) error {
	if amount <= 0 {
		return errors.New("refund amount must be positive")
	}

	updateQuery := `UPDATE player_energy 
	               SET energy_remaining = energy_remaining + $1, 
	                   updated_at = NOW()
	               WHERE game_id = $2 AND player_id = $3`
	_, err := pool.Exec(ctx, updateQuery, amount, gameID, playerID)
	if err != nil {
		return err
	}

	logQuery := `INSERT INTO energy_transactions (game_id, player_id, amount, action, details, created_at)
	           VALUES ($1, $2, $3, 'refund', $4, NOW())`
	_, err = pool.Exec(ctx, logQuery, gameID, playerID, -amount, reason)
	return err
}

func InitTimelineMetadata(ctx context.Context, pool *pgxpool.Pool, timelineID, gameID string, energyCost int) error {
	query := `
		INSERT INTO timeline_metadata (timeline_id, game_id, energy_cost_to_create, stability_score, created_at, updated_at)
		VALUES ($1, $2, $3, 100, NOW(), NOW())
	`
	_, err := pool.Exec(ctx, query, timelineID, gameID, energyCost)
	return err
}

func GetTimelineMetadata(ctx context.Context, pool *pgxpool.Pool, timelineID string) (*models.TimelineMetadata, error) {
	var tm models.TimelineMetadata
	query := `SELECT id, timeline_id, game_id, locked_by_player_id, is_locked, stability_score,
	                 energy_cost_to_create, paradox_count, is_collapsed, created_at, updated_at
	         FROM timeline_metadata WHERE timeline_id = $1`
	err := pool.QueryRow(ctx, query, timelineID).Scan(
		&tm.ID, &tm.TimelineID, &tm.GameID, &tm.LockedByPlayerID, &tm.IsLocked, &tm.StabilityScore,
		&tm.EnergyCostToCreate, &tm.ParadoxCount, &tm.IsCollapsed, &tm.CreatedAt, &tm.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &tm, nil
}

func LockTimeline(ctx context.Context, pool *pgxpool.Pool, timelineID, playerID string) error {
	query := `UPDATE timeline_metadata 
	         SET is_locked = true, locked_by_player_id = $1, updated_at = NOW()
	         WHERE timeline_id = $2`
	_, err := pool.Exec(ctx, query, playerID, timelineID)
	return err
}

func UnlockTimeline(ctx context.Context, pool *pgxpool.Pool, timelineID string) error {
	query := `UPDATE timeline_metadata 
	         SET is_locked = false, locked_by_player_id = NULL, updated_at = NOW()
	         WHERE timeline_id = $1`
	_, err := pool.Exec(ctx, query, timelineID)
	return err
}

func IsTimelineLocked(ctx context.Context, pool *pgxpool.Pool, timelineID string) (bool, error) {
	var isLocked bool
	query := `SELECT is_locked FROM timeline_metadata WHERE timeline_id = $1`
	err := pool.QueryRow(ctx, query, timelineID).Scan(&isLocked)
	if err != nil {
		return false, err
	}
	return isLocked, nil
}

func ApplyParadoxPenalty(ctx context.Context, pool *pgxpool.Pool, timelineID string) error {
	query := `UPDATE timeline_metadata 
	         SET paradox_count = paradox_count + 1,
	             stability_score = GREATEST(0, stability_score - 10),
	             updated_at = NOW()
	         WHERE timeline_id = $1`
	_, err := pool.Exec(ctx, query, timelineID)
	return err
}

func GetTimelineMetadataForGame(ctx context.Context, pool *pgxpool.Pool, gameID string) ([]*models.TimelineMetadata, error) {
	query := `SELECT id, timeline_id, game_id, locked_by_player_id, is_locked, stability_score,
	                 energy_cost_to_create, paradox_count, is_collapsed, created_at, updated_at
	         FROM timeline_metadata WHERE game_id = $1 AND is_collapsed = false
	         ORDER BY stability_score ASC`
	rows, err := pool.Query(ctx, query, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timelines []*models.TimelineMetadata
	for rows.Next() {
		var tm models.TimelineMetadata
		err := rows.Scan(&tm.ID, &tm.TimelineID, &tm.GameID, &tm.LockedByPlayerID, &tm.IsLocked,
			&tm.StabilityScore, &tm.EnergyCostToCreate, &tm.ParadoxCount, &tm.IsCollapsed, &tm.CreatedAt, &tm.UpdatedAt)
		if err != nil {
			return nil, err
		}
		timelines = append(timelines, &tm)
	}
	return timelines, rows.Err()
}

func CheckTimelineCollapse(ctx context.Context, pool *pgxpool.Pool, gameID string, collapseThreshold int) error {
	timelines, err := GetTimelineMetadataForGame(ctx, pool, gameID)
	if err != nil {
		return err
	}

	if len(timelines) < collapseThreshold {
		return nil
	}

	timesToCollapse := len(timelines) - collapseThreshold + 5
	for i := 0; i < timesToCollapse && i < len(timelines); i++ {
		if !timelines[i].IsLocked {
			query := `UPDATE timeline_metadata SET is_collapsed = true, updated_at = NOW() WHERE timeline_id = $1`
			_, err := pool.Exec(ctx, query, timelines[i].TimelineID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func MarkTimelineForCollapse(ctx context.Context, pool *pgxpool.Pool, timelineID string) error {
	query := `UPDATE timeline_metadata SET is_collapsed = true, updated_at = NOW() WHERE timeline_id = $1`
	_, err := pool.Exec(ctx, query, timelineID)
	return err
}

func GetCollapsedTimelines(ctx context.Context, pool *pgxpool.Pool, gameID string) ([]string, error) {
	query := `SELECT timeline_id FROM timeline_metadata WHERE game_id = $1 AND is_collapsed = true`
	rows, err := pool.Query(ctx, query, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timelineIDs []string
	for rows.Next() {
		var timelineID string
		if err := rows.Scan(&timelineID); err != nil {
			return nil, err
		}
		timelineIDs = append(timelineIDs, timelineID)
	}
	return timelineIDs, rows.Err()
}

func DeleteCollapsedTimeline(ctx context.Context, pool *pgxpool.Pool, timelineID string) error {
	deleteNodesQuery := `DELETE FROM game_nodes WHERE timeline_id = $1`
	_, err := pool.Exec(ctx, deleteNodesQuery, timelineID)
	if err != nil {
		return fmt.Errorf("failed to delete nodes: %w", err)
	}

	deleteTimelineQuery := `DELETE FROM timelines WHERE id = $1`
	_, err = pool.Exec(ctx, deleteTimelineQuery, timelineID)
	if err != nil {
		return fmt.Errorf("failed to delete timeline: %w", err)
	}

	deleteMetadataQuery := `DELETE FROM timeline_metadata WHERE timeline_id = $1`
	_, err = pool.Exec(ctx, deleteMetadataQuery, timelineID)
	return err
}
