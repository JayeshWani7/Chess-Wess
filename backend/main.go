package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ChessWess/backend/db"
	"github.com/ChessWess/backend/server"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if present (ignored in production where env vars are set externally)
	if err := godotenv.Load("../.env"); err != nil {
		// Try current directory too
		_ = godotenv.Load(".env")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Initialize Redis (optional for Phase 1 — used for caching in later phases)
	rdb, err := db.ConnectRedis(os.Getenv("REDIS_URL"))
	if err != nil {
		log.Printf("WARNING: Redis unavailable (%v) — continuing without cache. Install Redis for full functionality.", err)
		rdb = nil
	} else {
		defer rdb.Close()
	}

	// Build and start HTTP server
	srv := server.New(pool, rdb)
	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      srv,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("ChessWess backend listening on :%s", port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
