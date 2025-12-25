package database

import (
	"time"
)

type User struct {
	ID       int
	Username string
	Password string // hash
	Role     string
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
	CreateUser(user *User) error

	//Player Intelligence
	UpsertRejectedPlayer(username string) error
	GetRejectedPlayers() ([]RejectedPlayer, error)
	DeleteRejectedPlayer(username string) error
}
