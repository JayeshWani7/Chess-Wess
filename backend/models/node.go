package models

import "time"

type Timeline struct {
	ID            string    `json:"id"`
	GameID        string    `json:"game_id"`
	RootNodeID    string    `json:"root_node_id"`
	TimelineName  string    `json:"timeline_name"`
	CreatedAt     time.Time `json:"created_at"`
	CreatedByUser string    `json:"created_by_user"`
}

type GameNode struct {
	ID            string           `json:"id"`
	GameID        string           `json:"game_id"`
	TimelineID    string           `json:"timeline_id"`
	ParentNodeID  *string          `json:"parent_node_id"`
	Move          *Move            `json:"move"`
	BoardState    string           `json:"board_state"`
	SnapshotFEN   *string          `json:"-"`
	IsSnapshot    bool             `json:"-"`
	TurnNumber    int              `json:"turn_number"`
	CreatedByUser string           `json:"created_by_user"`
	Metadata      GameNodeMetadata `json:"metadata"`
	CreatedAt     time.Time        `json:"created_at"`
}

type Move struct {
	UCI       string `json:"uci"`
	SAN       string `json:"san"`
	Promotion string `json:"promotion"`
}

type GameNodeMetadata struct {
	Check      bool   `json:"check"`
	Checkmate  bool   `json:"checkmate"`
	Stalemate  bool   `json:"stalemate"`
	Evaluation *int   `json:"evaluation,omitempty"`
	Captured   string `json:"captured,omitempty"`
}

type NodeBranch struct {
	NodeID     string    `json:"node_id"`
	MoveUCI    string    `json:"move_uci"`
	MoveSAN    string    `json:"move_san"`
	TimelineID string    `json:"timeline_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type GameNodePath struct {
	Nodes []GameNode `json:"nodes"`
	Count int        `json:"count"`
}

type NodeAnnotation struct {
	ID         string    `json:"id"`
	NodeID     string    `json:"node_id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	Annotation string    `json:"annotation"`
	LabelTag   *string   `json:"label_tag"`
	CreatedAt  time.Time `json:"created_at"`
}
