package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the configuration structure
type Config struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Action        string `json:"action"`
	CheckInterval int    `json:"check_interval"` // seconds, systemd timer interval
	RetryInterval int    `json:"retry_interval"` // seconds between attempts of one run
	MaxRetries    int    `json:"max_retries"`    // max attempts per run, including the first one
	CheckAccount  bool   `json:"check_account"`  // logout and re-login when the online account differs

	// Probe pool: instead of asking the portal on every run, a run first checks
	// whether the public internet answers, and only turns to the portal when it
	// does not. See probe.go for how the pool is walked.
	ProbeEnabled       bool     `json:"probe_enabled"`
	ProbeURLs          []string `json:"probe_urls"`
	ProbeTimeout       int      `json:"probe_timeout"`        // seconds per probe
	ProbeFailThreshold int      `json:"probe_fail_threshold"` // silent targets in a row before the portal is asked

	// AccountCheckInterval is the slow lane that catches a session held by
	// another account: the probe cannot see which account is online, so the
	// portal is asked about it at most this often. 0 disables it.
	AccountCheckInterval int `json:"account_check_interval"` // seconds

	StateFile string `json:"state_file"`
	LogFile   string `json:"log_file"`
	Quiet     bool   `json:"quiet"`
	NoLog     bool   `json:"no_log"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Action:        "",
		CheckInterval: 60, // 1 minute
		RetryInterval: 5,  // 5 seconds
		MaxRetries:    3,
		CheckAccount:  true,

		ProbeEnabled:       true,
		ProbeURLs:          append([]string(nil), defaultProbeURLs...),
		ProbeTimeout:       5,
		ProbeFailThreshold: 3,

		AccountCheckInterval: 1800, // 30 minutes

		LogFile: "buct-login.log",
		Quiet:   false,
		NoLog:   false,
	}
}

// LoadConfig loads configuration from file
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	if configPath == "" {
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return config, nil
}

// SaveConfig saves configuration to file, the file holds a password so it is
// only readable by its owner
func SaveConfig(config *Config, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	err = os.WriteFile(configPath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// configSearchPaths returns the candidate locations probed when no config path
// is given on the command line
func configSearchPaths() []string {
	paths := []string{"config.json"}

	if dir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(dir, "buct-login", "config.json"))
	}

	return append(paths, "/etc/buct-login/config.json")
}

// FindConfig returns the first existing config file among the search paths,
// or an empty string when none exists
func FindConfig() string {
	for _, path := range configSearchPaths() {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// generateConfigTemplate generates a template configuration file
func generateConfigTemplate(configPath string) error {
	config := DefaultConfig()
	config.Action = "login"
	config.Username = "your_student_id"
	config.Password = "your_password"

	return SaveConfig(config, configPath)
}
