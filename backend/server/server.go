package server

import (
	"net/http"
	"os"

	"github.com/ChessWess/backend/observability"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	db  *pgxpool.Pool
	rdb *redis.Client
	hub *Hub
	mux *http.ServeMux
	obs *observability.Registry
	log *observability.Logger
}

func New(pool *pgxpool.Pool, rdb *redis.Client) *Server {
	reg := prometheus.NewRegistry()
	obs := observability.New(reg)
	log := observability.NewLogger(os.Stdout)
	s := &Server{
		db:  pool,
		rdb: rdb,
		hub: NewHub(obs),
		mux: http.NewServeMux(),
		obs: obs,
		log: log,
	}
	go s.hub.Run()
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
