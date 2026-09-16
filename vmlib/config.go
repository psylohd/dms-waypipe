package vmlib

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type VMConfig struct {
	Name         string     `json:"name,omitempty"`
	Color        string     `json:"color,omitempty"`
	Color2       string     `json:"color2,omitempty"` // unfocused border color
	Apps         []AppEntry `json:"apps,omitempty"`
	SSHTarget    string     `json:"sshTarget,omitempty"`
	SSHHost      string     `json:"sshHost,omitempty"`
	SSHPort      int        `json:"sshPort,omitempty"`
	SSHUser      string     `json:"sshUser,omitempty"`
	WaypipeFlags string     `json:"waypipeFlags,omitempty"`
}

type AppEntry struct {
	Name     string   // display name
	Exec     string   // command to run inside guest
	Icon     string   // icon path or stock name
	Keywords []string // extra fuzzy-search terms
	Comment  string   // secondary description
	NoDisplay  bool   // whether desktop file has NoDisplay=true
	Type     string   // Desktop Entry type (Application, etc.)
	ForwardMode string // forwarding mode: "waypipe" (default) or "x11" for Java AWT/Swing apps
}

type Config struct {
	VMs map[string]VMConfig // keyed by lowercase VM name
}

func DefaultConfigPath() string {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		home, _ := os.UserHomeDir()
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, "dms-waypipe", "config.json")
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	// Re-serialize through LoadConfig's map approach to preserve
	// lowercase "vms" key and exact field casing.
	m := map[string]interface{}{
		"vms": cfg.VMs,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
