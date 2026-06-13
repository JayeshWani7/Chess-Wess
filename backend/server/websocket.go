package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/ChessWess/backend/db"
	"github.com/ChessWess/backend/models"
	"github.com/ChessWess/backend/observability"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/notnil/chess"
)

var (
	errMoveConflict   = errors.New("move conflict")
	errMoveNotActive  = errors.New("game not active")
	errTimelineAbsent = errors.New("timeline not found")
)

// upgrader is initialised per-server so it can validate origins.
// The zero-value is not used; s.newUpgrader() is called instead.

// newUpgrader builds a websocket.Upgrader that validates the Origin header
// against the server's allowed-origins list.
func (s *Server) newUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			// If no Origin header (e.g. non-browser clients), allow.
			if origin == "" {
				return true
			}
			return isOriginAllowed(origin, s.allowedOrigins)
		},
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, err := validateJWT(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	gameID := r.URL.Query().Get("game_id")
	if gameID == "" {
		http.Error(w, "game_id required", http.StatusBadRequest)
		return
	}

	// Ensure the bot is running if this is a bot game
	s.StartBotIfNeeded(r.Context(), gameID)

	upgrader := s.newUpgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	lastSeqStr := r.URL.Query().Get("last_seq")
	var lastSeq uint64
	var hasLastSeq bool
	if lastSeqStr != "" {
		if val, err := strconv.ParseUint(lastSeqStr, 10, 64); err == nil {
			lastSeq = val
			hasLastSeq = true
		}
	}

	// Set the initial read deadline; the writePump's pong handler extends it.
	conn.SetReadDeadline(time.Now().Add(pongWait))

	client := &Client{
		hub:        s.hub,
		gameID:     gameID,
		userID:     userID,
		send:       make(chan []byte, 64),
		conn:       conn,
		lastSeq:    lastSeq,
		hasLastSeq: hasLastSeq,
	}

	s.hub.join <- client
	go client.writePump()

	s.hub.Broadcast(gameID, WSMessage{
		Type:    "player_connected",
		Payload: map[string]string{"user_id": userID},
	})

	s.readPump(client)
}

func (s *Server) readPump(c *Client) {
	c.disconnectReason = "normal"
	defer func() {
		s.hub.leave <- c
		s.hub.Broadcast(c.gameID, WSMessage{
			Type:    "player_disconnected",
			Payload: map[string]string{"user_id": c.userID},
		})
	}()

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			c.disconnectReason = "error"
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("ws: bad message from %s: %v", c.userID, err)
			continue
		}

		switch msg.Type {
		case "move":
			s.handleMoveMessage(c, msg)
		case "rewind":
			s.handleRewindMessage(c, msg)
		case "switch_timeline":
			s.handleSwitchTimelineMessage(c, msg)
		case "ping":
			c.send <- mustMarshal(WSMessage{Type: "pong"})
		case "pong":
		default:
			log.Printf("ws: unknown message type %q from %s", msg.Type, c.userID)
		}
	}
}

func (s *Server) handleMoveMessage(c *Client, msg WSMessage) {
	start := time.Now()

	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return
	}

	uci, _ := payload["uci"].(string)
	timelineID, _ := payload["timeline_id"].(string)
	parentNodeID, _ := payload["parent_node_id"].(string)

	if uci == "" {
		s.obs.RecordMoveValidationError("missing_uci")
		s.obs.RecordMove(c.gameID, "", "error", time.Since(start))
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "move requires uci"})
		return
	}

	ctx := context.Background()

	parentNode, resolvedTimelineID, err := s.resolveTimelineParent(ctx, c.gameID, timelineID, parentNodeID)
	if err != nil {
		latencyMs := time.Since(start).Seconds() * 1000
		s.obs.RecordMoveValidationError("invalid_timeline_context")
		s.obs.RecordMove(c.gameID, "", "error", time.Since(start))
		s.log.Warn("move_timeline_error",
			"game_id", c.gameID,
			"timeline_id", timelineID,
			"user_id", c.userID,
			"uci", uci,
			"reason", err.Error(),
			"latency_ms", latencyMs,
		)
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "invalid timeline context"})
		return
	}

	fenOpt, err := chess.FEN(parentNode.BoardState)
	if err != nil {
		log.Printf("ws: bad parent FEN: %v", err)
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "invalid board state"})
		return
	}

	game := chess.NewGame(fenOpt)
	pos := game.Position()
	validMoves := game.ValidMoves()
	var selected *chess.Move
	for _, mv := range validMoves {
		if (chess.UCINotation{}).Encode(pos, mv) == uci {
			selected = mv
			break
		}
	}
	if selected == nil {
		latencyMs := time.Since(start).Seconds() * 1000
		s.obs.RecordMoveValidationError("illegal_move")
		s.obs.RecordMove(c.gameID, resolvedTimelineID, "illegal", time.Since(start))
		s.log.Warn("move_failed",
			append(observability.GameFields(c.gameID, resolvedTimelineID, parentNode.ID, parentNode.TurnNumber),
				"user_id", c.userID,
				"uci", uci,
				"reason", "illegal_move",
				"latency_ms", latencyMs,
			)...,
		)
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "illegal move"})
		return
	}

	san := (chess.AlgebraicNotation{}).Encode(pos, selected)
	if err := game.Move(selected); err != nil {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "illegal move"})
		return
	}

	fen := game.Position().String()

	promotion := ""
	if selected.Promo() != chess.NoPieceType {
		promotion = selected.Promo().String()
	}

	moveResult, err := s.applyMoveAtomic(ctx, c.gameID, c.userID, uci, san, promotion, fen, resolvedTimelineID, parentNode.ID)
	if err != nil {
		var outcome, reason string
		if errors.Is(err, errMoveConflict) {
			outcome = "conflict"
			reason = "conflict"
		} else if errors.Is(err, errMoveNotActive) {
			outcome = "error"
			reason = "game_not_active"
		} else if errors.Is(err, errTimelineAbsent) {
			outcome = "error"
			reason = "invalid_timeline"
		} else {
			outcome = "error"
			reason = "error"
			log.Printf("ws: failed to apply move: %v", err)
			s.log.Error("move_apply_error",
				"game_id", c.gameID,
				"timeline_id", resolvedTimelineID,
				"user_id", c.userID,
				"uci", uci,
				"reason", err.Error(),
				"latency_ms", time.Since(start).Seconds()*1000,
			)
		}
		s.obs.RecordMoveValidationError(reason)
		s.obs.RecordMove(c.gameID, resolvedTimelineID, outcome, time.Since(start))
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: reason})
		return
	}

	latencyMs := time.Since(start).Seconds() * 1000
	s.obs.RecordMove(c.gameID, moveResult.TimelineID, "success", time.Since(start))
	s.log.Info("move_applied",
		append(observability.GameFields(c.gameID, moveResult.TimelineID, moveResult.NodeID, moveResult.TurnNumber),
			"user_id", c.userID,
			"uci", uci,
			"san", san,
			"latency_ms", latencyMs,
		)...,
	)

	s.hub.Broadcast(c.gameID, WSMessage{
		Type: "move",
		Payload: map[string]interface{}{
			"id":             moveResult.NodeID,
			"player_id":      c.userID,
			"uci":            uci,
			"san":            san,
			"fen":            fen,
			"timeline_id":    moveResult.TimelineID,
			"parent_node_id": moveResult.ParentNodeID,
			"turn_number":    moveResult.TurnNumber,
			"created_at":     moveResult.CreatedAt.UTC().Format(time.RFC3339),
		},
	})

	if moveResult.GameOver {
		s.hub.Broadcast(c.gameID, WSMessage{
			Type:    "game_over",
			Payload: map[string]string{"winner_id": moveResult.WinnerID, "result": moveResult.Result},
		})
	}
}

func outcomeFromFEN(fen string) (string, bool) {
	fenOpt, err := chess.FEN(fen)
	if err != nil {
		return "", false
	}
	game := chess.NewGame(fenOpt)
	status := game.Position().Status()
	if status == chess.Checkmate {
		return "checkmate", true
	}
	if status == chess.Stalemate {
		return "stalemate", true
	}
	return "", false
}

func (s *Server) createGameNode(ctx context.Context, gameID, userID, uci, san, promotion, fen, timelineID, parentNodeID string) (string, error) {
	parentNode, resolvedTimelineID, err := s.resolveTimelineParent(ctx, gameID, timelineID, parentNodeID)
	if err != nil {
		return "", fmt.Errorf("createGameNode: %w", err)
	}

	var isCheck, isCheckmate, isStalemate bool

	fenOpt, err := chess.FEN(fen)
	if err == nil {
		game := chess.NewGame(fenOpt)
		pos := game.Position()
		status := pos.Status()

		isCheckmate = status == chess.Checkmate
		isStalemate = status == chess.Stalemate

		if isCheckmate {
			isCheck = true
		} else if !isStalemate && status == chess.NoMethod {
			isCheck = len(game.ValidMoves()) == 0
		}
	}

	resolvedParentID := parentNode.ID
	turnNumber := parentNode.TurnNumber + 1
	nodeCount, err := db.GetTimelineNodeCount(ctx, s.db, resolvedTimelineID)
	if err != nil {
		return "", fmt.Errorf("createGameNode count: %w", err)
	}
	timelineSize := nodeCount + 1
	nodeData := &models.GameNode{
		GameID:        gameID,
		TimelineID:    resolvedTimelineID,
		ParentNodeID:  &resolvedParentID,
		Move:          &models.Move{UCI: uci, SAN: san, Promotion: promotion},
		TurnNumber:    turnNumber,
		CreatedByUser: userID,
		Metadata: models.GameNodeMetadata{
			Check:     isCheck,
			Checkmate: isCheckmate,
			Stalemate: isStalemate,
		},
	}

	if db.ShouldSnapshotDynamic(turnNumber, timelineSize, parentNode.CreatedAt, time.Now()) {
		nodeData.IsSnapshot = true
		nodeData.SnapshotFEN = &fen
	}

	return db.CreateNode(ctx, s.db, nodeData, resolvedParentID)
}

type moveApplyResult struct {
	NodeID       string
	TimelineID   string
	ParentNodeID string
	TurnNumber   int
	CreatedAt    time.Time
	GameOver     bool
	Result       string
	WinnerID     string
}

func (s *Server) applyMoveAtomic(ctx context.Context, gameID, userID, uci, san, promotion, fen, timelineID, parentNodeID string) (*moveApplyResult, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("applyMoveAtomic begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var gameStatus string
	var activeTimelineID *string
	if err := tx.QueryRow(ctx,
		`SELECT status, active_timeline_id FROM games WHERE id = $1 FOR UPDATE`,
		gameID,
	).Scan(&gameStatus, &activeTimelineID); err != nil {
		return nil, fmt.Errorf("applyMoveAtomic game lock: %w", err)
	}

	if timelineID == "" {
		if activeTimelineID != nil && *activeTimelineID != "" {
			timelineID = *activeTimelineID
		} else {
			if err := tx.QueryRow(ctx,
				`SELECT id FROM timelines WHERE game_id = $1 ORDER BY created_at ASC LIMIT 1`,
				gameID,
			).Scan(&timelineID); err != nil {
				return nil, fmt.Errorf("applyMoveAtomic timeline fallback: %w", err)
			}
		}
	}

	var timelineExists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM timelines WHERE id = $1 AND game_id = $2)`,
		timelineID, gameID,
	).Scan(&timelineExists); err != nil {
		return nil, fmt.Errorf("applyMoveAtomic timeline check: %w", err)
	}
	if !timelineExists {
		return nil, errTimelineAbsent
	}

	type parentInfo struct {
		ID        string
		Timeline  string
		Turn      int
		CreatedAt time.Time
	}

	var parent parentInfo
	if parentNodeID != "" {
		if err := tx.QueryRow(ctx,
			`SELECT id, timeline_id, turn_number, created_at
			 FROM game_nodes
			 WHERE id = $1 AND game_id = $2
			 FOR UPDATE`,
			parentNodeID, gameID,
		).Scan(&parent.ID, &parent.Timeline, &parent.Turn, &parent.CreatedAt); err != nil {
			return nil, fmt.Errorf("applyMoveAtomic parent: %w", err)
		}
		if timelineID != parent.Timeline {
			return nil, errTimelineAbsent
		}
	} else {
		if err := tx.QueryRow(ctx,
			`SELECT id, timeline_id, turn_number, created_at
			 FROM game_nodes
			 WHERE timeline_id = $1
			 ORDER BY turn_number DESC
			 LIMIT 1
			 FOR UPDATE`,
			timelineID,
		).Scan(&parent.ID, &parent.Timeline, &parent.Turn, &parent.CreatedAt); err != nil {
			return nil, fmt.Errorf("applyMoveAtomic latest parent: %w", err)
		}
		parentNodeID = parent.ID
	}

	var latestID string
	if err := tx.QueryRow(ctx,
		`SELECT id FROM game_nodes WHERE timeline_id = $1 ORDER BY turn_number DESC LIMIT 1 FOR UPDATE`,
		timelineID,
	).Scan(&latestID); err != nil {
		return nil, fmt.Errorf("applyMoveAtomic latest check: %w", err)
	}
	if latestID != parent.ID {
		return nil, errMoveConflict
	}

	var existingID string
	var existingTurn int
	var existingCreatedAt time.Time
	err = tx.QueryRow(ctx,
		`SELECT gn.id, gn.turn_number, gn.created_at
		 FROM game_nodes gn
		 INNER JOIN node_children nc ON gn.id = nc.child_node_id
		 WHERE nc.parent_node_id = $1
		   AND gn.move_uci = $2
		   AND gn.timeline_id = $3
		 LIMIT 1`,
		parent.ID, uci, timelineID,
	).Scan(&existingID, &existingTurn, &existingCreatedAt)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("applyMoveAtomic commit existing: %w", err)
		}
		return &moveApplyResult{
			NodeID:       existingID,
			TimelineID:   timelineID,
			ParentNodeID: parent.ID,
			TurnNumber:   existingTurn,
			CreatedAt:    existingCreatedAt,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("applyMoveAtomic idempotent check: %w", err)
	}

	if gameStatus != string(models.GameStatusActive) {
		return nil, errMoveNotActive
	}

	var isCheck, isCheckmate, isStalemate bool
	if fenOpt, fenErr := chess.FEN(fen); fenErr == nil {
		game := chess.NewGame(fenOpt)
		pos := game.Position()
		status := pos.Status()

		isCheckmate = status == chess.Checkmate
		isStalemate = status == chess.Stalemate

		if isCheckmate {
			isCheck = true
		} else if !isStalemate && status == chess.NoMethod {
			isCheck = len(game.ValidMoves()) == 0
		}
	}

	turnNumber := parent.Turn + 1
	var nodeCount int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM game_nodes WHERE timeline_id = $1`,
		timelineID,
	).Scan(&nodeCount); err != nil {
		return nil, fmt.Errorf("applyMoveAtomic count: %w", err)
	}

	timelineSize := nodeCount + 1
	shouldSnapshot := db.ShouldSnapshotDynamic(turnNumber, timelineSize, parent.CreatedAt, time.Now())
	var snapshotFEN *string
	if shouldSnapshot {
		snapshotFEN = &fen
	}

	var nodeID string
	var createdAt time.Time
	if err := tx.QueryRow(ctx,
		`INSERT INTO game_nodes
		 (game_id, timeline_id, parent_node_id, move_uci, move_san, move_promotion,
		  board_state, is_snapshot, turn_number, created_by_user, is_check, is_checkmate, is_stalemate, captured_piece)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		 RETURNING id, created_at`,
		gameID, timelineID, parent.ID,
		uci, san, promotion,
		snapshotFEN, shouldSnapshot, turnNumber, userID,
		isCheck, isCheckmate, isStalemate, nil,
	).Scan(&nodeID, &createdAt); err != nil {
		return nil, fmt.Errorf("applyMoveAtomic insert node: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO node_children (parent_node_id, child_node_id) VALUES ($1, $2)`,
		parent.ID, nodeID,
	); err != nil {
		return nil, fmt.Errorf("applyMoveAtomic link node: %w", err)
	}

	result, shouldEnd := outcomeFromFEN(fen)
	winnerID := ""
	gameOver := false
	if shouldEnd {
		var winnerArg interface{}
		if result == "checkmate" {
			winnerID = userID
			winnerArg = userID
		}
		ct, err := tx.Exec(ctx,
			`UPDATE games
			 SET status = 'completed', winner_id = $1, result = $2, updated_at = NOW()
			 WHERE id = $3 AND status = 'active'`,
			winnerArg, result, gameID,
		)
		if err != nil {
			return nil, fmt.Errorf("applyMoveAtomic finalize: %w", err)
		}
		gameOver = ct.RowsAffected() > 0
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("applyMoveAtomic commit: %w", err)
	}

	return &moveApplyResult{
		NodeID:       nodeID,
		TimelineID:   timelineID,
		ParentNodeID: parent.ID,
		TurnNumber:   turnNumber,
		CreatedAt:    createdAt,
		GameOver:     gameOver,
		Result:       result,
		WinnerID:     winnerID,
	}, nil
}

func (s *Server) resolveTimelineParent(ctx context.Context, gameID, timelineID, parentNodeID string) (*models.GameNode, string, error) {
	var parentNode *models.GameNode

	if parentNodeID != "" {
		pn, err := db.GetNode(ctx, s.db, parentNodeID)
		if err != nil {
			return nil, "", fmt.Errorf("parent node not found: %w", err)
		}
		if pn.GameID != gameID {
			return nil, "", fmt.Errorf("parent node not in game")
		}
		parentNode = pn
		if timelineID == "" {
			timelineID = pn.TimelineID
		} else if timelineID != pn.TimelineID {
			return nil, "", fmt.Errorf("parent node not in timeline")
		}
	}

	if timelineID == "" {
		activeTimelineID, err := db.GetActiveTimelineID(ctx, s.db, gameID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read active timeline: %w", err)
		}
		if activeTimelineID != nil && *activeTimelineID != "" {
			timelineID = *activeTimelineID
		} else {
			timelines, err := db.GetGameTimelines(ctx, s.db, gameID)
			if err != nil || len(timelines) == 0 {
				return nil, "", fmt.Errorf("no timelines for game")
			}
			timelineID = timelines[0].ID
		}
	}

	if parentNode == nil {
		latest, err := db.GetLatestTimelineNode(ctx, s.db, timelineID)
		if err != nil {
			return nil, "", fmt.Errorf("latest node not found: %w", err)
		}
		parentNode = latest
	}

	return parentNode, timelineID, nil
}

func (s *Server) handleRewindMessage(c *Client, msg WSMessage) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return
	}

	nodeID, _ := payload["node_id"].(string)
	if nodeID == "" {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "rewind requires node_id"})
		return
	}

	ctx := context.Background()
	fromNode, err := db.GetNode(ctx, s.db, nodeID)
	if err != nil {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "node not found"})
		return
	}
	if fromNode.GameID != c.gameID {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "node not in game"})
		return
	}

	// Idempotency check: see if a branch root node created by this user already branches off this parent node
	var existingTimelineID, existingBranchName, existingRootNodeID string
	var existingBoardState string
	var existingTurnNumber int
	var existingCreatedAt time.Time

	err = s.db.QueryRow(ctx,
		`SELECT t.id, t.timeline_name, t.root_node_id, gn.board_state, gn.turn_number, t.created_at
		 FROM timelines t
		 INNER JOIN game_nodes gn ON t.root_node_id = gn.id
		 INNER JOIN node_children nc ON gn.id = nc.child_node_id
		 WHERE t.game_id = $1
		   AND t.created_by_user = $2
		   AND nc.parent_node_id = $3
		 LIMIT 1`,
		c.gameID, c.userID, fromNode.ID,
	).Scan(&existingTimelineID, &existingBranchName, &existingRootNodeID, &existingBoardState, &existingTurnNumber, &existingCreatedAt)

	if err == nil {
		// Branch already exists! Return and broadcast existing timeline
		s.hub.Broadcast(c.gameID, WSMessage{
			Type: "timeline_created",
			Payload: map[string]interface{}{
				"timeline_id":     existingTimelineID,
				"timeline_name":   existingBranchName,
				"root_node_id":    existingRootNodeID,
				"from_node_id":    fromNode.ID,
				"board_state":     existingBoardState,
				"turn_number":     existingTurnNumber,
				"created_by_user": c.userID,
				"created_at":      existingCreatedAt.UTC().Format(time.RFC3339),
			},
		})
		return
	}

	branchName := fmt.Sprintf("Branch T%d", fromNode.TurnNumber)
	timelineID, err := db.CreateTimeline(ctx, s.db, c.gameID, c.userID, branchName)
	if err != nil {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "failed to create timeline"})
		return
	}

	rootNodeID, err := db.CreateBranchRootNode(ctx, s.db, c.gameID, timelineID, c.userID, fromNode.BoardState, fromNode.TurnNumber)
	if err != nil {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "failed to create branch root"})
		return
	}

	if err := db.LinkNodeChild(ctx, s.db, fromNode.ID, rootNodeID); err != nil {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "failed to link branch"})
		return
	}

	s.hub.Broadcast(c.gameID, WSMessage{
		Type: "timeline_created",
		Payload: map[string]interface{}{
			"timeline_id":     timelineID,
			"timeline_name":   branchName,
			"root_node_id":    rootNodeID,
			"from_node_id":    fromNode.ID,
			"board_state":     fromNode.BoardState,
			"turn_number":     fromNode.TurnNumber,
			"created_by_user": c.userID,
			"created_at":      time.Now().UTC().Format(time.RFC3339),
		},
	})
}

func (s *Server) handleSwitchTimelineMessage(c *Client, msg WSMessage) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return
	}

	timelineID, _ := payload["timeline_id"].(string)
	if timelineID == "" {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "timeline_id required"})
		return
	}

	ctx := context.Background()
	var currentActive *string
	err := s.db.QueryRow(ctx, `SELECT active_timeline_id FROM games WHERE id = $1`, c.gameID).Scan(&currentActive)
	if err == nil && currentActive != nil && *currentActive == timelineID {
		// Already active, just broadcast to verify/sync
		s.hub.Broadcast(c.gameID, WSMessage{
			Type: "timeline_switched",
			Payload: map[string]string{
				"timeline_id": timelineID,
			},
		})
		return
	}

	if err := db.SetActiveTimelineID(ctx, s.db, c.gameID, timelineID); err != nil {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "timeline not found"})
		return
	}

	s.hub.Broadcast(c.gameID, WSMessage{
		Type: "timeline_switched",
		Payload: map[string]string{
			"timeline_id": timelineID,
		},
	})
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
