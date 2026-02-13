// Package config loads database connection configuration from environment
// variables and an optional config file. Connection URIs are never logged
// or exposed to tool responses.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Env var names for connection strings. If set, they define connections
// with fixed IDs "postgres", "sqlserver", and "sqlite".
const (
	EnvPostgresURI  = "MCP_DB_POSTGRES_URI"
	EnvSQLServerURI = "MCP_DB_SQLSERVER_URI"
	EnvSQLiteURI    = "MCP_DB_SQLITE_URI"
)

// DefaultConfigDir is the directory for the optional config file.
// Config file path: ~/.localdb-mcp/config.yaml
const DefaultConfigDir = ".localdb-mcp"
const ConfigFileName = "config.yaml"

// Config holds loaded connection configuration. URIs are stored but never
// included in logs or tool output.
type Config struct {
	connections map[string]connectionEntry
}

type connectionEntry struct {
	Type string // "postgres" or "sqlserver"
	uri  string
}

// ConnectionInfo is safe to log or return to tools: no credentials.
type ConnectionInfo struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Load reads configuration from the environment and, if present,
// a .env file in the current directory and ~/.localdb-mcp/config.yaml.
// Env vars override .env and file values for the same connection ID.
func Load() (*Config, error) {
	// 0) Optional .env in cwd (so server sees MCP_DB_* when run via mcpclient or from project root)
	loadEnvFile(".")

	c := &Config{connections: make(map[string]connectionEntry)}

	// 1) Optional config file (base)
	configPath, err := configFilePath()
	if err != nil {
		return nil, fmt.Errorf("config path: %w", err)
	}
	if configPath != "" {
		if err := c.loadFile(configPath); err != nil {
			return nil, fmt.Errorf("config file %s: %w", configPath, err)
		}
	}

	// 2) Env overrides
	if v := os.Getenv(EnvPostgresURI); v != "" {
		c.connections["postgres"] = connectionEntry{Type: "postgres", uri: v}
	}
	if v := os.Getenv(EnvSQLServerURI); v != "" {
		c.connections["sqlserver"] = connectionEntry{Type: "sqlserver", uri: v}
	}
	if v := os.Getenv(EnvSQLiteURI); v != "" {
		c.connections["sqlite"] = connectionEntry{Type: "sqlite", uri: v}
	}

	if len(c.connections) == 0 {
		return c, nil
	}
	return c, nil
}

// loadEnvFile reads .env from dir and sets env vars for any key not already set.
func loadEnvFile(dir string) {
	path := filepath.Join(dir, ".env")
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}
		if strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`) {
			val = strings.Trim(val, `"`)
		} else if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") {
			val = strings.Trim(val, "'")
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(home, DefaultConfigDir, ConfigFileName)
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return p, nil
}

type fileFormat struct {
	Connections map[string]string `yaml:"connections"`
}

func (c *Config) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var f fileFormat
	if err := yaml.Unmarshal(data, &f); err != nil {
		return err
	}
	for id, uri := range f.Connections {
		if uri == "" {
			continue
		}
		typ := idToType(id)
		c.connections[id] = connectionEntry{Type: typ, uri: uri}
	}
	return nil
}

func idToType(id string) string {
	switch id {
	case "postgres", "sqlserver", "sqlite":
		return id
	default:
		return "postgres"
	}
}

// ConnectionIDs returns all configured connection IDs. Safe to log.
func (c *Config) ConnectionIDs() []string {
	ids := make([]string, 0, len(c.connections))
	for id := range c.connections {
		ids = append(ids, id)
	}
	return ids
}

// ConnectionInfos returns connection id and type for each connection. Safe to return from tools.
func (c *Config) ConnectionInfos() []ConnectionInfo {
	infos := make([]ConnectionInfo, 0, len(c.connections))
	for id, e := range c.connections {
		infos = append(infos, ConnectionInfo{ID: id, Type: e.Type})
	}
	return infos
}

// URI returns the connection URI for the given ID. For use only by the db layer; never log the result.
func (c *Config) URI(id string) (uri string, ok bool) {
	e, ok := c.connections[id]
	if !ok {
		return "", false
	}
	return e.uri, true
}

// HasConnection returns whether the given connection ID is configured.
func (c *Config) HasConnection(id string) bool {
	_, ok := c.connections[id]
	return ok
}

// Type returns the database type for the connection ID ("postgres" or "sqlserver"). ok is false if ID is not configured.
func (c *Config) Type(id string) (typ string, ok bool) {
	e, ok := c.connections[id]
	if !ok {
		return "", false
	}
	return e.Type, true
}
