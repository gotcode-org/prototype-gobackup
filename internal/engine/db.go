package engine

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	Username string
	Role     string
}

type DB struct {
	conn *sql.DB
}

// InitDB connects to the SQLite database and runs migrations
func InitDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Create users table if it doesn't exist
	query := `
	CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		token_hash TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	
	if _, err := conn.Exec(query); err != nil {
		return nil, fmt.Errorf("failed to create users table: %w", err)
	}

	return &DB{conn: conn}, nil
}

// CreateUser generates a token, hashes it, stores it, and returns the raw token to the user
func (db *DB) CreateUser(username, role string) (string, error) {
	rawToken, err := GenerateToken()
	if err != nil {
		return "", err
	}

	hash := HashToken(rawToken)

	query := `INSERT INTO users (username, token_hash, role) VALUES (?, ?, ?)
			  ON CONFLICT(username) DO UPDATE SET token_hash=excluded.token_hash, role=excluded.role`
	
	if _, err := db.conn.Exec(query, username, hash, role); err != nil {
		return "", err
	}

	log.Printf("🔐 Generated new token for user '%s' (Role: %s)", username, role)
	return rawToken, nil
}

// ValidateToken checks if a raw token exists and is valid, returning the associated User
func (db *DB) ValidateToken(rawToken string) (*User, error) {
	hash := HashToken(rawToken)
	
	var user User
	query := `SELECT username, role FROM users WHERE token_hash = ?`
	
	err := db.conn.QueryRow(query, hash).Scan(&user.Username, &user.Role)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid token")
		}
		return nil, err
	}
	
	return &user, nil
}
