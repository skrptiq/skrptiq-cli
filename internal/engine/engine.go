// Package engine provides the CLI's interface to the shared skrptiq engine.
// It opens the shared database at ~/.skrptiq/skrptiq.db and exposes
// convenience methods for querying nodes, profiles, connections, and runs.
package engine

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/skrptiq/engine/storage"
)

// DefaultDBPath returns the default database path: ~/.skrptiq/skrptiq.db
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".skrptiq", "skrptiq.db")
}

// App is the CLI's handle to the engine.
type App struct {
	DB *storage.DB
}

// Open opens the engine database at the given path.
// If path is empty, uses DefaultDBPath().
func Open(path string) (*App, error) {
	if path == "" {
		path = DefaultDBPath()
	}
	db, err := storage.Open(path)
	if err != nil {
		return nil, err
	}
	return &App{DB: db}, nil
}

// Close closes the database connection.
func (a *App) Close() error {
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}

// Workflows returns all nodes of type "workflow".
func (a *App) Workflows() ([]storage.Node, error) {
	nodes, err := a.DB.GetAllNodes()
	if err != nil {
		return nil, err
	}
	var workflows []storage.Node
	for _, n := range nodes {
		if n.Type == "workflow" {
			workflows = append(workflows, n)
		}
	}
	return workflows, nil
}

// Skills returns all nodes of type "skill".
func (a *App) Skills() ([]storage.Node, error) {
	nodes, err := a.DB.GetAllNodes()
	if err != nil {
		return nil, err
	}
	var skills []storage.Node
	for _, n := range nodes {
		if n.Type == "skill" {
			skills = append(skills, n)
		}
	}
	return skills, nil
}

// NodesByType returns all nodes matching the given type.
func (a *App) NodesByType(nodeType string) ([]storage.Node, error) {
	nodes, err := a.DB.GetAllNodes()
	if err != nil {
		return nil, err
	}
	var filtered []storage.Node
	for _, n := range nodes {
		if n.Type == nodeType {
			filtered = append(filtered, n)
		}
	}
	return filtered, nil
}

// Profiles returns all profiles.
func (a *App) Profiles() ([]storage.Profile, error) {
	return a.DB.GetAllProfiles()
}

// ActiveProfile returns the active profile of a given type (e.g. "voice", "audience").
func (a *App) ActiveProfile(profileType string) (*storage.Profile, error) {
	profiles, err := a.DB.GetAllProfiles()
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		if p.Type == profileType && p.IsActive == 1 {
			return &p, nil
		}
	}
	return nil, nil
}

// Connections returns all connections.
func (a *App) Connections() ([]storage.Connection, error) {
	return a.DB.GetAllConnections()
}

// MCPServers returns all connections of type "mcp-server".
func (a *App) MCPServers() ([]storage.Connection, error) {
	return a.DB.GetConnectionsByType("mcp-server")
}

// Providers returns all connections of type "llm-provider".
func (a *App) Providers() ([]storage.Connection, error) {
	return a.DB.GetConnectionsByType("llm-provider")
}

// Tags returns all tags.
func (a *App) Tags() ([]storage.Tag, error) {
	return a.DB.GetAllTags()
}

// FindNodeByTitle finds a node by title (case-insensitive).
func (a *App) FindNodeByTitle(title string) (*storage.Node, error) {
	nodes, err := a.DB.GetAllNodes()
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(title)
	for _, n := range nodes {
		if strings.ToLower(n.Title) == lower {
			return &n, nil
		}
	}
	return nil, nil
}

// SearchNodes finds nodes whose title contains the query.
func (a *App) SearchNodes(query string) ([]storage.Node, error) {
	nodes, err := a.DB.GetAllNodes()
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(query)
	var results []storage.Node
	for _, n := range nodes {
		if strings.Contains(strings.ToLower(n.Title), lower) {
			results = append(results, n)
		}
	}
	return results, nil
}

// HubImports returns all hub imports.
func (a *App) HubImports() ([]storage.HubImport, error) {
	return a.DB.GetAllHubImports()
}

// Setting returns a setting value by key.
func (a *App) Setting(key string) string {
	return a.DB.GetSetting(key)
}
