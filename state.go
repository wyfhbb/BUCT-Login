package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// State is the little bit of memory one run leaves for the next one. Runs are
// short lived processes started by a timer or by cron, so the position in the
// probe pool and the clock of the account check cannot live in memory.
type State struct {
	ProbeIndex       int   `json:"probe_index"`        // next target of the pool
	LastAccountCheck int64 `json:"last_account_check"` // unix time of the last portal query
}

// userStateDir returns the XDG state directory of the current user
func userStateDir() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return dir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state"), nil
}

// defaultStatePath picks a location the current user can write to
func defaultStatePath() string {
	if os.Geteuid() == 0 {
		return "/var/lib/buct-login/state.json"
	}

	dir, err := userStateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "buct-login", "state.json")
}

// resolveStatePath returns the state file of this installation
func resolveStatePath(config *Config) string {
	if config.StateFile != "" {
		return config.StateFile
	}
	return defaultStatePath()
}

// acquireRunLock keeps two runs of the same installation from overlapping.
//
// systemd never starts a second copy of a unit that is still running, cron
// happily does, so the guarantee has to live in the program for the two to
// behave the same way. It also caps the damage of a config that makes one run
// last longer than the interval between runs.
//
// The lock is a safety net, so failing to take it never stops a login: the run
// goes ahead unlocked and says so.
func acquireRunLock(statePath string) (func(), bool) {
	nothingToRelease := func() {}

	if statePath == "" {
		logrus.Debug("No state file location, running without the overlap lock")
		return nothingToRelease, true
	}

	path := statePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		logrus.Warnf("Cannot create lock directory for %s: %v", path, err)
		return nothingToRelease, true
	}

	release, acquired, err := acquireLock(path)
	if err != nil {
		logrus.Warnf("Cannot lock %s, running without the overlap lock: %v", path, err)
		return nothingToRelease, true
	}
	if !acquired {
		return nil, false
	}

	logrus.Debugf("Holding the run lock on %s", path)
	return release, true
}

// LoadState reads the state file. Losing it is not an error: the rotation
// restarts at the first target and the account check runs once more than it
// would have, so a run never fails over its own bookkeeping.
func LoadState(path string) *State {
	state := &State{}
	if path == "" {
		return state
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Warnf("Cannot read state file %s: %v", path, err)
		}
		return state
	}

	if err := json.Unmarshal(data, state); err != nil {
		logrus.Warnf("Ignoring damaged state file %s: %v", path, err)
		return &State{}
	}

	logrus.Debugf("State loaded from %s: %+v", path, *state)
	return state
}

// SaveState writes the state back, reporting problems without failing the run
func SaveState(path string, state *State) {
	if path == "" {
		return
	}

	if err := writeState(path, state); err != nil {
		logrus.Warnf("Cannot write state file %s: %v", path, err)
	}
}

func writeState(path string, state *State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %v", err)
	}

	return os.WriteFile(path, data, 0644)
}
