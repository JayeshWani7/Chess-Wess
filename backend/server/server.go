package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ChessWess/backend/observability"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// Server is the root application object.
type Server struct {
	db             *pgxpool.Pool
	rdb            *redis.Client
	hub            *Hub
	mux            *http.ServeMux
	obs            *observability.Registry
	log            *observability.Logger
	allowedOrigins []string // validated at startup via Config
	hubDone        chan struct{} // closed when hub.Run returns

	// Bot worker tracking — lets Shutdown() wait for all bots to exit.
	botWg      sync.WaitGroup
	botCancel  context.CancelFunc // cancels the shared bot context
	botCtx     context.Context
}

// New constructs the server with a validated Config.
func New(pool *pgxpool.Pool, rdb *redis.Client, cfg *Config) *Server {
	reg := prometheus.NewRegistry()
	obs := observability.New(reg)
	log := observability.NewLogger(os.Stdout)

	var origins []string
	if cfg != nil {
		origins = cfg.AllowedOrigins
	} else {
		origins = []string{"*"}
	}

	botCtx, botCancel := context.WithCancel(context.Background())

	s := &Server{
		db:             pool,
		rdb:            rdb,
		hub:            NewHub(obs),
		mux:            http.NewServeMux(),
		obs:            obs,
		log:            log,
		allowedOrigins: origins,
		hubDone:        make(chan struct{}),
		botCtx:         botCtx,
		botCancel:      botCancel,
	}

	go func() {
		s.hub.Run()
		close(s.hubDone)
	}()

	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Shutdown drains in-flight requests and stops background workers.
// Call this after http.Server.Shutdown so no new requests arrive.
func (s *Server) Shutdown() {
	// 1. Cancel all bot worker contexts — they will exit their Run loop.
	s.botCancel()

	// 2. Wait for all bot goroutines to exit, up to 10 seconds.
	botsDone := make(chan struct{})
	go func() {
		s.botWg.Wait()
		close(botsDone)
	}()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-botsDone:
	case <-timer.C:
		log.Println("shutdown: timed out waiting for bot workers to exit")
	}

	// 3. Stop the hub event loop and wait for it to drain.
	s.hub.stop <- struct{}{}
	<-s.hubDone
}
