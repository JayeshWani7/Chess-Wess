package db

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type BotDef struct {
	Username string
	Rating   int
}

var DefaultBots = []BotDef{
	{Username: "Bot-400", Rating: 400},
	{Username: "Bot-600", Rating: 600},
	{Username: "Bot-800", Rating: 800},
	{Username: "Bot-1000", Rating: 1000},
	{Username: "Bot-1200", Rating: 1200},
	{Username: "Bot-1400", Rating: 1400},
	{Username: "Bot-1600", Rating: 1600},
}

func SeedBots(ctx context.Context, pool *pgxpool.Pool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte("bot-internal-password-not-for-login"), bcrypt.MinCost)
	if err != nil {
		return fmt.Errorf("bcrypt for bots: %w", err)
	}

	for _, b := range DefaultBots {
		var existing string
		err := pool.QueryRow(ctx,
			`SELECT id FROM users WHERE username = $1`, b.Username,
		).Scan(&existing)

		if err == nil {
			_, err = pool.Exec(ctx,
				`UPDATE users SET is_bot = TRUE, rating = $1 WHERE username = $2`,
				b.Rating, b.Username,
			)
			if err != nil {
				log.Printf("warn: could not update bot %s: %v", b.Username, err)
			}
			continue
		}

		_, err = pool.Exec(ctx,
			`INSERT INTO users (username, password_hash, is_bot, rating)
			 VALUES ($1, $2, TRUE, $3)
			 ON CONFLICT (username) DO UPDATE SET is_bot = TRUE, rating = $3`,
			b.Username, string(hash), b.Rating,
		)
		if err != nil {
			return fmt.Errorf("seed bot %s: %w", b.Username, err)
		}
		log.Printf("seeded bot: %s (rating %d)", b.Username, b.Rating)
	}
	return nil
}
