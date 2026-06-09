package server

import (
	"net/http"
	"strings"

	"github.com/ChessWess/backend/observability"
)

func (s *Server) routes() {
	s.mux.Handle("/api/auth/register", cors(observability.InstrumentHandler(s.obs, "/api/auth/register", http.HandlerFunc(s.handleRegister))))
	s.mux.Handle("/api/auth/login", cors(observability.InstrumentHandler(s.obs, "/api/auth/login", http.HandlerFunc(s.handleLogin))))

	s.mux.Handle("/api/games", cors(s.requireAuth(observability.InstrumentHandler(s.obs, "/api/games", http.HandlerFunc(s.handleGames)))))
	s.mux.Handle("/api/games/bot", cors(s.requireAuth(http.HandlerFunc(s.handleCreateBotGame))))
	s.mux.Handle("/api/games/history", cors(s.requireAuth(http.HandlerFunc(s.listMyGames))))
	s.mux.Handle("/api/games/", cors(s.requireAuth(http.HandlerFunc(s.handleGameRoutes))))

	s.mux.Handle("/api/users/", cors(s.requireAuth(http.HandlerFunc(s.handleGetUser))))

	s.mux.Handle("/api/nodes/", cors(s.requireAuth(http.HandlerFunc(s.handleNodeRoutes))))

	s.mux.Handle("/ws", cors(observability.InstrumentHandler(s.obs, "/ws", http.HandlerFunc(s.handleWebSocket))))

	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.Handle("/metrics", s.obs.Handler())
}

func (s *Server) handleGameRoutes(w http.ResponseWriter, r *http.Request) {
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
	case r.Method == http.MethodGet && action == "timeline":
		s.handleGameTimeline(w, r)
	case r.Method == http.MethodPost && action == "timeline":
		s.handleGameTimeline(w, r)
	case r.Method == http.MethodGet && action == "replay":
		s.handleGameReplay(w, r)
	case r.Method == http.MethodPost && action == "join":
		s.joinGame(w, r, gameID)
	case r.Method == http.MethodPost && action == "resign":
		s.resignGame(w, r, gameID)
	case action == "energy":
		s.handleEnergyRoutes(w, r)
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

func (s *Server) handleNodeRoutes(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/nodes/"), "/")
	nodeID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case r.Method == http.MethodGet && action == "branches":
		s.handleNodeBranches(w, r, nodeID)
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}
