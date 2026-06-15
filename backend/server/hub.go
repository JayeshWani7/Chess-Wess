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
	Seq     uint64      `json:"seq,omitempty"`
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
	lastSeq          uint64
	hasLastSeq       bool
	isPlayer         bool
}

// wsConn is the interface satisfied by both *websocket.Conn and the bot nullConn.
type wsConn interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	Close() error
}

type GameRoom struct {
	clients map[*Client]struct{}
	seq     uint64
	history []WSMessage
}

// Hub manages all connected clients, grouped by game room.
type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]*GameRoom
	join    chan *Client
	leave   chan *Client
	message chan roomMessage
	stop    chan struct{}
	obs     *observability.Registry
}

type roomMessage struct {
	gameID string
	msg    WSMessage
}

// NewHub creates and returns an uninitialised Hub; call Run() in a goroutine.
func NewHub(obs *observability.Registry) *Hub {
	return &Hub{
		rooms:   make(map[string]*GameRoom),
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
				for c := range room.clients {
					close(c.send)
				}
			}
			h.rooms = make(map[string]*GameRoom)
			h.mu.Unlock()
			return

		case c := <-h.join:
			h.mu.Lock()
			room := h.getOrCreateRoom(c.gameID)
			room.clients[c] = struct{}{}
			if c.hasLastSeq {
				h.recoverClient(c, room)
			}
			h.mu.Unlock()
			if h.obs != nil {
				h.obs.RecordWSConnect()
			}

		case c := <-h.leave:
			h.mu.Lock()
			if room, ok := h.rooms[c.gameID]; ok {
				delete(room.clients, c)
				if len(room.clients) == 0 {
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

		case rMsg := <-h.message:
			h.mu.Lock()
			room := h.getOrCreateRoom(rMsg.gameID)
			room.seq++
			rMsg.msg.Seq = room.seq

			data, err := json.Marshal(rMsg.msg)
			if err != nil {
				log.Printf("hub: marshal error: %v", err)
				h.mu.Unlock()
				continue
			}

			room.history = append(room.history, rMsg.msg)
			if len(room.history) > 500 {
				copy(room.history, room.history[1:])
				room.history = room.history[:500]
			}

			for c := range room.clients {
				select {
				case c.send <- data:
				default:
					log.Printf("hub: slow client detected, disconnecting %s", c.userID)
					go c.CloseWithReason(4000, "slow_client")
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) getOrCreateRoom(gameID string) *GameRoom {
	room, ok := h.rooms[gameID]
	if !ok {
		room = &GameRoom{
			clients: make(map[*Client]struct{}),
			history: make([]WSMessage, 0, 200),
		}
		h.rooms[gameID] = room
	}
	return room
}

func (h *Hub) recoverClient(c *Client, room *GameRoom) {
	if c.lastSeq == room.seq {
		return
	}
	if len(room.history) == 0 {
		h.sendResync(c)
		return
	}
	firstSeq := room.history[0].Seq
	if c.lastSeq < firstSeq-1 {
		h.sendResync(c)
		return
	}
	for _, msg := range room.history {
		if msg.Seq > c.lastSeq {
			data, err := json.Marshal(msg)
			if err == nil {
				select {
				case c.send <- data:
				default:
					log.Printf("hub: failed to replay message to client %s (buffer full)", c.userID)
				}
			}
		}
	}
}

func (h *Hub) sendResync(c *Client) {
	msg := WSMessage{
		Type:    "resync",
		Payload: "state out of sync",
	}
	data, _ := json.Marshal(msg)
	select {
	case c.send <- data:
	default:
	}
}

func (c *Client) CloseWithReason(code int, reason string) {
	c.disconnectReason = reason
	if wc, ok := c.conn.(*websocket.Conn); ok {
		msg := websocket.FormatCloseMessage(code, reason)
		_ = wc.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
	}
	_ = c.conn.Close()
}

// ActiveConnections returns the total number of connected clients.
func (h *Hub) ActiveConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, room := range h.rooms {
		total += len(room.clients)
	}
	return total
}

// Broadcast sends msg to every client in the named game room.
func (h *Hub) Broadcast(gameID string, msg WSMessage) {
	h.message <- roomMessage{gameID: gameID, msg: msg}
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
