package bridge

import (
	"encoding/json"
	"os"

	enginemcp "github.com/skrptiq/engine/mcp"
)

// Lifecycle for the browser bridge in the interactive session: the enable flag
// (default-OFF), installing the native-messaging host so Chrome can spawn the
// host mode on demand, and registering the MCP-mode bridge with the engine as a
// stdio MCP child pointed at THIS binary. Mirrors the app's connectBrowserBridge,
// Enhancement-class: failures are surfaced, never fatal to the REPL.

const (
	// bridgeConfigKey stores the non-secret enable flag (settings K-V).
	bridgeConfigKey = "BROWSER_BRIDGE"
	// serverID matches the app's BROWSER_BRIDGE_SERVER_ID so browser tools land
	// under a consistent MCP server id within the engine.
	serverID   = "skrptiq-browser"
	serverName = "Browser"
)

// SettingsStore is the minimal persistence the bridge needs (satisfied by the
// engine storage.DB, exposed via the CLI's engine wrapper).
type SettingsStore interface {
	GetSetting(key string) string
	SetSetting(key, value string) error
}

// Config is the non-secret bridge config.
type Config struct {
	Enabled bool `json:"enabled"`
}

// Status is the bridge state for a Settings/Connections view.
type Status struct {
	Enabled       bool   `json:"enabled"`
	HostInstalled bool   `json:"hostInstalled"`
	Running       bool   `json:"running"`
	SocketPath    string `json:"socketPath"`
	Available     bool   `json:"available"`
}

// Manager owns the bridge lifecycle against a settings store.
type Manager struct {
	settings SettingsStore
}

// NewManager builds a lifecycle manager over the given settings store.
func NewManager(settings SettingsStore) *Manager {
	return &Manager{settings: settings}
}

// GetConfig reads the enable flag (default-OFF on any read/parse failure).
func (m *Manager) GetConfig() Config {
	raw := m.settings.GetSetting(bridgeConfigKey)
	if raw == "" {
		return Config{Enabled: false}
	}
	var c Config
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return Config{Enabled: false}
	}
	return c
}

func (m *Manager) setConfig(c Config) error {
	body, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return m.settings.SetSetting(bridgeConfigKey, string(body))
}

// Enable turns the bridge on: persist the flag, install the native-messaging host
// (idempotent; non-fatal on failure — retryable), and register the MCP-mode
// bridge with the engine. Returns the resulting status.
func (m *Manager) Enable() (Status, error) {
	if err := m.setConfig(Config{Enabled: true}); err != nil {
		return m.Status(), err
	}
	// Host install is non-fatal: the MCP-mode bridge can still own the socket; a
	// missing host just means Chrome can't reach the bridge yet (retry from status).
	_, _ = installNativeHost(nativeHostOpts{})

	self, err := os.Executable()
	if err != nil {
		return m.Status(), err
	}
	enginemcp.Disconnect(serverID) // clear any stale registration
	_, err = enginemcp.Connect(enginemcp.ServerConfig{
		ID:          serverID,
		Name:        serverName,
		Transport:   "stdio",
		Command:     self,
		Args:        []string{"__bridge", "--mode", "mcp"},
		AutoConnect: true,
	})
	return m.Status(), err
}

// Disable turns the bridge off: disconnect the MCP-mode child, remove the
// native-messaging host, and clear the flag.
func (m *Manager) Disable() error {
	enginemcp.Disconnect(serverID)
	uninstallNativeHost(nativeHostOpts{})
	return m.setConfig(Config{Enabled: false})
}

// Status reports the current bridge state.
func (m *Manager) Status() Status {
	_, catErr := loadCatalog()
	running := false
	for _, c := range enginemcp.GetAllConnections() {
		if c.ID == serverID && c.Status == "connected" {
			running = true
			break
		}
	}
	return Status{
		Enabled:       m.GetConfig().Enabled,
		HostInstalled: isNativeHostInstalled(nativeHostOpts{}),
		Running:       running,
		SocketPath:    socketPathFor(RoleCLI),
		Available:     catErr == nil,
	}
}

// ConnectIfEnabled registers the bridge at session start when the user has it
// enabled (default-OFF ⇒ no-op). Enhancement-class: returns false on any failure
// without disturbing startup.
func (m *Manager) ConnectIfEnabled() bool {
	if !m.GetConfig().Enabled {
		return false
	}
	if _, err := m.Enable(); err != nil {
		return false
	}
	return true
}
