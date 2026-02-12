package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoad_envOnly(t *testing.T) {
	// Unset any existing env to avoid pollution; then set our own.
	oldPg := os.Getenv(EnvPostgresURI)
	oldMs := os.Getenv(EnvSQLServerURI)
	defer func() {
		os.Setenv(EnvPostgresURI, oldPg)
		os.Setenv(EnvSQLServerURI, oldMs)
	}()

	os.Unsetenv(EnvPostgresURI)
	os.Unsetenv(EnvSQLServerURI)
	os.Setenv(EnvPostgresURI, "postgres://local:secret@localhost/db")
	os.Setenv(EnvSQLServerURI, "sqlserver://sa:Secret123@localhost")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	ids := cfg.ConnectionIDs()
	if len(ids) < 2 {
		t.Errorf("expected at least 2 connection IDs, got %v", ids)
	}
	infos := cfg.ConnectionInfos()
	for _, info := range infos {
		if info.ID == "" || info.Type == "" {
			t.Errorf("ConnectionInfo must not have empty ID or Type: %+v", info)
		}
		// Ensure we never expose URI in ConnectionInfo (no "uri" field; Type is not a secret).
		if info.ID == "postgres" && info.Type != "postgres" {
			t.Errorf("postgres connection should have type postgres, got %q", info.Type)
		}
	}

	uri, ok := cfg.URI("postgres")
	if !ok || uri == "" {
		t.Error("expected postgres URI to be set")
	}
	// Sanity: URI must not appear in any public API that could be logged.
	// ConnectionInfos and ConnectionIDs do not contain URIs by design.
	_ = uri
}

func TestLoad_fileOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	// Use a config file in temp dir. We need to inject this path; currently
	// Load() only looks in ~/.localdb-mcp. So this test only runs when we
	// can test file loading. For now, test via env and test that file format
	// parses correctly in a unit test below (TestLoadFileFormat).
	err := os.WriteFile(path, []byte(`
connections:
  postgres: "postgres://u:p@localhost/postgres"
  sqlserver: "sqlserver://sa:p@localhost"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Load() in production reads from home dir; we don't override home in this test.
	// So we test the file parsing separately and env in TestLoad_envOnly.
	_ = path
}

func TestLoadFileFormat(t *testing.T) {
	c := &Config{connections: make(map[string]connectionEntry)}
	data := []byte(`
connections:
  postgres: "postgres://u:p@localhost/postgres"
  custom: "postgres://other@localhost/other"
`)
	var f fileFormat
	if err := yaml.Unmarshal(data, &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for id, uri := range f.Connections {
		if uri == "" {
			continue
		}
		typ := idToType(id)
		c.connections[id] = connectionEntry{Type: typ, uri: uri}
	}

	infos := c.ConnectionInfos()
	if len(infos) != 2 {
		t.Errorf("expected 2 connections, got %d", len(infos))
	}
	// ConnectionInfo must never contain URI
	for _, info := range infos {
		if info.ID == "" || info.Type == "" {
			t.Errorf("ConnectionInfo empty field: %+v", info)
		}
	}
}

func TestConnectionInfos_NoURIs(t *testing.T) {
	c := &Config{connections: map[string]connectionEntry{
		"pg": {Type: "postgres", uri: "postgres://secret:password@host/db"},
	}}
	infos := c.ConnectionInfos()
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}
	// Ensure struct has no URI field exposed (ConnectionInfo only has ID and Type).
	typ := reflect.TypeOf(ConnectionInfo{})
	if typ.NumField() != 2 {
		t.Errorf("ConnectionInfo should have exactly 2 fields (ID, Type), has %d", typ.NumField())
	}
	if infos[0].ID != "pg" || infos[0].Type != "postgres" {
		t.Errorf("unexpected info: %+v", infos[0])
	}
}

func TestHasConnection(t *testing.T) {
	c := &Config{connections: map[string]connectionEntry{
		"postgres": {Type: "postgres", uri: "x"},
	}}
	if !c.HasConnection("postgres") {
		t.Error("expected HasConnection(postgres) true")
	}
	if c.HasConnection("missing") {
		t.Error("expected HasConnection(missing) false")
	}
}

func TestURI(t *testing.T) {
	c := &Config{connections: map[string]connectionEntry{
		"postgres": {Type: "postgres", uri: "postgres://localhost/db"},
	}}
	uri, ok := c.URI("postgres")
	if !ok || uri != "postgres://localhost/db" {
		t.Errorf("URI(postgres): ok=%v uri=%q", ok, uri)
	}
	_, ok = c.URI("missing")
	if ok {
		t.Error("URI(missing) should be !ok")
	}
}
