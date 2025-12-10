package database

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(storePath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", storePath)
	if err != nil {
		return nil, err
	}
	// Ping, make sure it is alive

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Enable Foreign Keys for this connection
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return nil, err
	}

	store := &SQLiteStore{db: db}
	// Migrate
	if err := store.Migrate(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Migrate() error {
	SQL := `CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			password TEXT NOT NULL,
			role TEXT NOT NULL
		)`

	_, err := s.db.Exec(SQL)
	return err
}

func (s *SQLiteStore) GetUser(username string) (*User, error) {
	SQL := `SELECT id, username, password, role FROM users WHERE username = ?`
	row := s.db.QueryRow(SQL, username)

	var user User
	err := row.Scan(&user.ID, &user.Username, &user.Password, &user.Role)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *SQLiteStore) CreateUser(user *User) error {
	SQL := `INSERT INTO users (username, password, role) VALUES (?, ?, ?)`
	_, err := s.db.Exec(SQL, user.Username, user.Password, user.Role)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
