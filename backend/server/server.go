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

	// Bot tracking map to ensure only one bot engine runs per active game.
	botMu       sync.Mutex
	runningBots map[string]bool // key: gameID
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
		runningBots:    make(map[string]bool),
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

func (s *Server) StartBotIfNeeded(ctx context.Context, gameID string) {
	s.botMu.Lock()
	if s.runningBots[gameID] {
		s.botMu.Unlock()
		return
	}
	s.runningBots[gameID] = true
	s.botMu.Unlock()

	// If query or check fails, make sure to clean up the map entry so we can try again
	cleanup := func() {
		s.botMu.Lock()
		delete(s.runningBots, gameID)
		s.botMu.Unlock()
	}

	// Query DB to check if this game has a bot
	var whitePlayerID, blackPlayerID *string
	var gameStatus string
	err := s.db.QueryRow(ctx,
		`SELECT white_player_id, black_player_id, status FROM games WHERE id = $1`, gameID,
	).Scan(&whitePlayerID, &blackPlayerID, &gameStatus)
	if err != nil {
		cleanup()
		return
	}

	// Only active games should have running bots
	if gameStatus != "active" {
		cleanup()
		return
	}

	var botID string
	var botColor string
	var botRating int

	if whitePlayerID != nil {
		var isBot bool
		var rating int
		err := s.db.QueryRow(ctx,
			`SELECT is_bot, rating FROM users WHERE id = $1`, *whitePlayerID,
		).Scan(&isBot, &rating)
		if err == nil && isBot {
			botID = *whitePlayerID
			botColor = "w"
			botRating = rating
		}
	}

	if botID == "" && blackPlayerID != nil {
		var isBot bool
		var rating int
		err := s.db.QueryRow(ctx,
			`SELECT is_bot, rating FROM users WHERE id = $1`, *blackPlayerID,
		).Scan(&isBot, &rating)
		if err == nil && isBot {
			botID = *blackPlayerID
			botColor = "b"
			botRating = rating
		}
	}

	if botID == "" {
		cleanup()
		return
	}

	// Start bot engine starting from the latest state of the active timeline
	parentNode, _, err := s.resolveTimelineParent(ctx, gameID, "", "")
	if err != nil {
		cleanup()
		return
	}

	engine := NewBotEngine(s, gameID, botID, botColor, botRating)
	s.botWg.Add(1)
	go func() {
		defer s.botWg.Done()
		engine.Run(s.botCtx, parentNode.BoardState)

		s.botMu.Lock()
		delete(s.runningBots, gameID)
		s.botMu.Unlock()
	}()
}

