package repository

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

func TestNewUserRepository(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer db.Close()

	repo := NewUserRepository(db)
	if repo == nil {
		t.Fatal("repo should not be nil")
	}
}

func TestNewSessionRepository(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer db.Close()

	repo := NewSessionRepository(db)
	if repo == nil {
		t.Fatal("repo should not be nil")
	}
}

func TestNewChatRepository(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer db.Close()

	repo := NewChatRepository(db)
	if repo == nil {
		t.Fatal("repo should not be nil")
	}
}
