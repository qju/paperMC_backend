package database

type User struct {
	ID       int
	Username string
	Password string // hash
	Role     string
}

type Store interface {
	Migrate() error
	GetUser(username string) (*User, error)
	CreateUser(user *User) error
	Close() error
}
