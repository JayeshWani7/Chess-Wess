package server

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/ChessWess/backend/observability"
	"github.com/gorilla/websocket"
)

// WSMessage is the envelope used for every WebSocket message.
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Client represents one connected WebSocket peer.
type Client struct {
	hub              *Hub
	gameID           string
	userID           string
	send             chan []byte
	conn             wsConn
	disconnectReason string
}

// wsConn is the interface satisfied by both *websocket.Conn and the bot nullConn.
type wsConn interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	Close() error
}

// Hub manages all connected clients, grouped by game room.
type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]map[*Client]struct{}
	join    chan *Client
	leave   chan *Client
	message chan roomMessage
	stop    chan struct{}
	obs     *observability.Registry
}

type roomMessage struct {
	gameID string
	data   []byte
}

// NewHub creates and returns an uninitialised Hub; call Run() in a goroutine.
func NewHub(obs *observability.Registry) *Hub {
	return &Hub{
		rooms:   make(map[string]map[*Client]struct{}),
		join:    make(chan *Client, 64),
		leave:   make(chan *Client, 64),
		message: make(chan roomMessage, 256),
		stop:    make(chan struct{}, 1),
		obs:     obs,
	}
}

// Run is the single-goroutine event loop for the hub.
// It returns (and the caller should close hubDone) when stop is signalled.
func (h *Hub) Run() {
	for {
		select {
		case <-h.stop:
			// Close all client send channels so writePumps exit cleanly.
			h.mu.Lock()
			for _, room := range h.rooms {
				for c := range room {
					close(c.send)
				}
			}
			h.rooms = make(map[string]map[*Client]struct{})
			h.mu.Unlock()
			return

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
			// Only close the channel if it has not already been closed by the
			// stop path above.
			select {
			case _, open := <-c.send:
				if open {
					close(c.send)
				}
			default:
				close(c.send)
			}

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

// ActiveConnections returns the total number of connected clients.
func (h *Hub) ActiveConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, room := range h.rooms {
		total += len(room)
	}
	return total
}

// Broadcast sends msg to every client in the named game room.
func (h *Hub) Broadcast(gameID string, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.message <- roomMessage{gameID: gameID, data: data}
}

// --------------------------------------------------------------------------
// Per-client write pump with ping/pong keepalive
// --------------------------------------------------------------------------

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period (must be less than pongWait).
	pingPeriod = (pongWait * 9) / 10
)

// writePump pumps messages from the send channel to the WebSocket connection.
// It also sends periodic pings so the connection is kept alive and dead peers
// are detected within pongWait seconds.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	// Set up pong handler to keep the read deadline moving.
	// We can only do this for real WebSocket connections, not the bot nullConn.
	if wc, ok := c.conn.(*websocket.Conn); ok {
		wc.SetReadDeadline(time.Now().Add(pongWait))
		wc.SetPongHandler(func(string) error {
			wc.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})
	}

	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				// Channel was closed — send close frame.
				if wc, ok2 := c.conn.(*websocket.Conn); ok2 {
					wc.SetWriteDeadline(time.Now().Add(writeWait))
					wc.WriteMessage(websocket.CloseMessage, []byte{})
				}
				return
			}
			if wc, ok2 := c.conn.(*websocket.Conn); ok2 {
				wc.SetWriteDeadline(time.Now().Add(writeWait))
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.disconnectReason = "error"
				return
			}

		case <-ticker.C:
			if wc, ok := c.conn.(*websocket.Conn); ok {
				wc.SetWriteDeadline(time.Now().Add(writeWait))
				if err := wc.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}
}
