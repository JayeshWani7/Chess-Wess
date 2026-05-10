package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ChessWess/backend/models"
	"github.com/notnil/chess"
)

// handleGames handles GET /api/games (list open games) and POST /api/games (create game).
func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listGames(w, r)
	case http.MethodPost:
		s.createGame(w, r)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleGame handles /api/games/{id} — GET (fetch), POST /join, POST /resign.
func (s *Server) handleGame(w http.ResponseWriter, r *http.Request) {
	// Extract game ID from path: /api/games/{id}[/action]
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/games/"), "/")
	gameID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case r.Method == http.MethodGet && action == "":
		s.getGame(w, r, gameID)
	case r.Method == http.MethodGet && action == "moves":
		s.getGameMoves(w, r, gameID)
	case r.Method == http.MethodPost && action == "join":
		s.joinGame(w, r, gameID)
	case r.Method == http.MethodPost && action == "resign":
		s.resignGame(w, r, gameID)
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

func (s *Server) listGames(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(),
		`SELECT id, white_player_id, black_player_id, status, time_control, created_at, updated_at
		 FROM games WHERE status = 'pending' ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	games := []models.Game{}
	for rows.Next() {
		var g models.Game
		if err := rows.Scan(&g.ID, &g.WhitePlayerID, &g.BlackPlayerID, &g.Status, &g.TimeControl, &g.CreatedAt, &g.UpdatedAt); err != nil {
			continue
		}
		games = append(games, g)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(games)
}

type createGameRequest struct {
	TimeControl int    `json:"time_control"` // seconds; 0 = unlimited
	Color       string `json:"color"`        // "white", "black", "random"
}

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)

	var req createGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.TimeControl < 0 {
		req.TimeControl = 0
	}
	if req.Color == "" {
		req.Color = "white"
	}

	var whiteID, blackID *string
	switch req.Color {
	case "black":
		blackID = &userID
	default:
		whiteID = &userID
	}

	var g models.Game
	err := s.db.QueryRow(r.Context(),
		`INSERT INTO games (white_player_id, black_player_id, time_control)
		 VALUES ($1, $2, $3)
		 RETURNING id, white_player_id, black_player_id, status, time_control, created_at, updated_at`,
		whiteID, blackID, req.TimeControl,
	).Scan(&g.ID, &g.WhitePlayerID, &g.BlackPlayerID, &g.Status, &g.TimeControl, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(g)
}

func (s *Server) getGame(w http.ResponseWriter, r *http.Request, gameID string) {
	var g models.Game
	err := s.db.QueryRow(r.Context(),
		`SELECT id, white_player_id, black_player_id, status, time_control, winner_id, result, created_at, updated_at
		 FROM games WHERE id = $1`, gameID,
	).Scan(&g.ID, &g.WhitePlayerID, &g.BlackPlayerID, &g.Status, &g.TimeControl, &g.WinnerID, &g.Result, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"game not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(g)
}

func (s *Server) getGameMoves(w http.ResponseWriter, r *http.Request, gameID string) {
	rows, err := s.db.Query(r.Context(),
		`SELECT id, game_id, player_id, move_number, move_san, move_uci, fen_after, created_at
		 FROM game_moves WHERE game_id = $1 ORDER BY move_number ASC`, gameID)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	moves := []models.GameMove{}
	for rows.Next() {
		var m models.GameMove
		if err := rows.Scan(&m.ID, &m.GameID, &m.PlayerID, &m.MoveNumber, &m.MoveSAN, &m.MoveUCI, &m.FENAfter, &m.CreatedAt); err != nil {
			continue
		}
		moves = append(moves, m)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(moves)
}

func (s *Server) joinGame(w http.ResponseWriter, r *http.Request, gameID string) {
	userID := r.Context().Value(userIDKey).(string)

	// Find the game and determine which slot is open
	var g models.Game
	err := s.db.QueryRow(r.Context(),
		`SELECT id, white_player_id, black_player_id, status FROM games WHERE id = $1`, gameID,
	).Scan(&g.ID, &g.WhitePlayerID, &g.BlackPlayerID, &g.Status)
	if err != nil {
		http.Error(w, `{"error":"game not found"}`, http.StatusNotFound)
		return
	}
	if g.Status != models.GameStatusPending {
		http.Error(w, `{"error":"game is not joinable"}`, http.StatusConflict)
		return
	}

	var query string
	if g.WhitePlayerID == nil {
		query = `UPDATE games SET white_player_id = $1, status = 'active', updated_at = NOW() WHERE id = $2 AND white_player_id IS NULL`
	} else if g.BlackPlayerID == nil {
		query = `UPDATE games SET black_player_id = $1, status = 'active', updated_at = NOW() WHERE id = $2 AND black_player_id IS NULL`
	} else {
		http.Error(w, `{"error":"game is full"}`, http.StatusConflict)
		return
	}

	ct, err := s.db.Exec(r.Context(), query, userID, gameID)
	if err != nil || ct.RowsAffected() == 0 {
		http.Error(w, `{"error":"could not join game"}`, http.StatusConflict)
		return
	}

	// Notify room via WebSocket hub
	s.hub.Broadcast(gameID, WSMessage{Type: "player_joined", Payload: map[string]string{"user_id": userID}})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "joined", "game_id": gameID})
}

func (s *Server) resignGame(w http.ResponseWriter, r *http.Request, gameID string) {
	userID := r.Context().Value(userIDKey).(string)

	var g models.Game
	err := s.db.QueryRow(r.Context(),
		`SELECT id, white_player_id, black_player_id, status FROM games WHERE id = $1`, gameID,
	).Scan(&g.ID, &g.WhitePlayerID, &g.BlackPlayerID, &g.Status)
	if err != nil {
		http.Error(w, `{"error":"game not found"}`, http.StatusNotFound)
		return
	}
	if g.Status != models.GameStatusActive {
		http.Error(w, `{"error":"game is not active"}`, http.StatusConflict)
		return
	}

	// Determine winner
	var winnerID string
	if g.WhitePlayerID != nil && *g.WhitePlayerID == userID {
		if g.BlackPlayerID != nil {
			winnerID = *g.BlackPlayerID
		}
	} else if g.BlackPlayerID != nil && *g.BlackPlayerID == userID {
		if g.WhitePlayerID != nil {
			winnerID = *g.WhitePlayerID
		}
	} else {
		http.Error(w, `{"error":"you are not in this game"}`, http.StatusForbidden)
		return
	}

	result := "resign"
	_, err = s.db.Exec(r.Context(),
		`UPDATE games SET status = 'completed', winner_id = $1, result = $2, updated_at = NOW() WHERE id = $3`,
		winnerID, result, gameID)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	s.hub.Broadcast(gameID, WSMessage{Type: "game_over", Payload: map[string]string{"winner_id": winnerID, "result": result}})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "resigned"})
}

// ── Bot game ─────────────────────────────────────────────────────────────────

type createBotGameRequest struct {
	TimeControl int `json:"time_control"` // seconds; 0 = unlimited
	BotRating   int `json:"bot_rating"`   // 400, 600, 800, 1000, 1200, 1400, 1600
	// Color is always "white" for the human player (bot plays black).
	// Pass color = "black" to play as black (bot plays white).
	Color string `json:"color"` // "white" | "black"
}

// handleCreateBotGame creates a game against a bot and immediately starts it.
// POST /api/games/bot
func (s *Server) handleCreateBotGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value(userIDKey).(string)

	var req createBotGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.TimeControl < 0 {
		req.TimeControl = 0
	}
	if req.Color == "" {
		req.Color = "white"
	}

	// Validate bot rating
	validRatings := map[int]bool{400: true, 600: true, 800: true, 1000: true, 1200: true, 1400: true, 1600: true}
	if !validRatings[req.BotRating] {
		http.Error(w, `{"error":"invalid bot_rating; choose 400, 600, 800, 1000, 1200, 1400, or 1600"}`, http.StatusBadRequest)
		return
	}

	// Look up the bot user
	botUsername := botUsernameForRating(req.BotRating)
	var botID string
	err := s.db.QueryRow(r.Context(),
		`SELECT id FROM users WHERE username = $1 AND is_bot = TRUE`, botUsername,
	).Scan(&botID)
	if err != nil {
		http.Error(w, `{"error":"bot not found — ensure bots are seeded"}`, http.StatusInternalServerError)
		return
	}

	// Assign colors
	var whiteID, blackID string
	var botColor string
	if req.Color == "black" {
		// Human plays black, bot plays white
		whiteID = botID
		blackID = userID
		botColor = "w"
	} else {
		// Human plays white, bot plays black
		whiteID = userID
		blackID = botID
		botColor = "b"
	}

	// Create the game in 'active' state (both players are known immediately)
	var g models.Game
	err = s.db.QueryRow(r.Context(),
		`INSERT INTO games (white_player_id, black_player_id, status, time_control)
		 VALUES ($1, $2, 'active', $3)
		 RETURNING id, white_player_id, black_player_id, status, time_control, created_at, updated_at`,
		whiteID, blackID, req.TimeControl,
	).Scan(&g.ID, &g.WhitePlayerID, &g.BlackPlayerID, &g.Status, &g.TimeControl, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	// Start the bot engine in the background
	initialFEN := chess.NewGame().Position().String()
	engine := NewBotEngine(s, g.ID, botID, botColor, req.BotRating)
	go engine.Run(context.Background(), initialFEN)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(g)
}

func botUsernameForRating(rating int) string {
	switch rating {
	case 400:
		return "Bot-400"
	case 600:
		return "Bot-600"
	case 800:
		return "Bot-800"
	case 1000:
		return "Bot-1000"
	case 1200:
		return "Bot-1200"
	case 1400:
		return "Bot-1400"
	case 1600:
		return "Bot-1600"
	default:
		return "Bot-1200"
	}
}

// ── Game history ──────────────────────────────────────────────────────────────

// listMyGames returns completed games the authenticated user participated in.
// GET /api/games/history
func (s *Server) listMyGames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	userID := r.Context().Value(userIDKey).(string)

	rows, err := s.db.Query(r.Context(),
		`SELECT
		   g.id,
		   g.white_player_id,
		   g.black_player_id,
		   g.status,
		   g.time_control,
		   g.winner_id,
		   g.result,
		   g.created_at,
		   g.updated_at,
		   wu.username  AS white_username,
		   bu.username  AS black_username
		 FROM games g
		 LEFT JOIN users wu ON wu.id = g.white_player_id
		 LEFT JOIN users bu ON bu.id = g.black_player_id
		 WHERE (g.white_player_id = $1 OR g.black_player_id = $1)
		   AND g.status IN ('completed', 'abandoned')
		 ORDER BY g.updated_at DESC
		 LIMIT 50`,
		userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type gameHistoryRow struct {
		models.Game
		WhiteUsername string `json:"white_username"`
		BlackUsername string `json:"black_username"`
	}

	games := []gameHistoryRow{}
	for rows.Next() {
		var row gameHistoryRow
		if err := rows.Scan(
			&row.ID, &row.WhitePlayerID, &row.BlackPlayerID,
			&row.Status, &row.TimeControl,
			&row.WinnerID, &row.Result,
			&row.CreatedAt, &row.UpdatedAt,
			&row.WhiteUsername, &row.BlackUsername,
		); err != nil {
			continue
		}
		games = append(games, row)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(games)
}
