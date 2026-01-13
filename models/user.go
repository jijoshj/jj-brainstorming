package models

import "time"

type User struct {
	Email    string    `json:"email"`
	LobbyID  string    `json:"lobby_id"`
	JoinedAt time.Time `json:"joined_at"`
	IsActive bool      `json:"is_active"`
	LastSeen time.Time `json:"last_seen"`
}
