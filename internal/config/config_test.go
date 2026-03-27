package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Test loading config file if it exists
	if _, err := os.Stat("config.example.yaml"); err == nil {
		cfg, err := Load("config.example.yaml")
		if err != nil {
			t.Errorf("Load() error = %v", err)
		}
		if cfg == nil {
			t.Error("Load() returned nil")
		}
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("nonexistent.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}
