// Package engine provides the CLI's interface to the shared skrptiq engine.
// It opens the shared database at ~/.skrptiq/skrptiq.db and exposes
// convenience methods for querying nodes, profiles, connections, and runs.
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skrptiq/engine/hubapi"
	"github.com/skrptiq/engine/storage"
)

// DefaultDBPath returns the database path, checking locations in order:
//  1. ~/Library/Application Support/skrptiq/data/skrptiq.db (macOS app location)
//  2. ~/.skrptiq/skrptiq.db (CLI default)
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check the desktop app's location first (shared DB).
	appSupport := filepath.Join(home, "Library", "Application Support", "skrptiq", "data", "skrptiq.db")
	if _, err := os.Stat(appSupport); err == nil {
		return appSupport
	}

	return filepath.Join(home, ".skrptiq", "skrptiq.db")
}

// App is the CLI's handle to the engine.
type App struct {
	DB  *storage.DB
	Hub *hubapi.Client
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
	return &App{
		DB:  db,
		Hub: hubapi.NewClient(db),
	}, nil
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

// ExecutionSummary is a lightweight execution record for listing.
type ExecutionSummary struct {
	ID            string
	WorkflowTitle string
	Status        string
	TotalTokens   int
	StartedAt     string
	CompletedAt   *string
	Error         *string
}

// ListExecutions returns recent executions with workflow titles.
func (a *App) ListExecutions(limit int) ([]ExecutionSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := a.DB.Query(
		`SELECT e.id, COALESCE(n.title, e.workflow_node_id), e.status, e.total_tokens, e.started_at, e.completed_at, e.error
		 FROM executions e
		 LEFT JOIN nodes n ON n.id = e.workflow_node_id
		 ORDER BY e.started_at DESC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ExecutionSummary
	for rows.Next() {
		var s ExecutionSummary
		if err := rows.Scan(&s.ID, &s.WorkflowTitle, &s.Status, &s.TotalTokens, &s.StartedAt, &s.CompletedAt, &s.Error); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, nil
}

// RunDetail is a full execution record with resolved titles.
type RunDetail struct {
	ID            string
	WorkflowTitle string
	Status        string
	TotalTokens   int
	StartedAt     string
	CompletedAt   *string
	Error         *string
	Steps         []StepDetail
}

// StepDetail is a step with its node title resolved.
type StepDetail struct {
	Position  int
	NodeTitle string
	Status    string
	Provider  string
	Model     string
	Output    string
	Error     string
	StartedAt string
	Duration  string
}

// GetRunDetail returns a full execution with resolved node titles and steps.
func (a *App) GetRunDetail(id string) (*RunDetail, error) {
	exec, err := a.DB.GetExecution(id)
	if err != nil {
		return nil, err
	}

	// Resolve workflow title.
	wfTitle := exec.WorkflowNodeID
	if node, err := a.DB.GetNode(exec.WorkflowNodeID); err == nil && node != nil {
		wfTitle = node.Title
	}

	steps, err := a.DB.GetStepsByExecution(id)
	if err != nil {
		return nil, err
	}

	// Build node title cache.
	nodeCache := make(map[string]string)
	for _, s := range steps {
		if _, ok := nodeCache[s.NodeID]; !ok {
			if node, err := a.DB.GetNode(s.NodeID); err == nil && node != nil {
				nodeCache[s.NodeID] = node.Title
			} else {
				nodeCache[s.NodeID] = s.NodeID
			}
		}
	}

	var details []StepDetail
	for _, s := range steps {
		d := StepDetail{
			Position:  s.Position,
			NodeTitle: nodeCache[s.NodeID],
			Status:    s.Status,
		}
		if s.Provider != nil {
			d.Provider = *s.Provider
		}
		if s.Model != nil {
			d.Model = *s.Model
		}
		if s.Output != nil {
			d.Output = *s.Output
		}
		if s.Error != nil {
			d.Error = *s.Error
		}
		if s.StartedAt != nil {
			d.StartedAt = *s.StartedAt
		}
		if s.StartedAt != nil && s.CompletedAt != nil {
			d.Duration = formatDuration(*s.StartedAt, *s.CompletedAt)
		}
		details = append(details, d)
	}

	return &RunDetail{
		ID:            exec.ID,
		WorkflowTitle: wfTitle,
		Status:        exec.Status,
		TotalTokens:   exec.TotalTokens,
		StartedAt:     exec.StartedAt,
		CompletedAt:   exec.CompletedAt,
		Error:         exec.Error,
		Steps:         details,
	}, nil
}

// FindRunByPrefix finds a run whose ID starts with the given prefix.
func (a *App) FindRunByPrefix(prefix string) (*string, error) {
	rows, err := a.DB.Query(
		`SELECT id FROM executions WHERE id LIKE ? ORDER BY started_at DESC LIMIT 1`,
		prefix+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var id string
		rows.Scan(&id)
		return &id, nil
	}
	return nil, nil
}

func formatDuration(start, end string) string {
	t1, err1 := time.Parse(time.RFC3339, start)
	t2, err2 := time.Parse(time.RFC3339, end)
	if err1 != nil || err2 != nil {
		return ""
	}
	d := t2.Sub(t1)
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// GetSteps returns all steps for an execution.
func (a *App) GetSteps(executionID string) ([]storage.ExecutionStep, error) {
	return a.DB.GetStepsByExecution(executionID)
}

// SetActiveProfile sets a profile as active for its type.
func (a *App) SetActiveProfile(id, profileType string) error {
	return a.DB.SetActiveProfile(id, profileType)
}

// FindProfileByName finds a profile by name (case-insensitive).
func (a *App) FindProfileByName(name string) (*storage.Profile, error) {
	profiles, err := a.DB.GetAllProfiles()
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(name)
	for _, p := range profiles {
		if strings.ToLower(p.Name) == lower {
			return &p, nil
		}
	}
	return nil, nil
}
