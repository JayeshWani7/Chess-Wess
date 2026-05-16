package models

import "time"

// PlayerEnergy tracks time energy pool for each player in a game
// Phase 5: Players get limited rewinds per game with energy costs
type PlayerEnergy struct {
	ID              string    `json:"id"`
	GameID          string    `json:"game_id"`
	PlayerID        string    `json:"player_id"`
	EnergyRemaining int       `json:"energy_remaining"` // Current energy pool (starts at 15)
	EnergySpent     int       `json:"energy_spent"`     // Total energy spent in game
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TimelineMetadata stores energy costs, locking status, and stability info
// Phase 5: Timelines can be locked, have paradox penalties, and collapse
type TimelineMetadata struct {
	ID                 string    `json:"id"`
	TimelineID         string    `json:"timeline_id"`
	GameID             string    `json:"game_id"`
	LockedByPlayerID   *string   `json:"locked_by_player_id,omitempty"` // nil if not locked
	IsLocked           bool      `json:"is_locked"`                     // Prevents opponent rewinding into it
	StabilityScore     int       `json:"stability_score"`               // 0-100, decreases with paradoxes
	EnergyCostToCreate int       `json:"energy_cost_to_create"`         // How much energy was spent creating this timeline
	ParadoxCount       int       `json:"paradox_count"`                 // Number of contradictions detected
	IsCollapsed        bool      `json:"is_collapsed"`                  // Marked for deletion (Time Collapse)
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// EnergyCostConfig defines the energy costs for different actions
// Phase 5: Energy economics
type EnergyCostConfig struct {
	RewindPerTurn        int // Rewind 1 turn costs 1 energy
	JumpTimelineCost     int // Jump to different timeline costs 1 energy
	LockTimelineCost     int // Lock a timeline costs 3 energy
	ParadoxPenalty       int // Paradox created: -2 energy (penalty)
	InitialEnergyPool    int // Players start with 15 energy per game
	TimelineCollapseSize int // If 30+ timelines, weakest collapses
}

// GetDefaultEnergyCostConfig returns Phase 5 energy mechanics
func GetDefaultEnergyCostConfig() EnergyCostConfig {
	return EnergyCostConfig{
		RewindPerTurn:        1,  // Rewind N turns = N energy
		JumpTimelineCost:     1,  // Jump timeline = 1 energy
		LockTimelineCost:     3,  // Lock timeline = 3 energy
		ParadoxPenalty:       2,  // Paradox penalty = -2 energy
		InitialEnergyPool:    15, // Start with 15 energy
		TimelineCollapseSize: 30, // Collapse if 30+ timelines exist
	}
}

// EnergyTransaction records energy spending for audit trail
type EnergyTransaction struct {
	ID        string    `json:"id"`
	GameID    string    `json:"game_id"`
	PlayerID  string    `json:"player_id"`
	Amount    int       `json:"amount"`  // Positive = spent, Negative = refund
	Action    string    `json:"action"`  // "rewind", "jump_timeline", "lock", "paradox_penalty"
	Details   string    `json:"details"` // Additional context
	CreatedAt time.Time `json:"created_at"`
}
