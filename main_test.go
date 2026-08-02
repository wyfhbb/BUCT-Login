package main

import (
	"strings"
	"testing"
)

func TestAccountMatches(t *testing.T) {
	cases := []struct {
		expected string
		online   string
		want     bool
	}{
		{"2021001", "2021001", true},
		{"2021001", " 2021001", true},
		{"2021001", "2021001@cmcc", true},  // portal appends the ISP suffix
		{"2021001@cmcc", "2021001", false}, // an explicit suffix must match
		{"2021001@cmcc", "2021001@cmcc", true},
		{"2021001@cmcc", "2021001@unicom", false},
		{"2021001", "2021002", false},
		{"2021001", "2021002@cmcc", false},
		{"2021001", "", false},
	}

	for _, c := range cases {
		if got := accountMatches(c.expected, c.online); got != c.want {
			t.Errorf("accountMatches(%q, %q) = %v, want %v", c.expected, c.online, got, c.want)
		}
	}
}

func TestAccountName(t *testing.T) {
	if got := accountName(map[string]interface{}{"user_name": " 2021001 "}); got != "2021001" {
		t.Errorf("accountName = %q, want %q", got, "2021001")
	}
	if got := accountName(map[string]interface{}{}); got != "" {
		t.Errorf("accountName without user_name = %q, want empty", got)
	}
}

func TestRenderUnits(t *testing.T) {
	env := &systemdEnv{
		UnitDir:    "/etc/systemd/system",
		ConfigPath: "/etc/buct-login/config.json",
		BinaryPath: "/usr/local/bin/buct-login",
	}
	config := DefaultConfig()
	config.CheckInterval = 120
	config.RetryInterval = 5
	config.MaxRetries = 3

	service := env.renderService(config)
	for _, want := range []string{
		"Type=oneshot",
		"ExecStart=/usr/local/bin/buct-login -action=login -config=/etc/buct-login/config.json",
		"TimeoutStartSec=75",
		"After=network-online.target",
	} {
		if !strings.Contains(service, want) {
			t.Errorf("service unit missing %q:\n%s", want, service)
		}
	}
	if strings.Contains(service, "[Install]") {
		t.Errorf("service unit must not be enableable on its own:\n%s", service)
	}

	timer := env.renderTimer(config)
	for _, want := range []string{
		"OnBootSec=30s",
		"OnUnitActiveSec=120s",
		"Unit=buct-login.service",
		"WantedBy=timers.target",
	} {
		if !strings.Contains(timer, want) {
			t.Errorf("timer unit missing %q:\n%s", want, timer)
		}
	}

	userEnv := &systemdEnv{UserScope: true, BinaryPath: "/home/u/bin/buct login", ConfigPath: "/home/u/c.json"}
	userService := userEnv.renderService(config)
	if !strings.Contains(userService, `ExecStart="/home/u/bin/buct login" -action=login -config=/home/u/c.json`) {
		t.Errorf("paths with spaces must be quoted:\n%s", userService)
	}
	if strings.Contains(userService, "network-online.target") {
		t.Errorf("user units must not depend on system targets:\n%s", userService)
	}
	if !strings.Contains(userEnv.renderTimer(config), "OnStartupSec=30s") {
		t.Errorf("user timer must trigger on manager startup")
	}
}
