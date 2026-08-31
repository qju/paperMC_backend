package database

import (
	"time"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"` // hash, not serialized in JSON
	Role     string `json:"role"`
}

type RejectedPlayer struct {
	Username string    `json:"username"`
	Count    int       `json:"count"`
	LastSeen time.Time `json:"last_seen"`
}

type Store interface {
	Migrate() error
	Close() error

	// User Auth
	GetUser(username string) (*User, error)
	ListUsers() ([]User, error)
	CreateUser(user *User) error
	UpdateUserPassword(username, passwordHash string) error
	DeleteUser(username string) error

	// Player Intelligence
	UpsertRejectedPlayer(username string) error
	GetRejectedPlayers() ([]RejectedPlayer, error)
	DeleteRejectedPlayer(username string) error
}
