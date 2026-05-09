package server

import (
	"encoding/json"
	"log"
	"sync"
)

// WSMessage is the envelope for all WebSocket messages.
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Client represents a connected WebSocket player.
type Client struct {
	hub    *Hub
	gameID string
	userID string
	send   chan []byte
	conn   wsConn
}

// wsConn abstracts the WebSocket connection for testability.
type wsConn interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	Close() error
}

// Hub manages all active WebSocket clients grouped by game room.
type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]map[*Client]struct{} // gameID → set of clients
	join    chan *Client
	leave   chan *Client
	message chan roomMessage
}

type roomMessage struct {
	gameID string
	data   []byte
}

// NewHub creates an initialised Hub.
func NewHub() *Hub {
	return &Hub{
		rooms:   make(map[string]map[*Client]struct{}),
		join:    make(chan *Client, 64),
		leave:   make(chan *Client, 64),
		message: make(chan roomMessage, 256),
	}
}

// Run processes hub events. Must be called in a goroutine.
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

		case c := <-h.leave:
			h.mu.Lock()
			if room, ok := h.rooms[c.gameID]; ok {
				delete(room, c)
				if len(room) == 0 {
					delete(h.rooms, c.gameID)
				}
			}
			h.mu.Unlock()
			close(c.send)

		case msg := <-h.message:
			h.mu.RLock()
			for c := range h.rooms[msg.gameID] {
				select {
				case c.send <- msg.data:
				default:
					// Slow client — drop message
					log.Printf("hub: dropping message for slow client %s", c.userID)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a WSMessage to all clients in a game room.
func (h *Hub) Broadcast(gameID string, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.message <- roomMessage{gameID: gameID, data: data}
}

// writePump drains the send channel and writes to the WebSocket.
func (c *Client) writePump() {
	defer c.conn.Close()
	for data := range c.send {
		if err := c.conn.WriteMessage(1 /* TextMessage */, data); err != nil {
			return
		}
	}
}
