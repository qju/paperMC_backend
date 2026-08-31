package database

import (
	"database/sql"
	"time"

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
	return RunMigrations(s.db)
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

func (s *SQLiteStore) ListUsers() ([]User, error) {
	SQL := `SELECT id, username, role FROM users ORDER BY id ASC`
	rows, err := s.db.Query(SQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *SQLiteStore) CreateUser(user *User) error {
	SQL := `INSERT INTO users (username, password, role) VALUES (?, ?, ?)`
	_, err := s.db.Exec(SQL, user.Username, user.Password, user.Role)
	return err
}

func (s *SQLiteStore) UpdateUserPassword(username, passwordHash string) error {
	SQL := `UPDATE users SET password = ? WHERE username = ?`
	_, err := s.db.Exec(SQL, passwordHash, username)
	return err
}

func (s *SQLiteStore) DeleteUser(username string) error {
	SQL := `DELETE FROM users WHERE username = ?`
	_, err := s.db.Exec(SQL, username)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- Players Inteligence ---

func (s *SQLiteStore) UpsertRejectedPlayer(username string) error {
	// If exist, update count and time. If not INSERT
	SQL := `INSERT INTO rejected_players (username, count, last_seen)
			VALUES (?, 1, CURRENT_TIMESTAMP)
			ON CONFLICT(username) DO UPDATE SET
				count = count + 1,
				last_seen = CURRENT_TIMESTAMP;`
	_, err := s.db.Exec(SQL, username)
	return err
}

func parseSQLiteTime(t string) time.Time {
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, t); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func (s *SQLiteStore) GetRejectedPlayers() ([]RejectedPlayer, error) {
	SQL := `SELECT username, count, last_seen FROM rejected_players ORDER BY last_seen DESC LIMIT 50`
	rows, err := s.db.Query(SQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []RejectedPlayer{}
	for rows.Next() {
		var p RejectedPlayer
		var t string
		if err := rows.Scan(&p.Username, &p.Count, &t); err != nil {
			continue
		}
		p.LastSeen = parseSQLiteTime(t)
		list = append(list, p)
	}
	return list, nil
}

func (s *SQLiteStore) DeleteRejectedPlayer(username string) error {
	SQL := `DELETE FROM rejected_players WHERE username = ?`
	_, err := s.db.Exec(SQL, username)
	return err
}
