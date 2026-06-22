package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ChessWess/backend/db"
	"github.com/ChessWess/backend/models"
)

func (s *Server) handleGameTimeline(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/games/"), "/")
	if len(parts) < 2 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	gameID := parts[0]
	action := ""
	if len(parts) > 2 {
		action = parts[2]
	}

	if action == "active" {
		s.handleActiveTimeline(w, r, gameID)
		return
	}

	if r.Method == http.MethodPost {
		userID := r.Context().Value(userIDKey).(string)
		game, err := db.GetGame(r.Context(), s.db, gameID)
		if err != nil {
			http.Error(w, `{"error":"game not found"}`, http.StatusNotFound)
			return
		}
		if game.WhitePlayerID == nil || game.BlackPlayerID == nil ||
			(*game.WhitePlayerID != userID && *game.BlackPlayerID != userID) {
			http.Error(w, `{"error":"not authorized"}`, http.StatusForbidden)
			return
		}

		var payload struct {
			TimelineID   string `json:"timeline_id"`
			TimelineName string `json:"timeline_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		payload.TimelineName = strings.TrimSpace(payload.TimelineName)
		if payload.TimelineID == "" || payload.TimelineName == "" {
			http.Error(w, `{"error":"timeline_id and timeline_name required"}`, http.StatusBadRequest)
			return
		}
		if err := db.UpdateTimelineName(r.Context(), s.db, gameID, payload.TimelineID, payload.TimelineName); err != nil {
			http.Error(w, `{"error":"timeline not found"}`, http.StatusNotFound)
			return
		}
		if s.rdb != nil {
			_ = s.rdb.Del(r.Context(), "game:"+gameID+":timeline").Err()
		}
		s.hub.Broadcast(gameID, WSMessage{
			Type: "timeline_renamed",
			Payload: map[string]string{
				"timeline_id":   payload.TimelineID,
				"timeline_name": payload.TimelineName,
			},
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"game_id":       gameID,
			"timeline_id":   payload.TimelineID,
			"timeline_name": payload.TimelineName,
		})
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var nodeLimit int
	if rawLimit := r.URL.Query().Get("node_limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			http.Error(w, `{"error":"invalid node_limit"}`, http.StatusBadRequest)
			return
		}
		if parsed > 2000 {
			parsed = 2000
		}
		nodeLimit = parsed
	}

	var cacheKey string
	if s.rdb != nil && nodeLimit == 0 {
		cacheKey = "game:" + gameID + ":timeline"
		if cachedVal, err := s.rdb.Get(r.Context(), cacheKey).Result(); err == nil && cachedVal != "" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(cachedVal))
			return
		}
	}

	timelines, err := db.GetGameTimelines(r.Context(), s.db, gameID)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	activeTimelineID, err := db.GetActiveTimelineID(r.Context(), s.db, gameID)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	type TimelineData struct {
		Timeline     string            `json:"timeline_id"`
		TimelineName string            `json:"timeline_name"`
		Nodes        []models.GameNode `json:"nodes"`
		NodeCount    int               `json:"node_count"`
		NodesPartial bool              `json:"nodes_partial"`
	}

	result := make([]TimelineData, len(timelines))
	for i, timeline := range timelines {
		nodeCount, err := db.GetTimelineNodeCount(r.Context(), s.db, timeline.ID)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}

		var nodes []models.GameNode
		if nodeLimit > 0 {
			nodes, err = db.GetTimelineNodesWindow(r.Context(), s.db, timeline.ID, nodeLimit)
		} else {
			nodes, err = db.GetTimelineNodes(r.Context(), s.db, timeline.ID)
		}
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}

		result[i] = TimelineData{
			Timeline:     timeline.ID,
			TimelineName: timeline.TimelineName,
			Nodes:        nodes,
			NodeCount:    nodeCount,
			NodesPartial: nodeLimit > 0 && nodeCount > len(nodes),
		}
	}

	merges, err := db.GetGameMerges(r.Context(), s.db, gameID)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	responseObj := map[string]interface{}{
		"game_id":            gameID,
		"active_timeline_id": activeTimelineID,
		"timelines":          result,
		"merges":             merges,
	}

	responseBytes, err := json.Marshal(responseObj)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	if cacheKey != "" {
		_ = s.rdb.Set(r.Context(), cacheKey, responseBytes, 2*time.Hour).Err()
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(responseBytes)
}

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

	if nodeID == "" {
		timelineID, err := db.GetActiveTimelineID(r.Context(), s.db, gameID)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		if timelineID == nil || *timelineID == "" {
			timelines, err := db.GetGameTimelines(r.Context(), s.db, gameID)
			if err != nil || len(timelines) == 0 {
				http.Error(w, `{"error":"no timelines found"}`, http.StatusNotFound)
				return
			}
			timelineID = &timelines[0].ID
		}

		latest, err := db.GetLatestTimelineNode(r.Context(), s.db, *timelineID)
		if err != nil {
			http.Error(w, `{"error":"no nodes found"}`, http.StatusNotFound)
			return
		}
		nodeID = latest.ID
	}

	path, err := db.GetNodePath(r.Context(), s.db, nodeID)
	if err != nil {
		http.Error(w, `{"error":"node not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(path)
}

func (s *Server) handleActiveTimeline(w http.ResponseWriter, r *http.Request, gameID string) {
	switch r.Method {
	case http.MethodGet:
		activeTimelineID, err := db.GetActiveTimelineID(r.Context(), s.db, gameID)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"game_id":            gameID,
			"active_timeline_id": activeTimelineID,
		})
	case http.MethodPost:
		userID := r.Context().Value(userIDKey).(string)
		game, err := db.GetGame(r.Context(), s.db, gameID)
		if err != nil {
			http.Error(w, `{"error":"game not found"}`, http.StatusNotFound)
			return
		}
		if game.WhitePlayerID == nil || game.BlackPlayerID == nil ||
			(*game.WhitePlayerID != userID && *game.BlackPlayerID != userID) {
			http.Error(w, `{"error":"not authorized"}`, http.StatusForbidden)
			return
		}

		var payload struct {
			TimelineID string `json:"timeline_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.TimelineID == "" {
			http.Error(w, `{"error":"timeline_id required"}`, http.StatusBadRequest)
			return
		}
		if err := db.SetActiveTimelineID(r.Context(), s.db, gameID, payload.TimelineID); err != nil {
			http.Error(w, `{"error":"timeline not found"}`, http.StatusNotFound)
			return
		}
		if s.rdb != nil {
			_ = s.rdb.Del(r.Context(), "game:"+gameID+":timeline").Err()
		}
		s.hub.Broadcast(gameID, WSMessage{
			Type: "timeline_switched",
			Payload: map[string]string{
				"timeline_id": payload.TimelineID,
			},
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"game_id":            gameID,
			"active_timeline_id": payload.TimelineID,
		})
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

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

func (s *Server) handleMergeTimelines(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	userID := r.Context().Value(userIDKey).(string)
	game, err := db.GetGame(r.Context(), s.db, gameID)
	if err != nil {
		http.Error(w, `{"error":"game not found"}`, http.StatusNotFound)
		return
	}
	if game.WhitePlayerID == nil || game.BlackPlayerID == nil ||
		(*game.WhitePlayerID != userID && *game.BlackPlayerID != userID) {
		http.Error(w, `{"error":"not authorized"}`, http.StatusForbidden)
		return
	}

	var payload struct {
		SourceNodeID string `json:"source_node_id"`
		TargetNodeID string `json:"target_node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.SourceNodeID == "" || payload.TargetNodeID == "" {
		http.Error(w, `{"error":"source_node_id and target_node_id required"}`, http.StatusBadRequest)
		return
	}

	sourceNode, err := db.GetNode(r.Context(), s.db, payload.SourceNodeID)
	if err != nil || sourceNode.GameID != gameID {
		http.Error(w, `{"error":"source node not found in game"}`, http.StatusBadRequest)
		return
	}
	targetNode, err := db.GetNode(r.Context(), s.db, payload.TargetNodeID)
	if err != nil || targetNode.GameID != gameID {
		http.Error(w, `{"error":"target node not found in game"}`, http.StatusBadRequest)
		return
	}

	if sourceNode.BoardState != targetNode.BoardState {
		http.Error(w, `{"error":"board states do not match"}`, http.StatusBadRequest)
		return
	}

	err = db.CreateMerge(r.Context(), s.db, gameID, payload.SourceNodeID, payload.TargetNodeID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to merge: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Invalidate timeline cache
	if s.rdb != nil {
		_ = s.rdb.Del(r.Context(), "game:"+gameID+":timeline").Err()
	}

	s.hub.Broadcast(gameID, WSMessage{
		Type: "timeline_merged",
		Payload: map[string]string{
			"source_node_id": payload.SourceNodeID,
			"target_node_id": payload.TargetNodeID,
		},
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
