package server

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/ChessWess/backend/observability"
)

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type Client struct {
	hub              *Hub
	gameID           string
	userID           string
	send             chan []byte
	conn             wsConn
	disconnectReason string
}

type wsConn interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	Close() error
}

type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]map[*Client]struct{}
	join    chan *Client
	leave   chan *Client
	message chan roomMessage
	obs     *observability.Registry
}

type roomMessage struct {
	gameID string
	data   []byte
}

func NewHub(obs *observability.Registry) *Hub {
	return &Hub{
		rooms:   make(map[string]map[*Client]struct{}),
		join:    make(chan *Client, 64),
		leave:   make(chan *Client, 64),
		message: make(chan roomMessage, 256),
		obs:     obs,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.join:
			h.mu.Lock()
			if h.rooms[c.gameID] == nil {
				h.rooms[c.gameID] = make(map[*Client]struct{})
			}
			h.rooms[c.gameID][c] = struct{}{}
			h.mu.Unlock()
			if h.obs != nil {
				h.obs.RecordWSConnect()
			}

		case c := <-h.leave:
			h.mu.Lock()
			if room, ok := h.rooms[c.gameID]; ok {
				delete(room, c)
				if len(room) == 0 {
					delete(h.rooms, c.gameID)
				}
			}
			h.mu.Unlock()
			if h.obs != nil {
				h.obs.RecordWSDisconnect(c.disconnectReason)
			}
			close(c.send)

		case msg := <-h.message:
			h.mu.RLock()
			for c := range h.rooms[msg.gameID] {
				select {
				case c.send <- msg.data:
				default:
					log.Printf("hub: dropping message for slow client %s", c.userID)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// ActiveConnections returns the total number of clients connected across all game rooms.
// It is safe to call concurrently with join/leave operations.
func (h *Hub) ActiveConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, room := range h.rooms {
		total += len(room)
	}
	return total
}

func (h *Hub) Broadcast(gameID string, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.message <- roomMessage{gameID: gameID, data: data}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for data := range c.send {
		if err := c.conn.WriteMessage(1, data); err != nil {
			c.disconnectReason = "error"
			return
		}
	}
}
