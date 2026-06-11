package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/ChessWess/backend/db"
	"github.com/ChessWess/backend/models"
	"github.com/notnil/chess"
)

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

func (s *Server) listGames(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(),
		`SELECT id, white_player_id, black_player_id, status, time_control, active_timeline_id, created_at, updated_at
		 FROM games WHERE status = 'pending' ORDER BY created_at DESC LIMIT 50`)
	defer s.obs.TrackDBQuery("query", "games", time.Now(), &err)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	games := []models.Game{}
	for rows.Next() {
		var g models.Game
		if err := rows.Scan(&g.ID, &g.WhitePlayerID, &g.BlackPlayerID, &g.Status, &g.TimeControl, &g.ActiveTimelineID, &g.CreatedAt, &g.UpdatedAt); err != nil {
			continue
		}
		games = append(games, g)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(games)
}

type createGameRequest struct {
	TimeControl int    `json:"time_control"`
	Color       string `json:"color"`
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
	defer s.obs.TrackDBQuery("query", "games", time.Now(), &err)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	timelineID, err := db.CreateTimeline(r.Context(), s.db, g.ID, userID, "Mainline")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create timeline: %v"}`, err), http.StatusInternalServerError)
		return
	}

	initialFEN := chess.NewGame().Position().String()
	_, err = db.CreateRootNode(r.Context(), s.db, g.ID, timelineID, userID, initialFEN)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create root node: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if err := db.SetActiveTimelineID(r.Context(), s.db, g.ID, timelineID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to set active timeline: %v"}`, err), http.StatusInternalServerError)
		return
	}
	g.ActiveTimelineID = &timelineID

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(g)
}

func (s *Server) getGame(w http.ResponseWriter, r *http.Request, gameID string) {
	var g models.Game
	err := s.db.QueryRow(r.Context(),
		`SELECT id, white_player_id, black_player_id, status, time_control, active_timeline_id, winner_id, result, created_at, updated_at
		 FROM games WHERE id = $1`, gameID,
	).Scan(&g.ID, &g.WhitePlayerID, &g.BlackPlayerID, &g.Status, &g.TimeControl, &g.ActiveTimelineID, &g.WinnerID, &g.Result, &g.CreatedAt, &g.UpdatedAt)
	defer s.obs.TrackDBQuery("query", "games", time.Now(), &err)
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
	defer s.obs.TrackDBQuery("query", "game_moves", time.Now(), &err)
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

	var g models.Game
	err := s.db.QueryRow(r.Context(),
		`SELECT id, white_player_id, black_player_id, status FROM games WHERE id = $1`, gameID,
	).Scan(&g.ID, &g.WhitePlayerID, &g.BlackPlayerID, &g.Status)
	defer s.obs.TrackDBQuery("query", "games", time.Now(), &err)
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

	{
		var execErr error
		defer s.obs.TrackDBQuery("exec", "games", time.Now(), &execErr)
		ct, e := s.db.Exec(r.Context(), query, userID, gameID)
		execErr = e
		if execErr != nil || ct.RowsAffected() == 0 {
			http.Error(w, `{"error":"could not join game"}`, http.StatusConflict)
			return
		}
	}

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
	defer s.obs.TrackDBQuery("query", "games", time.Now(), &err)
	if err != nil {
		http.Error(w, `{"error":"game not found"}`, http.StatusNotFound)
		return
	}
	if g.Status != models.GameStatusActive {
		http.Error(w, `{"error":"game is not active"}`, http.StatusConflict)
		return
	}

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
	{
		var execErr error
		defer s.obs.TrackDBQuery("exec", "games", time.Now(), &execErr)
		_, execErr = s.db.Exec(r.Context(),
			`UPDATE games SET status = 'completed', winner_id = $1, result = $2, updated_at = NOW() WHERE id = $3`,
			winnerID, result, gameID)
		if execErr != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
	}

	s.hub.Broadcast(gameID, WSMessage{Type: "game_over", Payload: map[string]string{"winner_id": winnerID, "result": result}})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "resigned"})
}

type createBotGameRequest struct {
	TimeControl int    `json:"time_control"`
	BotRating   int    `json:"bot_rating"`
	Color       string `json:"color"`
}

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

	validRatings := map[int]bool{400: true, 600: true, 800: true, 1000: true, 1200: true, 1400: true, 1600: true}
	if !validRatings[req.BotRating] {
		http.Error(w, `{"error":"invalid bot_rating; choose 400, 600, 800, 1000, 1200, 1400, or 1600"}`, http.StatusBadRequest)
		return
	}

	botUsername := botUsernameForRating(req.BotRating)
	var botID string
	err := s.db.QueryRow(r.Context(),
		`SELECT id FROM users WHERE username = $1 AND is_bot = TRUE`, botUsername,
	).Scan(&botID)
	defer s.obs.TrackDBQuery("query", "users", time.Now(), &err)
	if err != nil {
		http.Error(w, `{"error":"bot not found — ensure bots are seeded"}`, http.StatusInternalServerError)
		return
	}

	var whiteID, blackID string
	var botColor string
	if req.Color == "black" {
		whiteID = botID
		blackID = userID
		botColor = "w"
	} else {
		whiteID = userID
		blackID = botID
		botColor = "b"
	}

	var g models.Game
	{
		var insertErr error
		defer s.obs.TrackDBQuery("query", "games", time.Now(), &insertErr)
		insertErr = s.db.QueryRow(r.Context(),
			`INSERT INTO games (white_player_id, black_player_id, status, time_control)
		 VALUES ($1, $2, 'active', $3)
		 RETURNING id, white_player_id, black_player_id, status, time_control, created_at, updated_at`,
			whiteID, blackID, req.TimeControl,
		).Scan(&g.ID, &g.WhitePlayerID, &g.BlackPlayerID, &g.Status, &g.TimeControl, &g.CreatedAt, &g.UpdatedAt)
		if insertErr != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
	}

	timelineID, err := db.CreateTimeline(r.Context(), s.db, g.ID, userID, "Mainline")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create timeline: %v"}`, err), http.StatusInternalServerError)
		return
	}

	initialFEN := chess.NewGame().Position().String()
	_, err = db.CreateRootNode(r.Context(), s.db, g.ID, timelineID, userID, initialFEN)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create root node: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if err := db.InitPlayerEnergy(r.Context(), s.db, g.ID, whiteID, blackID, 15); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to initialize energy: %v"}`, err), http.StatusInternalServerError)
		return
	}

	engine := NewBotEngine(s, g.ID, botID, botColor, req.BotRating)
	s.botWg.Add(1)
	go func() {
		defer s.botWg.Done()
		engine.Run(s.botCtx, initialFEN)
	}()

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

func (s *Server) listMyGames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	userID := r.Context().Value(userIDKey).(string)

	// Parse pagination query params: ?page=1&limit=10&filter=all|win|loss|draw
	page := 1
	limit := 10
	filterParam := r.URL.Query().Get("filter") // "all","win","loss","draw"

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	offset := (page - 1) * limit

	// Build outcome filter clause using parameterized args to avoid any injection risk.
	// $1 is already bound to userID; additional args start at $4 (after $1, $2=limit, $3=offset).
	var filterClause string
	var filterArgs []interface{}
	switch filterParam {
	case "win":
		filterClause = " AND g.winner_id = $1"
	case "loss":
		filterClause = " AND g.winner_id IS NOT NULL AND g.winner_id != $1"
	case "draw":
		filterClause = " AND g.result IN ('stalemate','draw')"
	}
	_ = filterArgs // all filter conditions reuse $1 (userID)

	baseWhere := fmt.Sprintf(
		`(g.white_player_id = $1 OR g.black_player_id = $1) AND g.status IN ('completed','abandoned')%s`,
		filterClause,
	)

	// Count total matching rows
	var total int
	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM games g WHERE %s`, baseWhere)
	countErr := s.db.QueryRow(r.Context(), countSQL, userID).Scan(&total)
	defer s.obs.TrackDBQuery("query", "games", time.Now(), &countErr)
	if countErr != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	// Fetch page
	querySQL := fmt.Sprintf(`
		SELECT
		  g.id, g.white_player_id, g.black_player_id,
		  g.status, g.time_control, g.winner_id, g.result,
		  g.created_at, g.updated_at,
		  COALESCE(wu.username,'') AS white_username,
		  COALESCE(bu.username,'') AS black_username
		FROM games g
		LEFT JOIN users wu ON wu.id = g.white_player_id
		LEFT JOIN users bu ON bu.id = g.black_player_id
		WHERE %s
		ORDER BY g.updated_at DESC
		LIMIT $2 OFFSET $3`, baseWhere)

	rows, err := s.db.Query(r.Context(), querySQL, userID, limit, offset)
	defer s.obs.TrackDBQuery("query", "games", time.Now(), &err)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type gameHistoryRow struct {
		ID            string     `json:"id"`
		WhitePlayerID *string    `json:"white_player_id"`
		BlackPlayerID *string    `json:"black_player_id"`
		Status        string     `json:"status"`
		TimeControl   int        `json:"time_control"`
		WinnerID      *string    `json:"winner_id,omitempty"`
		Result        *string    `json:"result,omitempty"`
		CreatedAt     time.Time  `json:"created_at"`
		UpdatedAt     time.Time  `json:"updated_at"`
		WhiteUsername string     `json:"white_username"`
		BlackUsername string     `json:"black_username"`
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
			log.Printf("listMyGames scan error: %v", err)
			continue
		}
		games = append(games, row)
	}
	if err := rows.Err(); err != nil {
		log.Printf("listMyGames rows error: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"games": games,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}
