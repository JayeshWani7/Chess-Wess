package models

import "time"

// Timeline represents a branch of game history.
// Each timeline starts from a root node and can have multiple leaf nodes.
type Timeline struct {
	ID            string    `json:"id"`
	GameID        string    `json:"game_id"`
	RootNodeID    string    `json:"root_node_id"`
	CreatedAt     time.Time `json:"created_at"`
	CreatedByUser string    `json:"created_by_user"` // User ID who created this timeline (via rewind in Phase 3)
}

// GameNode represents a single immutable board state within a timeline.
// Nodes form a DAG: multiple children can reference one parent (branching).
type GameNode struct {
	ID            string           `json:"id"`
	GameID        string           `json:"game_id"`
	TimelineID    string           `json:"timeline_id"`
	ParentNodeID  *string          `json:"parent_node_id"` // nil if root node
	Move          *Move            `json:"move"`           // nil if root node
	BoardState    string           `json:"board_state"`    // FEN string
	TurnNumber    int              `json:"turn_number"`    // 0-based move count
	CreatedByUser string           `json:"created_by_user"`
	Metadata      GameNodeMetadata `json:"metadata"`
	CreatedAt     time.Time        `json:"created_at"`
}

// Move represents a single chess move with full details.
type Move struct {
	UCI       string `json:"uci"`       // e2e4 format
	SAN       string `json:"san"`       // e4 (algebraic notation)
	Promotion string `json:"promotion"` // Empty or q/r/b/n
}

// GameNodeMetadata contains analysis and game state info for a node.
type GameNodeMetadata struct {
	Check      bool   `json:"check"`
	Checkmate  bool   `json:"checkmate"`
	Stalemate  bool   `json:"stalemate"`
	Evaluation *int   `json:"evaluation,omitempty"` // Stockfish score (centipawns), will be used in Phase 7
	Captured   string `json:"captured,omitempty"`   // Piece that was captured (if any)
}

// NodeBranch represents a child node of a given parent (for traversal).
type NodeBranch struct {
	NodeID     string    `json:"node_id"`
	MoveUCI    string    `json:"move_uci"`
	MoveSAN    string    `json:"move_san"`
	TimelineID string    `json:"timeline_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// GameNodePath represents the path from root to a specific node.
// Used for replay: contains all nodes from root to target in order.
type GameNodePath struct {
	Nodes []GameNode `json:"nodes"`
	Count int        `json:"count"`
}
