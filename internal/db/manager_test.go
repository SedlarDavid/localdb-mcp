package db

import (
	"context"
	"testing"

	"github.com/SedlarDavid/localdb-mcp/internal/config"
)

func TestManager_Driver_unknownConnection(t *testing.T) {
	cfg, _ := config.Load() // may have no connections
	m := NewManager(cfg)
	ctx := context.Background()

	_, err := m.Driver(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for unknown connection")
	}
}

func TestManager_Close_idempotent(t *testing.T) {
	m := NewManager(nil)
	if err := m.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Errorf("Close again: %v", err)
	}
}
