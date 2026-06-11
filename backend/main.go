package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ChessWess/backend/db"
	"github.com/ChessWess/backend/server"
	"github.com/joho/godotenv"
)

func main() {
	// ── 1. Load environment ──────────────────────────────────────────────────
	if err := godotenv.Load("../.env"); err != nil {
		_ = godotenv.Load(".env")
	}

	// ── 2. Validate required env vars — fail fast before touching the DB ─────
	cfg, err := server.LoadConfig()
	if err != nil {
		log.Fatalf("FATAL: %v\n\nSet the missing values in your .env file or environment and restart.", err)
	}

	// ── 3. Database ──────────────────────────────────────────────────────────
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer dbCancel()

	pool, err := db.Connect(dbCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(dbCtx, pool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	if err := db.SeedBots(dbCtx, pool); err != nil {
		log.Fatalf("failed to seed bots: %v", err)
	}

	// ── 4. Redis (optional) ──────────────────────────────────────────────────
	rdb, err := db.ConnectRedis(cfg.RedisURL)
	if err != nil {
		log.Printf("WARNING: Redis unavailable (%v) — continuing without cache.", err)
		rdb = nil
	} else {
		defer rdb.Close()
	}

	// ── 5. Application server ────────────────────────────────────────────────
	appSrv := server.New(pool, rdb, cfg)

	// The HTTP server must NOT have a WriteTimeout when serving WebSocket
	// connections because the WebSocket hijacks the connection and the write
	// timeout would fire on long-lived connections.  Instead we enforce per-
	// message write deadlines inside the writePump (hub.go).
	httpSrv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: appSrv,
		// ReadHeaderTimeout prevents Slowloris-style header attacks.
		ReadHeaderTimeout: 10 * time.Second,
		// ReadTimeout only applies to the HTTP portion of the connection (before
		// the WebSocket upgrade).
		ReadTimeout: 15 * time.Second,
		// No WriteTimeout — see note above.
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 16, // 64 KiB — protects against oversized headers
	}

	// ── 6. Start serving ─────────────────────────────────────────────────────
	// Run in a goroutine so main can listen for shutdown signals.
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("ChessWess backend listening on :%s", cfg.Port)
		err := httpSrv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
		close(serveErr)
	}()

	// ── 7. Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("received signal %s — shutting down", sig)
	case err := <-serveErr:
		log.Printf("server error: %v — shutting down", err)
	}

	// Give in-flight HTTP requests up to 30 seconds to complete.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	// Stop the WebSocket hub and bot workers after HTTP is drained.
	appSrv.Shutdown()

	log.Println("server shutdown complete")
}
