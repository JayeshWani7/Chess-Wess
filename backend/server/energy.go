package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ChessWess/backend/db"
	"github.com/ChessWess/backend/models"
)

func (s *Server) handleEnergyRoutes(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/games/"), "/")
	if len(parts) < 2 || parts[1] != "energy" {
		http.Error(w, "Invalid energy route", http.StatusBadRequest)
		return
	}

	gameID := parts[0]
	requestingPlayerID := r.Context().Value(userIDKey).(string)
	action := ""

	var targetPlayerID string
	if len(parts) >= 3 && parts[2] != "" {
		action = parts[2]
		if isUUID(action) {
			targetPlayerID = action
			action = ""
		}
	}
	if len(parts) >= 4 && parts[3] != "" {
		if targetPlayerID == "" {
			targetPlayerID = parts[2]
		}
		action = parts[3]
	}

	switch {
	case r.Method == http.MethodGet && action == "" && targetPlayerID == "":
		s.getPlayerEnergy(w, r, gameID, requestingPlayerID)
	case r.Method == http.MethodGet && action == "" && targetPlayerID != "":
		s.getSpecificPlayerEnergy(w, r, gameID, requestingPlayerID, targetPlayerID)
	case r.Method == http.MethodPost && action == "spend":
		s.spendEnergy(w, r, gameID, requestingPlayerID)
	case r.Method == http.MethodPost && action == "refund":
		s.refundEnergy(w, r, gameID, requestingPlayerID)
	case r.Method == http.MethodPost && action == "lock-timeline":
		s.lockTimeline(w, r, gameID, requestingPlayerID)
	case r.Method == http.MethodGet && action == "timeline-status":
		s.getTimelineStatus(w, r, gameID)
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func (s *Server) getPlayerEnergy(w http.ResponseWriter, r *http.Request, gameID, playerID string) {
	ctx := r.Context()

	game, err := db.GetGame(ctx, s.db, gameID)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	if game.WhitePlayerID == nil || game.BlackPlayerID == nil ||
		(*game.WhitePlayerID != playerID && *game.BlackPlayerID != playerID) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	pe, err := db.GetPlayerEnergy(ctx, s.db, gameID, playerID)
	if err != nil {
		http.Error(w, "Failed to get energy", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pe)
}

func (s *Server) getSpecificPlayerEnergy(w http.ResponseWriter, r *http.Request, gameID, requestingPlayerID, targetPlayerID string) {
	ctx := r.Context()

	game, err := db.GetGame(ctx, s.db, gameID)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	if game.WhitePlayerID == nil || game.BlackPlayerID == nil ||
		(*game.WhitePlayerID != requestingPlayerID && *game.BlackPlayerID != requestingPlayerID) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	if (game.WhitePlayerID != nil && *game.WhitePlayerID == targetPlayerID) ||
		(game.BlackPlayerID != nil && *game.BlackPlayerID == targetPlayerID) {
	} else {
		http.Error(w, "Target player not in this game", http.StatusForbidden)
		return
	}

	pe, err := db.GetPlayerEnergy(ctx, s.db, gameID, targetPlayerID)
	if err != nil {
		http.Error(w, "Failed to get energy", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pe)
}

type spendEnergyRequest struct {
	Amount  int    `json:"amount"`
	Action  string `json:"action"`
	Details string `json:"details"`
}

func (s *Server) spendEnergy(w http.ResponseWriter, r *http.Request, gameID, playerID string) {
	ctx := r.Context()

	var req spendEnergyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	game, err := db.GetGame(ctx, s.db, gameID)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	if game.WhitePlayerID == nil || game.BlackPlayerID == nil ||
		(*game.WhitePlayerID != playerID && *game.BlackPlayerID != playerID) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	err = db.SpendEnergy(ctx, s.db, gameID, playerID, req.Amount, req.Action, req.Details)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pe, err := db.GetPlayerEnergy(ctx, s.db, gameID, playerID)
	if err != nil {
		http.Error(w, "Failed to get updated energy", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pe)
}

type refundEnergyRequest struct {
	Amount int    `json:"amount"`
	Reason string `json:"reason"`
}

func (s *Server) refundEnergy(w http.ResponseWriter, r *http.Request, gameID, playerID string) {
	ctx := r.Context()

	var req refundEnergyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	game, err := db.GetGame(ctx, s.db, gameID)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	if game.WhitePlayerID == nil || game.BlackPlayerID == nil ||
		(*game.WhitePlayerID != playerID && *game.BlackPlayerID != playerID) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	err = db.RefundEnergy(ctx, s.db, gameID, playerID, req.Amount, req.Reason)
	if err != nil {
		http.Error(w, "Failed to refund energy", http.StatusInternalServerError)
		return
	}

	pe, err := db.GetPlayerEnergy(ctx, s.db, gameID, playerID)
	if err != nil {
		http.Error(w, "Failed to get updated energy", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pe)
}

type lockTimelineRequest struct {
	TimelineID string `json:"timeline_id"`
}

func (s *Server) lockTimeline(w http.ResponseWriter, r *http.Request, gameID, playerID string) {
	ctx := r.Context()

	var req lockTimelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	game, err := db.GetGame(ctx, s.db, gameID)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	if game.WhitePlayerID == nil || game.BlackPlayerID == nil ||
		(*game.WhitePlayerID != playerID && *game.BlackPlayerID != playerID) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	lockCost := 3
	err = db.SpendEnergy(ctx, s.db, gameID, playerID, lockCost, "lock_timeline", "Locked timeline "+req.TimelineID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = db.LockTimeline(ctx, s.db, req.TimelineID, playerID)
	if err != nil {
		_ = db.RefundEnergy(ctx, s.db, gameID, playerID, lockCost, "Failed to lock timeline")
		http.Error(w, "Failed to lock timeline", http.StatusInternalServerError)
		return
	}

	tm, err := db.GetTimelineMetadata(ctx, s.db, req.TimelineID)
	if err != nil {
		http.Error(w, "Failed to get timeline status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tm)
}

func (s *Server) getTimelineStatus(w http.ResponseWriter, r *http.Request, gameID string) {
	ctx := r.Context()

	timelineID := r.URL.Query().Get("timeline_id")
	if timelineID == "" {
		http.Error(w, "Missing timeline_id query parameter", http.StatusBadRequest)
		return
	}

	tm, err := db.GetTimelineMetadata(ctx, s.db, timelineID)
	if err != nil {
		http.Error(w, "Timeline not found", http.StatusNotFound)
		return
	}

	if tm.GameID != gameID {
		http.Error(w, "Timeline does not belong to this game", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tm)
}

type EnergyStatusResponse struct {
	PlayerEnergy   *models.PlayerEnergy       `json:"player_energy"`
	AllTimelines   []*models.TimelineMetadata `json:"timelines"`
	TotalTimelines int                        `json:"total_timelines"`
}

func (s *Server) getEnergyStatus(w http.ResponseWriter, r *http.Request, gameID, playerID string) {
	ctx := r.Context()

	pe, err := db.GetPlayerEnergy(ctx, s.db, gameID, playerID)
	if err != nil {
		http.Error(w, "Failed to get player energy", http.StatusInternalServerError)
		return
	}

	timelines, err := db.GetTimelineMetadataForGame(ctx, s.db, gameID)
	if err != nil {
		http.Error(w, "Failed to get timelines", http.StatusInternalServerError)
		return
	}

	response := EnergyStatusResponse{
		PlayerEnergy:   pe,
		AllTimelines:   timelines,
		TotalTimelines: len(timelines),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func isUUID(s string) bool {
	if len(s) == 36 && strings.Count(s, "-") == 4 {
		return true
	}
	if len(s) == 32 && strings.Count(s, "-") == 0 {
		return true
	}
	return false
}
