package models

import "time"

type PlayerEnergy struct {
	ID              string    `json:"id"`
	GameID          string    `json:"game_id"`
	PlayerID        string    `json:"player_id"`
	EnergyRemaining int       `json:"energy_remaining"`
	EnergySpent     int       `json:"energy_spent"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type TimelineMetadata struct {
	ID                 string    `json:"id"`
	TimelineID         string    `json:"timeline_id"`
	GameID             string    `json:"game_id"`
	LockedByPlayerID   *string   `json:"locked_by_player_id,omitempty"`
	IsLocked           bool      `json:"is_locked"`
	StabilityScore     int       `json:"stability_score"`
	EnergyCostToCreate int       `json:"energy_cost_to_create"`
	ParadoxCount       int       `json:"paradox_count"`
	IsCollapsed        bool      `json:"is_collapsed"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type EnergyCostConfig struct {
	RewindPerTurn        int
	JumpTimelineCost     int
	LockTimelineCost     int
	ParadoxPenalty       int
	InitialEnergyPool    int
	TimelineCollapseSize int
}

func GetDefaultEnergyCostConfig() EnergyCostConfig {
	return EnergyCostConfig{
		RewindPerTurn:        1,
		JumpTimelineCost:     1,
		LockTimelineCost:     3,
		ParadoxPenalty:       2,
		InitialEnergyPool:    15,
		TimelineCollapseSize: 30,
	}
}

type EnergyTransaction struct {
	ID        string    `json:"id"`
	GameID    string    `json:"game_id"`
	PlayerID  string    `json:"player_id"`
	Amount    int       `json:"amount"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}
