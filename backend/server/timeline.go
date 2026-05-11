package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ChessWess/backend/db"
)

// handleGameTimeline handles GET /api/games/{id}/timeline — returns the game's timeline tree.
func (s *Server) handleGameTimeline(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/games/"), "/")
	if len(parts) < 2 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	gameID := parts[0]

	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get all timelines for this game
	timelines, err := db.GetGameTimelines(r.Context(), s.db, gameID)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	// For each timeline, get all nodes
	type TimelineData struct {
		Timeline string      `json:"timeline_id"`
		Nodes    interface{} `json:"nodes"`
	}

	result := make([]TimelineData, len(timelines))
	for i, timeline := range timelines {
		nodes, err := db.GetTimelineNodes(r.Context(), s.db, timeline.ID)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		result[i] = TimelineData{
			Timeline: timeline.ID,
			Nodes:    nodes,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"game_id":   gameID,
		"timelines": result,
	})
}

// handleGameReplay handles GET /api/games/{id}/replay — returns the node path for replay.
// Query param: ?node_id={nodeID} to get path up to that node (defaults to latest).
func (s *Server) handleGameReplay(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/games/"), "/")
	if len(parts) < 2 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	gameID := parts[0]

	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	nodeID := r.URL.Query().Get("node_id")

	// If no node_id provided, get the latest node in the primary timeline
	if nodeID == "" {
		timelines, err := db.GetGameTimelines(r.Context(), s.db, gameID)
		if err != nil || len(timelines) == 0 {
			http.Error(w, `{"error":"no timelines found"}`, http.StatusNotFound)
			return
		}

		nodes, err := db.GetTimelineNodes(r.Context(), s.db, timelines[0].ID)
		if err != nil || len(nodes) == 0 {
			http.Error(w, `{"error":"no nodes found"}`, http.StatusNotFound)
			return
		}

		nodeID = nodes[len(nodes)-1].ID
	}

	// Get path from root to target node
	path, err := db.GetNodePath(r.Context(), s.db, nodeID)
	if err != nil {
		http.Error(w, `{"error":"node not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(path)
}

// handleNodeBranches handles GET /api/nodes/{id}/branches — returns children of a node.
func (s *Server) handleNodeBranches(w http.ResponseWriter, r *http.Request, nodeID string) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	branches, err := db.GetNodeBranches(r.Context(), s.db, nodeID)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"node_id":  nodeID,
		"branches": branches,
	})
}
