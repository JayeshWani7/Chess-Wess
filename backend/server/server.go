package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Server holds shared dependencies and implements http.Handler.
type Server struct {
	db   *pgxpool.Pool
	rdb  *redis.Client
	hub  *Hub
	mux  *http.ServeMux
}

// New creates a configured Server.
func New(pool *pgxpool.Pool, rdb *redis.Client) *Server {
	s := &Server{
		db:  pool,
		rdb: rdb,
		hub: NewHub(),
		mux: http.NewServeMux(),
	}
	go s.hub.Run()
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
