package db

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/SedlarDavid/localdb-mcp/internal/config"
)

// Manager holds configuration and caches drivers by connection ID.
type Manager struct {
	cfg    *config.Config
	mu     sync.Mutex
	drivers map[string]Driver
}

// NewManager returns a manager that will create drivers from cfg.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:    cfg,
		drivers: make(map[string]Driver),
	}
}

// Driver returns a Driver for the given connection ID, creating and caching it if needed.
func (m *Manager) Driver(ctx context.Context, connectionID string) (Driver, error) {
	uri, ok := m.cfg.URI(connectionID)
	if !ok {
		return nil, fmt.Errorf("unknown connection: %q", connectionID)
	}
	typ, _ := m.cfg.Type(connectionID)

	m.mu.Lock()
	d, cached := m.drivers[connectionID]
	m.mu.Unlock()

	if cached {
		return d, nil
	}

	var newDriver Driver
	var err error
	switch typ {
	case "postgres":
		newDriver, err = NewPostgresDriver(ctx, uri)
	case "sqlserver":
		newDriver, err = NewSQLServerDriver(ctx, uri)
	case "sqlite":
		newDriver, err = NewSQLiteDriver(ctx, uri)
	case "mysql":
		newDriver, err = NewMySQLDriver(ctx, uri)
	default:
		return nil, fmt.Errorf("unsupported connection type %q for %q", typ, connectionID)
	}
	if err != nil {
		// Log the full error (may contain the URI) for debugging, but
		// return only a safe message to the caller — tool responses must
		// never expose connection strings or credentials.
		log.Printf("driver %q (%s): %v", connectionID, typ, err)
		return nil, fmt.Errorf("failed to connect to %q (%s); check server logs for details", connectionID, typ)
	}

	m.mu.Lock()
	if existing, ok := m.drivers[connectionID]; ok {
		m.mu.Unlock()
		newDriver.Close()
		return existing, nil
	}
	m.drivers[connectionID] = newDriver
	m.mu.Unlock()

	return newDriver, nil
}

// Close closes all cached drivers. Call when shutting down.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, d := range m.drivers {
		_ = d.Close()
		delete(m.drivers, id)
	}
	return nil
}
