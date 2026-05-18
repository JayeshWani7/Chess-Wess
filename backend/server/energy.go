package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ChessWess/backend/db"
	"github.com/ChessWess/backend/models"
)

// handleEnergyRoutes routes /api/games/{id}/energy/* endpoints
func (s *Server) handleEnergyRoutes(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/games/"), "/")
	if len(parts) < 2 || parts[1] != "energy" {
		http.Error(w, "Invalid energy route", http.StatusBadRequest)
		return
	}

	gameID := parts[0]
	requestingPlayerID := r.Context().Value(userIDKey).(string)
	action := ""

	// /api/games/{id}/energy
	// /api/games/{id}/energy/{playerID}
	// /api/games/{id}/energy/{action}
	// /api/games/{id}/energy/{playerID}/{action}
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
		// Get requesting player's energy
		s.getPlayerEnergy(w, r, gameID, requestingPlayerID)
	case r.Method == http.MethodGet && action == "" && targetPlayerID != "":
		// Get specific player's energy (for opponent display)
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

// getPlayerEnergy returns current energy level for the requesting player
// GET /api/games/{gameID}/energy
func (s *Server) getPlayerEnergy(w http.ResponseWriter, r *http.Request, gameID, playerID string) {
	ctx := r.Context()

	// Verify player is in this game
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

// getSpecificPlayerEnergy returns energy level for a specific player (allows viewing opponent's energy)
// GET /api/games/{gameID}/energy/{playerID}
func (s *Server) getSpecificPlayerEnergy(w http.ResponseWriter, r *http.Request, gameID, requestingPlayerID, targetPlayerID string) {
	ctx := r.Context()

	// Verify requesting player is in this game
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

	// Verify target player is in this game
	if (game.WhitePlayerID != nil && *game.WhitePlayerID == targetPlayerID) ||
		(game.BlackPlayerID != nil && *game.BlackPlayerID == targetPlayerID) {
		// Target player is in the game, allow fetching their energy
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

// spendEnergyRequest defines energy spending request
type spendEnergyRequest struct {
	Amount  int    `json:"amount"`
	Action  string `json:"action"` // "rewind", "jump_timeline", "lock", etc.
	Details string `json:"details"`
}

// spendEnergy deducts energy from player's pool
// POST /api/games/{gameID}/energy/spend
func (s *Server) spendEnergy(w http.ResponseWriter, r *http.Request, gameID, playerID string) {
	ctx := r.Context()

	var req spendEnergyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify player is in game
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

	// Spend the energy
	err = db.SpendEnergy(ctx, s.db, gameID, playerID, req.Amount, req.Action, req.Details)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return updated energy
	pe, err := db.GetPlayerEnergy(ctx, s.db, gameID, playerID)
	if err != nil {
		http.Error(w, "Failed to get updated energy", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pe)
}

// refundEnergyRequest defines energy refund request
type refundEnergyRequest struct {
	Amount int    `json:"amount"`
	Reason string `json:"reason"`
}

// refundEnergy returns energy to player (e.g., on invalid action)
// POST /api/games/{gameID}/energy/refund
func (s *Server) refundEnergy(w http.ResponseWriter, r *http.Request, gameID, playerID string) {
	ctx := r.Context()

	var req refundEnergyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Only allow self-refunds (host can refund players in disputes)
	// This is a simplified check; add admin logic if needed
	err := db.RefundEnergy(ctx, s.db, gameID, playerID, req.Amount, req.Reason)
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

// lockTimelineRequest defines timeline lock request
type lockTimelineRequest struct {
	TimelineID string `json:"timeline_id"`
}

// lockTimeline locks a timeline (prevents opponent rewinding into it)
// POST /api/games/{gameID}/energy/lock-timeline
func (s *Server) lockTimeline(w http.ResponseWriter, r *http.Request, gameID, playerID string) {
	ctx := r.Context()

	var req lockTimelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify player is in game
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

	// Cost: 3 energy per lock
	lockCost := 3
	err = db.SpendEnergy(ctx, s.db, gameID, playerID, lockCost, "lock_timeline", "Locked timeline "+req.TimelineID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Lock the timeline
	err = db.LockTimeline(ctx, s.db, req.TimelineID, playerID)
	if err != nil {
		// Refund energy if lock fails
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

// getTimelineStatus returns metadata about a timeline (lock status, stability, etc.)
// GET /api/games/{gameID}/energy/timeline-status?timeline_id={id}
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

	// Verify timeline belongs to this game
	if tm.GameID != gameID {
		http.Error(w, "Timeline does not belong to this game", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tm)
}

// ===== Energy System Utility Response Types =====

// EnergyStatusResponse includes player energy and timeline info
type EnergyStatusResponse struct {
	PlayerEnergy   *models.PlayerEnergy       `json:"player_energy"`
	AllTimelines   []*models.TimelineMetadata `json:"timelines"`
	TotalTimelines int                        `json:"total_timelines"`
}

// getEnergyStatus returns full energy context for a game (all player & timeline info)
// GET /api/games/{gameID}/energy/status
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

// isUUID checks if a string looks like a UUID (rough check)
func isUUID(s string) bool {
	// UUIDs are typically 36 characters with hyphens (8-4-4-4-12 format)
	// or 32 characters without hyphens
	if len(s) == 36 && strings.Count(s, "-") == 4 {
		return true
	}
	if len(s) == 32 && strings.Count(s, "-") == 0 {
		return true
	}
	return false
}
