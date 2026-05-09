package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
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
		case "ping":
			c.send <- mustMarshal(WSMessage{Type: "pong"})
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

	if uci == "" || san == "" || fen == "" {
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "move requires uci, san, and fen"})
		return
	}

	ctx := context.Background()

	// Persist the move
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
		log.Printf("ws: failed to persist move: %v", err)
		c.send <- mustMarshal(WSMessage{Type: "error", Payload: "failed to save move"})
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

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
