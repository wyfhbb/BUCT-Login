package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestMain keeps the log lines of the code under test out of the test output
func TestMain(m *testing.M) {
	logrus.SetOutput(io.Discard)
	os.Exit(m.Run())
}

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
		// 5s * 3 retries + 3 probes * 5s + 60s of headroom
		"TimeoutStartSec=90",
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

// deadTarget is a port nothing listens on, a probe of it fails right away
const deadTarget = "http://127.0.0.1:1/generate_204"

// probePool starts a server answering the way each kind of path suggests:
// /ok_204 behaves like a real detection endpoint, /portal_204 like the campus
// portal catching the request, /page_204 like anything else replying in place
// of the endpoint
func probePool(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/ok"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/portal"):
			http.Redirect(w, r, "http://202.4.130.95/srun_portal_pc", http.StatusFound)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>login page</html>"))
		}
	}))
	t.Cleanup(server.Close)

	return server
}

func TestProbeTarget(t *testing.T) {
	server := probePool(t)
	client := newProbeClient(2 * time.Second)

	cases := []struct {
		name   string
		target string
		want   probeResult
	}{
		{"empty 204", server.URL + "/ok_204", probeOK},
		{"portal redirect", server.URL + "/portal_204", probeIntercepted},
		{"page in place of the endpoint", server.URL + "/page_204", probeIntercepted},
		{"custom target answering 200", server.URL + "/page", probeOK},
		{"nothing listening", deadTarget, probeUnreachable},
	}

	for _, c := range cases {
		got, err := probeTarget(client, c.target)
		if got != c.want {
			t.Errorf("%s: probeTarget(%s) = %v (err %v), want %v", c.name, c.target, got, err, c.want)
		}
	}
}

func TestRunProbes(t *testing.T) {
	server := probePool(t)
	ok := server.URL + "/ok_204"
	portal := server.URL + "/portal_204"

	cases := []struct {
		name        string
		pool        []string
		threshold   int
		index       int
		wantBlocked bool
		wantIndex   int
	}{
		{
			name:      "the first answer ends the cycle",
			pool:      []string{ok, ok, ok},
			threshold: 3,
			wantIndex: 1, // only one target was touched, the next run starts at the second
		},
		{
			name:      "one silent target is not enough",
			pool:      []string{deadTarget, ok, ok},
			threshold: 3,
			wantIndex: 2,
		},
		{
			name:        "the threshold of silent targets is",
			pool:        []string{deadTarget, deadTarget, deadTarget, ok},
			threshold:   3,
			wantBlocked: true,
			wantIndex:   3,
		},
		{
			name:        "interception ends the cycle right away",
			pool:        []string{portal, ok, ok},
			threshold:   3,
			wantBlocked: true,
			wantIndex:   1,
		},
		{
			name:      "the rotation wraps around",
			pool:      []string{ok, ok, ok},
			threshold: 3,
			index:     2,
			wantIndex: 0,
		},
		{
			name:        "a threshold above the pool size stops at the pool size",
			pool:        []string{deadTarget, deadTarget},
			threshold:   5,
			wantBlocked: true,
			wantIndex:   0, // both targets tried, the rotation is back at the start
		},
		{
			name:      "an index outside the pool restarts at the first target",
			pool:      []string{ok, ok},
			threshold: 3,
			index:     7,
			wantIndex: 1,
		},
	}

	for _, c := range cases {
		config := DefaultConfig()
		config.ProbeURLs = c.pool
		config.ProbeFailThreshold = c.threshold
		config.ProbeTimeout = 2

		state := &State{ProbeIndex: c.index}
		blocked, reason := runProbes(config, state)

		if blocked != c.wantBlocked {
			t.Errorf("%s: blocked = %v (%s), want %v", c.name, blocked, reason, c.wantBlocked)
		}
		if state.ProbeIndex != c.wantIndex {
			t.Errorf("%s: next index = %d, want %d", c.name, state.ProbeIndex, c.wantIndex)
		}
	}
}

func TestProbeURLsFallBackToTheBuiltInPool(t *testing.T) {
	config := DefaultConfig()
	config.ProbeURLs = nil

	if got := probeURLs(config); len(got) != len(defaultProbeURLs) {
		t.Errorf("an empty pool must fall back to the built-in one, got %v", got)
	}
}

func TestAccountCheckDue(t *testing.T) {
	now := time.Now().Unix()

	cases := []struct {
		name     string
		tune     func(*Config)
		lastSeen int64
		want     bool
	}{
		{name: "never checked before", want: true},
		{name: "checked long enough ago", lastSeen: now - 3600, want: true},
		{name: "checked just now", lastSeen: now, want: false},
		{name: "checked within the interval", lastSeen: now - 60, want: false},
		{name: "state file from the future", lastSeen: now + 3600, want: true},
		{
			name:     "account check disabled",
			tune:     func(c *Config) { c.CheckAccount = false },
			lastSeen: now - 3600,
		},
		{
			name:     "no username to compare against",
			tune:     func(c *Config) { c.Username = "" },
			lastSeen: now - 3600,
		},
		{
			name:     "interval zero disables the slow lane",
			tune:     func(c *Config) { c.AccountCheckInterval = 0 },
			lastSeen: now - 3600,
		},
	}

	for _, c := range cases {
		config := DefaultConfig()
		config.Username = "2021001"
		if c.tune != nil {
			c.tune(config)
		}

		if got := accountCheckDue(config, &State{LastAccountCheck: c.lastSeen}); got != c.want {
			t.Errorf("%s: accountCheckDue = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")

	SaveState(path, &State{ProbeIndex: 3, LastAccountCheck: 1700000000})

	got := LoadState(path)
	if got.ProbeIndex != 3 || got.LastAccountCheck != 1700000000 {
		t.Errorf("state round trip = %+v, want {3 1700000000}", *got)
	}
}

func TestLoadStateSurvivesABadFile(t *testing.T) {
	dir := t.TempDir()

	if got := LoadState(filepath.Join(dir, "missing.json")); *got != (State{}) {
		t.Errorf("a missing state file must read as empty, got %+v", *got)
	}

	damaged := filepath.Join(dir, "damaged.json")
	if err := os.WriteFile(damaged, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(damaged); *got != (State{}) {
		t.Errorf("a damaged state file must read as empty, got %+v", *got)
	}
}

func TestLoadConfigKeepsProbeDefaults(t *testing.T) {
	dir := t.TempDir()

	bare := filepath.Join(dir, "bare.json")
	if err := os.WriteFile(bare, []byte(`{"username":"2021001"}`), 0600); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(bare)
	if err != nil {
		t.Fatal(err)
	}
	if !config.ProbeEnabled {
		t.Error("a config written before the probe pool existed must get the probe enabled")
	}
	if len(config.ProbeURLs) != len(defaultProbeURLs) {
		t.Errorf("pool = %v, want the built-in one", config.ProbeURLs)
	}
	if config.AccountCheckInterval != 1800 {
		t.Errorf("account check interval = %d, want 1800", config.AccountCheckInterval)
	}

	custom := filepath.Join(dir, "custom.json")
	if err := os.WriteFile(custom, []byte(`{"probe_urls":["http://a.example/generate_204"]}`), 0600); err != nil {
		t.Fatal(err)
	}

	config, err = LoadConfig(custom)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.ProbeURLs) != 1 || config.ProbeURLs[0] != "http://a.example/generate_204" {
		t.Errorf("a configured pool must replace the built-in one, got %v", config.ProbeURLs)
	}
}

func TestRunLockPreventsOverlap(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "nested", "state.json")

	release, acquired := acquireRunLock(statePath)
	if !acquired {
		t.Fatal("the first run must get the lock")
	}

	if _, acquired := acquireRunLock(statePath); acquired {
		t.Error("a second run must not get the lock while the first one holds it")
	}

	release()

	again, acquired := acquireRunLock(statePath)
	if !acquired {
		t.Error("the lock must be free again once the first run released it")
	} else {
		again()
	}
}

func TestRunLockWithoutAStateFile(t *testing.T) {
	release, acquired := acquireRunLock("")
	if !acquired {
		t.Fatal("a run with nowhere to put the lock must go ahead unlocked")
	}
	release()
}

func TestEffectiveAttempts(t *testing.T) {
	cases := []struct {
		name          string
		retryInterval int
		maxRetries    int
		want          int
	}{
		{"the defaults fit in the budget", 5, 3, 3},
		{"a long retry interval trims the attempts", 60, 100, 3},
		{"one attempt is always allowed", 3600, 100, 1},
		{"zero attempts still runs once", 5, 0, 1},
	}

	for _, c := range cases {
		config := DefaultConfig()
		config.RetryInterval = c.retryInterval
		config.MaxRetries = c.maxRetries

		if got := effectiveAttempts(config); got != c.want {
			t.Errorf("%s: effectiveAttempts(retry=%ds, max=%d) = %d, want %d",
				c.name, c.retryInterval, c.maxRetries, got, c.want)
		}
	}
}

func TestSplitList(t *testing.T) {
	got := splitList(" http://a.example/generate_204 ,, http://b.example/generate_204,")
	want := []string{"http://a.example/generate_204", "http://b.example/generate_204"}

	if len(got) != len(want) {
		t.Fatalf("splitList = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitList[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if splitList("") != nil {
		t.Errorf("an empty value must produce no entries")
	}
}
