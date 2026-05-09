package models

import "time"

// GameStatus represents the lifecycle state of a game.
type GameStatus string

const (
	GameStatusPending   GameStatus = "pending"
	GameStatusActive    GameStatus = "active"
	GameStatusCompleted GameStatus = "completed"
	GameStatusAbandoned GameStatus = "abandoned"
)

// Game represents a chess match between two players.
type Game struct {
	ID             string     `json:"id"`
	WhitePlayerID  *string    `json:"white_player_id"`
	BlackPlayerID  *string    `json:"black_player_id"`
	Status         GameStatus `json:"status"`
	TimeControl    int        `json:"time_control"` // seconds per player; 0 = unlimited
	WinnerID       *string    `json:"winner_id,omitempty"`
	Result         *string    `json:"result,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// GameMove represents a single move within a game.
type GameMove struct {
	ID         string    `json:"id"`
	GameID     string    `json:"game_id"`
	PlayerID   string    `json:"player_id"`
	MoveNumber int       `json:"move_number"`
	MoveSAN    string    `json:"move_san"`
	MoveUCI    string    `json:"move_uci"`
	FENAfter   string    `json:"fen_after"`
	CreatedAt  time.Time `json:"created_at"`
}
