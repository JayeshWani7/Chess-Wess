package server

import (
	"net/http"
	"strings"

	"github.com/ChessWess/backend/observability"
)

func (s *Server) routes() {
	// Auth endpoints: rate-limited + body-size capped.
	s.mux.Handle("/api/auth/register",
		s.cors(
			rateLimitAuth(
				maxBodyBytes(maxRequestBodyBytes,
					observability.InstrumentHandler(s.obs, "/api/auth/register",
						http.HandlerFunc(s.handleRegister))))))

	s.mux.Handle("/api/auth/login",
		s.cors(
			rateLimitAuth(
				maxBodyBytes(maxRequestBodyBytes,
					observability.InstrumentHandler(s.obs, "/api/auth/login",
						http.HandlerFunc(s.handleLogin))))))

	// Game REST endpoints: body-size capped.
	s.mux.Handle("/api/games",
		s.cors(
			s.requireAuth(
				maxBodyBytes(maxRequestBodyBytes,
					observability.InstrumentHandler(s.obs, "/api/games",
						http.HandlerFunc(s.handleGames))))))

	s.mux.Handle("/api/games/bot",
		s.cors(
			s.requireAuth(
				maxBodyBytes(maxRequestBodyBytes,
					http.HandlerFunc(s.handleCreateBotGame)))))

	s.mux.Handle("/api/games/history",
		s.cors(
			s.requireAuth(
				http.HandlerFunc(s.listMyGames))))

	s.mux.Handle("/api/games/",
		s.cors(
			s.requireAuth(
				maxBodyBytes(maxRequestBodyBytes,
					http.HandlerFunc(s.handleGameRoutes)))))

	s.mux.Handle("/api/users/",
		s.cors(
			s.requireAuth(
				http.HandlerFunc(s.handleGetUser))))

	s.mux.Handle("/api/nodes/",
		s.cors(
			s.requireAuth(
				http.HandlerFunc(s.handleNodeRoutes))))

	// WebSocket: rate-limited at the connection-upgrade level.
	s.mux.Handle("/ws",
		s.cors(
			rateLimitWS(
				observability.InstrumentHandler(s.obs, "/ws",
					http.HandlerFunc(s.handleWebSocket)))))

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
