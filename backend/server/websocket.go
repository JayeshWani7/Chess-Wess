package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ChessWess/backend/db"
	"github.com/ChessWess/backend/models"
	"github.com/gorilla/websocket"
	"github.com/notnil/chess"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins in development; restrict in production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// handleWebSocket upgrades the connection and registers the client with the hub.
// Query params: token (JWT), game_id
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

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:    s.hub,
		gameID: gameID,
		userID: userID,
		send:   make(chan []byte, 64),
		conn:   conn,
	}

	s.hub.join <- client
	go client.writePump()

	// Announce presence
	s.hub.Broadcast(gameID, WSMessage{
		Type:    "player_connected",
		Payload: map[string]string{"user_id": userID},
	})

	// Read loop — blocks until client disconnects
	s.readPump(client)
}

// readPump reads incoming messages from the client and dispatches them.
func (s *Server) readPump(c *Client) {
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
			// Ignore pong messages (keepalive response from client)
		default:
			log.Printf("ws: unknown message type %q from %s", msg.Type, c.userID)
		}
	}
}

// handleMoveMessage processes a chess move sent over WebSocket.
// Expected payload: { "uci": "e2e4", "san": "e4", "fen": "<fen after move>" }
func (s *Server) handleMoveMessage(c *Client, msg WSMessage) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return
	}

	uci, _ := payload["uci"].(string)
	san, _ := payload["san"].(string)
	fen, _ := payload["fen"].(string)
	timelineID, _ := payload["timeline_id"].(string)
	parentNodeID, _ := payload["parent_node_id"].(string)

	if uci == "" || san == "" || fen == "" {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "move requires uci, san, and fen"})
		return
	}

	ctx := context.Background()

	// Extract promotion piece from UCI if present (e2e8q format)
	promotion := ""
	if len(uci) > 4 {
		promotion = string(uci[4])
	}

	// Persist the move (legacy game_moves for backwards compatibility)
	var moveID string
	err := s.db.QueryRow(
		ctx,
		`INSERT INTO game_moves (game_id, player_id, move_number, move_san, move_uci, fen_after)
		 VALUES ($1, $2,
		   (SELECT COALESCE(MAX(move_number), 0) + 1 FROM game_moves WHERE game_id = $1),
		   $3, $4, $5)
		 RETURNING id`,
		c.gameID, c.userID, san, uci, fen,
	).Scan(&moveID)
	if err != nil {
		log.Printf("ws: failed to persist game_move: %v", err)
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "failed to save move"})
		return
	}

	// Phase 2: Create game node for timeline system
	_, err = s.createGameNode(ctx, c.gameID, c.userID, uci, san, promotion, fen, timelineID, parentNodeID)
	if err != nil {
		log.Printf("ws: failed to create game node: %v", err)
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "failed to save timeline move"})
		return
	}

	// Broadcast the move to all players in the room
	s.hub.Broadcast(c.gameID, WSMessage{
		Type: "move",
		Payload: map[string]interface{}{
			"id":        moveID,
			"player_id": c.userID,
			"uci":       uci,
			"san":       san,
			"fen":       fen,
		},
	})
}

// createGameNode creates a game node for the timeline system.
// If timelineID or parentNodeID are empty, they are resolved from the latest node.
func (s *Server) createGameNode(ctx context.Context, gameID, userID, uci, san, promotion, fen, timelineID, parentNodeID string) (string, error) {
	var parentNode *models.GameNode

	if parentNodeID != "" {
		pn, err := db.GetNode(ctx, s.db, parentNodeID)
		if err != nil {
			return "", fmt.Errorf("createGameNode: parent node not found: %w", err)
		}
		if pn.GameID != gameID {
			return "", fmt.Errorf("createGameNode: parent node not in game")
		}
		parentNode = pn
		if timelineID == "" {
			timelineID = pn.TimelineID
		} else if timelineID != pn.TimelineID {
			return "", fmt.Errorf("createGameNode: parent node not in timeline")
		}
	}

	if timelineID == "" {
		activeTimelineID, err := db.GetActiveTimelineID(ctx, s.db, gameID)
		if err != nil {
			return "", fmt.Errorf("createGameNode: failed to read active timeline: %w", err)
		}
		if activeTimelineID != nil && *activeTimelineID != "" {
			timelineID = *activeTimelineID
		} else {
			timelines, err := db.GetGameTimelines(ctx, s.db, gameID)
			if err != nil || len(timelines) == 0 {
				return "", fmt.Errorf("createGameNode: no timelines for game")
			}
			timelineID = timelines[0].ID
		}
	}

	if parentNode == nil {
		latest, err := db.GetLatestTimelineNode(ctx, s.db, timelineID)
		if err != nil {
			return "", fmt.Errorf("createGameNode: latest node not found: %w", err)
		}
		parentNode = latest
	}

	// Get metadata from the board position
	var isCheck, isCheckmate, isStalemate bool

	// Parse the new position to determine game state
	fenOpt, err := chess.FEN(fen)
	if err == nil {
		game := chess.NewGame(fenOpt)
		pos := game.Position()
		status := pos.Status()

		// Determine game state from status
		isCheckmate = status == chess.Checkmate
		isStalemate = status == chess.Stalemate

		// Check if in check: has valid moves but is checkmate, or use opponent's perspective
		// If checkmate, it's also in check. If stalemate, it's not in check.
		if isCheckmate {
			isCheck = true
		} else if !isStalemate && status == chess.NoMethod {
			// If no moves are valid and it's not checkmate/stalemate, something is wrong
			// For now, assume check if there are no valid moves but not checkmate/stalemate
			isCheck = len(game.ValidMoves()) == 0
		}
	}

	// Create the node
	resolvedParentID := parentNode.ID
	nodeData := &models.GameNode{
		GameID:        gameID,
		TimelineID:    timelineID,
		ParentNodeID:  &resolvedParentID,
		Move:          &models.Move{UCI: uci, SAN: san, Promotion: promotion},
		BoardState:    fen,
		TurnNumber:    parentNode.TurnNumber + 1,
		CreatedByUser: userID,
		Metadata: models.GameNodeMetadata{
			Check:     isCheck,
			Checkmate: isCheckmate,
			Stalemate: isStalemate,
		},
	}

	return db.CreateNode(ctx, s.db, nodeData, resolvedParentID)
}

// handleRewindMessage creates a new timeline branching from a prior node.
// Expected payload: { "node_id": "<target_node_id>" }
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

	timelineID, err := db.CreateTimeline(ctx, s.db, c.gameID, c.userID)
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
			"timeline_id":  timelineID,
			"root_node_id": rootNodeID,
			"from_node_id": fromNode.ID,
			"board_state":  fromNode.BoardState,
			"turn_number":  fromNode.TurnNumber,
		},
	})
}

// handleSwitchTimelineMessage sets the active timeline for the game.
// Expected payload: { "timeline_id": "<timeline_id>" }
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
