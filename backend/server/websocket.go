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
	CheckOrigin:     func(r *http.Request) bool { return true },
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

	s.hub.Broadcast(gameID, WSMessage{
		Type:    "player_connected",
		Payload: map[string]string{"user_id": userID},
	})

	s.readPump(client)
}

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
		default:
			log.Printf("ws: unknown message type %q from %s", msg.Type, c.userID)
		}
	}
}

func (s *Server) handleMoveMessage(c *Client, msg WSMessage) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return
	}

	uci, _ := payload["uci"].(string)
	timelineID, _ := payload["timeline_id"].(string)
	parentNodeID, _ := payload["parent_node_id"].(string)

	if uci == "" {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "move requires uci"})
		return
	}

	ctx := context.Background()

	parentNode, resolvedTimelineID, err := s.resolveTimelineParent(ctx, c.gameID, timelineID, parentNodeID)
	if err != nil {
		log.Printf("ws: failed to resolve timeline parent: %v", err)
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

	nodeID, err := s.createGameNode(ctx, c.gameID, c.userID, uci, san, promotion, fen, resolvedTimelineID, parentNode.ID)
	if err != nil {
		log.Printf("ws: failed to create game node: %v", err)
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "failed to save timeline move"})
		return
	}

	if result, shouldEnd := outcomeFromFEN(fen); shouldEnd {
		var winnerArg interface{}
		winnerID := ""
		if result == "checkmate" {
			winnerID = c.userID
			winnerArg = c.userID
		}
		ct, err := s.db.Exec(ctx,
			`UPDATE games
			 SET status = 'completed', winner_id = $1, result = $2, updated_at = NOW()
			 WHERE id = $3 AND status = 'active'`,
			winnerArg, result, c.gameID,
		)
		if err != nil {
			log.Printf("ws: failed to finalize game: %v", err)
		} else if ct.RowsAffected() > 0 {
			s.hub.Broadcast(c.gameID, WSMessage{
				Type:    "game_over",
				Payload: map[string]string{"winner_id": winnerID, "result": result},
			})
		}
	}

	s.hub.Broadcast(c.gameID, WSMessage{
		Type: "move",
		Payload: map[string]interface{}{
			"id":        nodeID,
			"player_id": c.userID,
			"uci":       uci,
			"san":       san,
			"fen":       fen,
		},
	})
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
	nodeData := &models.GameNode{
		GameID:        gameID,
		TimelineID:    resolvedTimelineID,
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
			"timeline_id":  timelineID,
			"root_node_id": rootNodeID,
			"from_node_id": fromNode.ID,
			"board_state":  fromNode.BoardState,
			"turn_number":  fromNode.TurnNumber,
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
