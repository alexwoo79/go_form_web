// +build ignore

package models

import (
	"testing"
)

func TestNewDatabase(t *testing.T) {
	// Test database creation
	db, err := NewDatabase(&config.DatabaseConfig{Path: ":memory:", Type: "sqlite"})
	if err != nil {
		t.Errorf("NewDatabase() error = %v", err)
	}
	if db == nil {
		t.Error("NewDatabase() returned nil")
	}
	db.Close()
}

func TestTableExists(t *testing.T) {
	db, _ := NewDatabase(&config.DatabaseConfig{Path: ":memory:", Type: "sqlite"})
	defer db.Close()

	result := db.TableExists("nonexistent_table")
	if result {
		t.Error("TableExists() returned true for non-existent table")
	}
}
