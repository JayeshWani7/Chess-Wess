package db

import (
	"context"
	"fmt"
	"time"

	"github.com/ChessWess/backend/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/notnil/chess"
)

const (
	snapshotIntervalSmall        = 20
	snapshotIntervalMedium       = 12
	snapshotIntervalLarge        = 8
	snapshotIntervalSmallActive  = 12
	snapshotIntervalMediumActive = 8
	snapshotIntervalLargeActive  = 6

	smallTimelineMax  = 40
	mediumTimelineMax = 120

	recentActivityWindow = 2 * time.Minute
)

func ShouldSnapshotDynamic(turnNumber, timelineSize int, lastMoveAt, now time.Time) bool {
	if turnNumber <= 0 {
		return false
	}
	interval := snapshotIntervalFor(timelineSize, now.Sub(lastMoveAt) <= recentActivityWindow)
	return interval > 0 && turnNumber%interval == 0
}

func snapshotIntervalFor(timelineSize int, isActive bool) int {
	if isActive {
		if timelineSize <= smallTimelineMax {
			return snapshotIntervalSmallActive
		}
		if timelineSize <= mediumTimelineMax {
			return snapshotIntervalMediumActive
		}
		return snapshotIntervalLargeActive
	}

	if timelineSize <= smallTimelineMax {
		return snapshotIntervalSmall
	}
	if timelineSize <= mediumTimelineMax {
		return snapshotIntervalMedium
	}
	return snapshotIntervalLarge
}

func CreateTimeline(ctx context.Context, pool *pgxpool.Pool, gameID, createdByUser, timelineName string) (string, error) {
	var timelineID string
	var name string
	if timelineName != "" {
		name = timelineName
	} else {
		var count int
		err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM timelines WHERE game_id = $1`,
			gameID,
		).Scan(&count)
		if err != nil {
			return "", fmt.Errorf("CreateTimeline count: %w", err)
		}
		name = fmt.Sprintf("Timeline %d", count+1)
	}
	if len(name) > 64 {
		name = name[:64]
	}
	err := pool.QueryRow(ctx,
		`INSERT INTO timelines (game_id, created_by_user, timeline_name) 
		 VALUES ($1, $2, $3) 
		 RETURNING id`,
		gameID, createdByUser, name,
	).Scan(&timelineID)
	if err != nil {
		return "", fmt.Errorf("CreateTimeline: %w", err)
	}
	return timelineID, nil
}

func CreateRootNode(ctx context.Context, pool *pgxpool.Pool,
	gameID, timelineID, createdByUser string, initialFEN string) (string, error) {

	var nodeID string
	err := pool.QueryRow(ctx,
		`INSERT INTO game_nodes 
		 (game_id, timeline_id, parent_node_id, move_uci, move_san, move_promotion, 
		  board_state, is_snapshot, turn_number, created_by_user, is_check, is_checkmate, is_stalemate, captured_piece)
		 VALUES ($1, $2, NULL, NULL, NULL, NULL, $3, TRUE, 0, $4, FALSE, FALSE, FALSE, NULL)
		 RETURNING id`,
		gameID, timelineID, initialFEN, createdByUser,
	).Scan(&nodeID)
	if err != nil {
		return "", fmt.Errorf("CreateRootNode: %w", err)
	}

	_, err = pool.Exec(ctx,
		`UPDATE timelines SET root_node_id = $1 WHERE id = $2`,
		nodeID, timelineID,
	)
	if err != nil {
		return "", fmt.Errorf("CreateRootNode update timeline: %w", err)
	}

	return nodeID, nil
}

func CreateBranchRootNode(ctx context.Context, pool *pgxpool.Pool,
	gameID, timelineID, createdByUser, boardState string, turnNumber int) (string, error) {

	var nodeID string
	err := pool.QueryRow(ctx,
		`INSERT INTO game_nodes 
		 (game_id, timeline_id, parent_node_id, move_uci, move_san, move_promotion, 
		  board_state, is_snapshot, turn_number, created_by_user, is_check, is_checkmate, is_stalemate, captured_piece)
		 VALUES ($1, $2, NULL, NULL, NULL, NULL, $3, TRUE, $4, $5, FALSE, FALSE, FALSE, NULL)
		 RETURNING id`,
		gameID, timelineID, boardState, turnNumber, createdByUser,
	).Scan(&nodeID)
	if err != nil {
		return "", fmt.Errorf("CreateBranchRootNode: %w", err)
	}

	_, err = pool.Exec(ctx,
		`UPDATE timelines SET root_node_id = $1 WHERE id = $2`,
		nodeID, timelineID,
	)
	if err != nil {
		return "", fmt.Errorf("CreateBranchRootNode update timeline: %w", err)
	}

	return nodeID, nil
}

func CreateNode(ctx context.Context, pool *pgxpool.Pool, node *models.GameNode, parentNodeID string) (string, error) {
	var nodeID string

	var snapshotFEN *string
	if node.IsSnapshot {
		snapshotFEN = node.SnapshotFEN
		if snapshotFEN == nil && node.BoardState != "" {
			snapshotFEN = &node.BoardState
		}
	}

	promotion := ""
	if node.Move != nil && node.Move.Promotion != "" {
		promotion = node.Move.Promotion
	}

	err := pool.QueryRow(ctx,
		`INSERT INTO game_nodes 
		 (game_id, timeline_id, parent_node_id, move_uci, move_san, move_promotion,
		  board_state, is_snapshot, turn_number, created_by_user, is_check, is_checkmate, is_stalemate, captured_piece)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		 RETURNING id`,
		node.GameID, node.TimelineID, &parentNodeID,
		node.Move.UCI, node.Move.SAN, promotion,
		snapshotFEN, node.IsSnapshot, node.TurnNumber, node.CreatedByUser,
		node.Metadata.Check, node.Metadata.Checkmate, node.Metadata.Stalemate,
		node.Metadata.Captured,
	).Scan(&nodeID)
	if err != nil {
		return "", fmt.Errorf("CreateNode: %w", err)
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO node_children (parent_node_id, child_node_id) VALUES ($1, $2)`,
		parentNodeID, nodeID,
	)
	if err != nil {
		return "", fmt.Errorf("CreateNode insert relationship: %w", err)
	}

	return nodeID, nil
}

func LinkNodeChild(ctx context.Context, pool *pgxpool.Pool, parentNodeID, childNodeID string) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO node_children (parent_node_id, child_node_id) VALUES ($1, $2)`+
			` ON CONFLICT DO NOTHING`,
		parentNodeID, childNodeID,
	)
	if err != nil {
		return fmt.Errorf("LinkNodeChild: %w", err)
	}
	return nil
}

func GetNode(ctx context.Context, pool *pgxpool.Pool, nodeID string) (*models.GameNode, error) {
	path, err := GetNodePath(ctx, pool, nodeID)
	if err != nil {
		return nil, fmt.Errorf("GetNode: %w", err)
	}
	if len(path.Nodes) == 0 {
		return nil, fmt.Errorf("GetNode: no node found")
	}
	return &path.Nodes[len(path.Nodes)-1], nil
}

func GetNodePath(ctx context.Context, pool *pgxpool.Pool, targetNodeID string) (*models.GameNodePath, error) {
	rows, err := pool.Query(ctx,
		`WITH RECURSIVE path AS (
		   SELECT id, parent_node_id FROM game_nodes WHERE id = $1
		   
		   UNION
		   
		   SELECT gn.id, gn.parent_node_id FROM game_nodes gn
		   INNER JOIN path p ON gn.id = p.parent_node_id
		 )
		 SELECT gn.id, gn.game_id, gn.timeline_id, gn.parent_node_id, gn.move_uci, gn.move_san, 
		        gn.move_promotion, gn.board_state, gn.is_snapshot, gn.turn_number, gn.created_by_user, 
		        gn.is_check, gn.is_checkmate, gn.is_stalemate, gn.evaluation, 
		        gn.captured_piece, gn.created_at
		 FROM game_nodes gn
		 INNER JOIN path ON gn.id = path.id
		 ORDER BY gn.turn_number ASC`,
		targetNodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetNodePath query: %w", err)
	}
	defer rows.Close()

	var nodes []models.GameNode
	for rows.Next() {
		var node models.GameNode
		var moveUCI, moveSAN, promotion, capturedPiece *string
		var snapshotFEN *string
		var isSnapshot bool

		err := rows.Scan(
			&node.ID, &node.GameID, &node.TimelineID, &node.ParentNodeID,
			&moveUCI, &moveSAN, &promotion,
			&snapshotFEN, &isSnapshot, &node.TurnNumber, &node.CreatedByUser,
			&node.Metadata.Check, &node.Metadata.Checkmate, &node.Metadata.Stalemate,
			&node.Metadata.Evaluation, &capturedPiece,
			&node.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("GetNodePath scan: %w", err)
		}

		if moveUCI != nil && *moveUCI != "" {
			node.Move = &models.Move{
				UCI:       *moveUCI,
				SAN:       *moveSAN,
				Promotion: *promotion,
			}
		}
		if capturedPiece != nil {
			node.Metadata.Captured = *capturedPiece
		}

		node.SnapshotFEN = snapshotFEN
		node.IsSnapshot = isSnapshot

		nodes = append(nodes, node)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("GetNodePath: no path found")
	}

	if err := hydrateNodePath(nodes); err != nil {
		return nil, fmt.Errorf("GetNodePath hydrate: %w", err)
	}

	return &models.GameNodePath{Nodes: nodes, Count: len(nodes)}, nil
}

func GetNodeBranches(ctx context.Context, pool *pgxpool.Pool, parentNodeID string) ([]models.NodeBranch, error) {
	rows, err := pool.Query(ctx,
		`SELECT gn.id, gn.move_uci, gn.move_san, gn.timeline_id, gn.created_at
		 FROM game_nodes gn
		 INNER JOIN node_children nc ON gn.id = nc.child_node_id
		 WHERE nc.parent_node_id = $1
		 ORDER BY gn.created_at ASC`,
		parentNodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetNodeBranches: %w", err)
	}
	defer rows.Close()

	var branches []models.NodeBranch
	for rows.Next() {
		var branch models.NodeBranch
		var moveUCI, moveSAN *string

		err := rows.Scan(&branch.NodeID, &moveUCI, &moveSAN, &branch.TimelineID, &branch.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("GetNodeBranches scan: %w", err)
		}

		if moveUCI != nil {
			branch.MoveUCI = *moveUCI
		}
		if moveSAN != nil {
			branch.MoveSAN = *moveSAN
		}

		branches = append(branches, branch)
	}

	return branches, nil
}

func GetTimelineNodes(ctx context.Context, pool *pgxpool.Pool, timelineID string) ([]models.GameNode, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, game_id, timeline_id, parent_node_id, move_uci, move_san, move_promotion,
		        board_state, is_snapshot, turn_number, created_by_user, is_check, is_checkmate, is_stalemate,
		        evaluation, captured_piece, created_at
		 FROM game_nodes
		 WHERE timeline_id = $1
		 ORDER BY turn_number ASC`,
		timelineID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetTimelineNodes: %w", err)
	}
	defer rows.Close()

	var nodes []models.GameNode
	for rows.Next() {
		var node models.GameNode
		var moveUCI, moveSAN, promotion, capturedPiece *string
		var snapshotFEN *string
		var isSnapshot bool

		err := rows.Scan(
			&node.ID, &node.GameID, &node.TimelineID, &node.ParentNodeID,
			&moveUCI, &moveSAN, &promotion,
			&snapshotFEN, &isSnapshot, &node.TurnNumber, &node.CreatedByUser,
			&node.Metadata.Check, &node.Metadata.Checkmate, &node.Metadata.Stalemate,
			&node.Metadata.Evaluation, &capturedPiece,
			&node.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("GetTimelineNodes scan: %w", err)
		}

		if moveUCI != nil && *moveUCI != "" {
			node.Move = &models.Move{
				UCI:       *moveUCI,
				SAN:       *moveSAN,
				Promotion: *promotion,
			}
		}
		if capturedPiece != nil {
			node.Metadata.Captured = *capturedPiece
		}

		node.SnapshotFEN = snapshotFEN
		node.IsSnapshot = isSnapshot

		nodes = append(nodes, node)
	}

	if err := hydrateNodePath(nodes); err != nil {
		return nil, fmt.Errorf("GetTimelineNodes hydrate: %w", err)
	}

	return nodes, nil
}

func GetTimelineNodeCount(ctx context.Context, pool *pgxpool.Pool, timelineID string) (int, error) {
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM game_nodes WHERE timeline_id = $1`,
		timelineID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("GetTimelineNodeCount: %w", err)
	}
	return count, nil
}

func GetTimelineNodesWindow(ctx context.Context, pool *pgxpool.Pool, timelineID string, nodeLimit int) ([]models.GameNode, error) {
	rows, err := pool.Query(ctx,
		`WITH max_turn AS (
		   SELECT COALESCE(MAX(turn_number), 0) AS max_turn
		   FROM game_nodes
		   WHERE timeline_id = $1
		 ),
		 window_start AS (
		   SELECT GREATEST(max_turn - $2 + 1, 0) AS start_turn
		   FROM max_turn
		 ),
		 snapshot_start AS (
		   SELECT COALESCE(MAX(turn_number), 0) AS snapshot_turn
		   FROM game_nodes, window_start
		   WHERE timeline_id = $1
		     AND is_snapshot = TRUE
		     AND turn_number <= window_start.start_turn
		 )
		 SELECT id, game_id, timeline_id, parent_node_id, move_uci, move_san, move_promotion,
		        board_state, is_snapshot, turn_number, created_by_user, is_check, is_checkmate, is_stalemate,
		        evaluation, captured_piece, created_at
		 FROM game_nodes, snapshot_start
		 WHERE timeline_id = $1
		   AND turn_number >= snapshot_start.snapshot_turn
		 ORDER BY turn_number ASC`,
		timelineID, nodeLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("GetTimelineNodesWindow: %w", err)
	}
	defer rows.Close()

	var nodes []models.GameNode
	for rows.Next() {
		var node models.GameNode
		var moveUCI, moveSAN, promotion, capturedPiece *string
		var snapshotFEN *string
		var isSnapshot bool

		err := rows.Scan(
			&node.ID, &node.GameID, &node.TimelineID, &node.ParentNodeID,
			&moveUCI, &moveSAN, &promotion,
			&snapshotFEN, &isSnapshot, &node.TurnNumber, &node.CreatedByUser,
			&node.Metadata.Check, &node.Metadata.Checkmate, &node.Metadata.Stalemate,
			&node.Metadata.Evaluation, &capturedPiece,
			&node.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("GetTimelineNodesWindow scan: %w", err)
		}

		if moveUCI != nil && *moveUCI != "" {
			node.Move = &models.Move{
				UCI:       *moveUCI,
				SAN:       *moveSAN,
				Promotion: *promotion,
			}
		}
		if capturedPiece != nil {
			node.Metadata.Captured = *capturedPiece
		}

		node.SnapshotFEN = snapshotFEN
		node.IsSnapshot = isSnapshot

		nodes = append(nodes, node)
	}

	if err := hydrateNodePath(nodes); err != nil {
		return nil, fmt.Errorf("GetTimelineNodesWindow hydrate: %w", err)
	}

	return nodes, nil
}

func GetLatestTimelineNode(ctx context.Context, pool *pgxpool.Pool, timelineID string) (*models.GameNode, error) {
	var nodeID string
	if err := pool.QueryRow(ctx,
		`SELECT id FROM game_nodes WHERE timeline_id = $1 ORDER BY turn_number DESC LIMIT 1`,
		timelineID,
	).Scan(&nodeID); err != nil {
		return nil, fmt.Errorf("GetLatestTimelineNode: %w", err)
	}
	return GetNode(ctx, pool, nodeID)
}

func hydrateNodePath(nodes []models.GameNode) error {
	var game *chess.Game

	for i := range nodes {
		node := &nodes[i]

		if node.SnapshotFEN != nil && *node.SnapshotFEN != "" && node.IsSnapshot {
			fenOpt, err := chess.FEN(*node.SnapshotFEN)
			if err != nil {
				return fmt.Errorf("snapshot fen invalid: %w", err)
			}
			game = chess.NewGame(fenOpt)
			node.BoardState = *node.SnapshotFEN
			continue
		}

		if game == nil {
			if node.SnapshotFEN != nil && *node.SnapshotFEN != "" {
				fenOpt, err := chess.FEN(*node.SnapshotFEN)
				if err != nil {
					return fmt.Errorf("snapshot fen invalid: %w", err)
				}
				game = chess.NewGame(fenOpt)
				node.BoardState = *node.SnapshotFEN
				continue
			}
			return fmt.Errorf("missing snapshot for path")
		}

		if node.Move != nil && node.Move.UCI != "" {
			if err := applyUCIMove(game, node.Move.UCI); err != nil {
				return fmt.Errorf("apply move: %w", err)
			}
		}

		if game != nil {
			node.BoardState = game.Position().String()
		}
	}

	return nil
}

func applyUCIMove(game *chess.Game, uci string) error {
	pos := game.Position()
	for _, mv := range game.ValidMoves() {
		if (chess.UCINotation{}).Encode(pos, mv) == uci {
			return game.Move(mv)
		}
	}
	return fmt.Errorf("illegal move: %s", uci)
}

func GetGameTimelines(ctx context.Context, pool *pgxpool.Pool, gameID string) ([]models.Timeline, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, game_id, root_node_id, timeline_name, created_at, created_by_user
		 FROM timelines
		 WHERE game_id = $1
		 ORDER BY created_at ASC`,
		gameID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetGameTimelines: %w", err)
	}
	defer rows.Close()

	var timelines []models.Timeline
	for rows.Next() {
		var timeline models.Timeline
		err := rows.Scan(&timeline.ID, &timeline.GameID, &timeline.RootNodeID,
			&timeline.TimelineName, &timeline.CreatedAt, &timeline.CreatedByUser)
		if err != nil {
			return nil, fmt.Errorf("GetGameTimelines scan: %w", err)
		}
		timelines = append(timelines, timeline)
	}

	return timelines, nil
}

func UpdateTimelineName(ctx context.Context, pool *pgxpool.Pool, gameID, timelineID, timelineName string) error {
	if len(timelineName) > 64 {
		timelineName = timelineName[:64]
	}
	res, err := pool.Exec(ctx,
		`UPDATE timelines
		 SET timeline_name = $1
		 WHERE id = $2 AND game_id = $3`,
		timelineName, timelineID, gameID,
	)
	if err != nil {
		return fmt.Errorf("UpdateTimelineName: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("UpdateTimelineName: timeline not found")
	}
	return nil
}
