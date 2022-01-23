package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// TextsDB ...
type TextsDB struct {
	db *sql.DB
}

// NewTextsDB ...
func NewTextsDB(dbPath string) (*TextsDB, error) {
	if _, err := os.Stat(dbPath); err == nil {
		return nil, fmt.Errorf("file exists at %s", dbPath)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	return &TextsDB{db}, nil
}

// CreateTables ...
func (texts *TextsDB) CreateTables() error {
	_, err := texts.db.Exec(
		`
CREATE TABLE IF NOT EXISTS texts (
	id INT PRIMARY KEY,
	text TEXT,
	type TEXT,
	author TEXT,
	source TEXT
	)
`)

	if err != nil {
		return fmt.Errorf("executing sql statement failed: %w", err)
	}

	return nil
}

// Insert ...
func (texts *TextsDB) Insert(id int64, text, typ, author, source string) error {
	insert, err := texts.db.Prepare("INSERT INTO texts (id, text, type, author, source) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to created prepared insert statement: %w", err)
	}

	_, err = insert.Exec(text, typ, author, source, id)
	return err
}
