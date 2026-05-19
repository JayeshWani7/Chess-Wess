package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	IsBot        bool      `json:"is_bot"`
	Rating       int       `json:"rating"`
	CreatedAt    time.Time `json:"created_at"`
}
