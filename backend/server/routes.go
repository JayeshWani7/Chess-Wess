package server

import "net/http"

func (s *Server) routes() {
	// CORS middleware wraps all routes
	s.mux.Handle("/api/auth/register", cors(http.HandlerFunc(s.handleRegister)))
	s.mux.Handle("/api/auth/login", cors(http.HandlerFunc(s.handleLogin)))

	s.mux.Handle("/api/games", cors(s.requireAuth(http.HandlerFunc(s.handleGames))))
	s.mux.Handle("/api/games/bot", cors(s.requireAuth(http.HandlerFunc(s.handleCreateBotGame))))
	s.mux.Handle("/api/games/history", cors(s.requireAuth(http.HandlerFunc(s.listMyGames))))
	s.mux.Handle("/api/games/", cors(s.requireAuth(http.HandlerFunc(s.handleGame))))

	s.mux.Handle("/api/users/", cors(s.requireAuth(http.HandlerFunc(s.handleGetUser))))

	// WebSocket endpoint — auth via query param token
	s.mux.Handle("/ws", cors(http.HandlerFunc(s.handleWebSocket)))

	// Health check
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
}
