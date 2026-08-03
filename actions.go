package main

import (
	"buct-login/utils"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ErrCredentials means the account or the password is wrong, retrying is pointless
var ErrCredentials = errors.New("invalid username or password")

// queryStatus returns the portal user info and whether the device is online
func queryStatus() (map[string]interface{}, bool, error) {
	info, err := utils.GetUserInfo()
	if err != nil {
		return nil, false, err
	}

	logrus.Debugf("User info response: %v", info)

	if errorMsg, exists := info["error"]; exists {
		switch errorMsg {
		case "not_online_error":
			return info, false, nil
		case "ok":
			// online
		default:
			return nil, false, fmt.Errorf("failed to query status: %v", errorMsg)
		}
	}

	return info, true, nil
}

// accountName extracts the account currently online
func accountName(info map[string]interface{}) string {
	if userName, ok := info["user_name"].(string); ok {
		return strings.TrimSpace(userName)
	}
	return ""
}

// accountMatches reports whether the online account is the configured one.
// The portal may append an ISP suffix (e.g. 2021xxxx@cmcc), so a configured
// name without a suffix matches any suffix.
func accountMatches(expected, online string) bool {
	expected = strings.TrimSpace(expected)
	online = strings.TrimSpace(online)

	if strings.EqualFold(expected, online) {
		return true
	}

	if !strings.Contains(expected, "@") {
		if i := strings.Index(online, "@"); i >= 0 {
			return strings.EqualFold(expected, online[:i])
		}
	}

	return false
}

// performLogin sends a single login request
func performLogin(username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("username and password cannot be empty")
	}

	ip, _, err := utils.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get status: %v", err)
	}

	logrus.Infof("Attempting login with IP: %s", ip)
	res, err := utils.Login(username, password, "20", ip)
	if err != nil {
		return fmt.Errorf("login request failed: %v", err)
	}

	logrus.Debugf("Login response received: %v", res)

	if resStatus, ok := res["res"]; ok && resStatus == "ok" {
		logrus.Info("Login successful")
		return nil
	}

	if ecode, ok := res["ecode"]; ok && ecode == "E2901" {
		return ErrCredentials
	}

	if errorMsg, ok := res["error"]; ok {
		return fmt.Errorf("login error: %v", errorMsg)
	}

	return fmt.Errorf("unknown login error: %v", res)
}

// performLogout sends a single logout request
func performLogout() error {
	ip, loginStatus, err := utils.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get status: %v", err)
	}

	if !loginStatus {
		logrus.Info("Not logged in")
		return nil
	}

	if ip == "" {
		return fmt.Errorf("cannot get IP address")
	}

	logrus.Infof("Local IP address: %s", ip)
	logrus.Info("Attempting logout")

	ret, err := utils.Logout(ip, "20")
	if err != nil {
		return fmt.Errorf("logout request failed: %v", err)
	}

	logrus.Debugf("Logout response received: %v", ret)

	errorMsg, ok := ret["error"]
	if !ok {
		return fmt.Errorf("unknown logout response: %v", ret)
	}

	if errorMsg != "ok" {
		return fmt.Errorf("logout error: %v", errorMsg)
	}

	logrus.Info("Logout successful")
	return nil
}

// ensureOnline makes one attempt at bringing the device online with the
// configured account, logging out first when another account holds the session.
//
// loginWhenOffline tells whether a portal reporting no session should lead to a
// login. The probe path wants that; the slow lane does not, because it only
// runs when the public internet is reachable, and then a portal with no session
// means this machine is simply not behind the portal (tethering, home network),
// where a login attempt would only fail.
func ensureOnline(config *Config, loginWhenOffline bool) error {
	info, online, err := queryStatus()
	if err != nil {
		return err
	}

	if online {
		account := accountName(info)

		switch {
		case !config.CheckAccount:
			logrus.Infof("Already logged in as %s (account check disabled)", account)
			logUserInfo(info)
			return nil
		case config.Username == "":
			logrus.Warn("No username configured, skipping account check")
			logUserInfo(info)
			return nil
		case account == "":
			logrus.Warn("Cannot read the online account name, skipping account check")
			logUserInfo(info)
			return nil
		case accountMatches(config.Username, account):
			logrus.Infof("Already logged in as %s", account)
			logUserInfo(info)
			return nil
		}

		logrus.Warnf("Account mismatch: online as %s, expected %s", account, config.Username)
		if err := performLogout(); err != nil {
			return fmt.Errorf("logout before re-login failed: %v", err)
		}
		time.Sleep(2 * time.Second)
	} else {
		logrus.Info("Not logged in")
		if !loginWhenOffline {
			logrus.Info("The public network is reachable, this machine is not behind the portal, skipping login")
			return nil
		}
	}

	if err := performLogin(config.Username, config.Password); err != nil {
		return err
	}

	// Wait a moment for the portal status to update
	time.Sleep(1 * time.Second)
	if info, err := utils.GetUserInfo(); err == nil {
		logUserInfo(info)
	}

	return nil
}

// maxRunSeconds bounds the worst case duration of a single run. Runs are
// started by a timer or by cron and are expected to be over long before the
// next one begins, so a config asking for more retries than fit in that budget
// gets trimmed rather than left to overrun.
const maxRunSeconds = 300

// portalAttemptSeconds is what the portal round trips of one attempt can cost
// before its retry interval even starts
const portalAttemptSeconds = 20

// effectiveAttempts returns how many attempts of this config fit in the run
// budget. install-service warns about an oversized retry setting, but a
// hand-edited config file or a cron entry never passes through it, so the
// limit has to hold at run time too.
func effectiveAttempts(config *Config) int {
	attempts := config.MaxRetries
	if attempts < 1 {
		return 1
	}

	perAttempt := config.RetryInterval + portalAttemptSeconds
	if perAttempt < 1 {
		perAttempt = 1
	}

	allowed := (maxRunSeconds - runProbeBudget(config)) / perAttempt
	if allowed < 1 {
		allowed = 1
	}

	if attempts > allowed {
		return allowed
	}
	return attempts
}

// retryEnsureOnline runs ensureOnline until it succeeds or the attempts of this
// run are used up. Wrong credentials end it immediately, retrying them would
// only lock the account out faster.
func retryEnsureOnline(config *Config, loginWhenOffline bool) error {
	attempts := effectiveAttempts(config)
	if attempts < config.MaxRetries {
		logrus.Warnf("max_retries %d with a %ds retry interval would let one run last past %ds, using %d attempts instead",
			config.MaxRetries, config.RetryInterval, maxRunSeconds, attempts)
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			logrus.Infof("Attempt %d/%d", attempt, attempts)
		}

		err := ensureOnline(config, loginWhenOffline)
		if err == nil {
			return nil
		}
		lastErr = err

		if errors.Is(err, ErrCredentials) {
			logrus.Error("Login failed: invalid username or password")
			return err
		}

		logrus.Errorf("Attempt %d/%d failed: %v", attempt, attempts, err)
		if attempt < attempts {
			logrus.Infof("Retrying in %d seconds", config.RetryInterval)
			time.Sleep(time.Duration(config.RetryInterval) * time.Second)
		}
	}

	return lastErr
}

// accountCheckDue reports whether the periodic account check is due.
//
// The probe pool can tell that the network works, never which account made it
// work, so a session opened with the wrong account would keep the internet
// running and never be noticed. Asking the portal on a long interval catches
// that without turning every run into a portal request.
func accountCheckDue(config *Config, state *State) bool {
	if !config.CheckAccount {
		return false
	}

	if config.Username == "" {
		logrus.Debug("No username configured, skipping the account check")
		return false
	}

	if config.AccountCheckInterval <= 0 {
		logrus.Debug("Periodic account check disabled")
		return false
	}

	elapsed := time.Now().Unix() - state.LastAccountCheck
	// A state file from the future (clock jump, restored backup) must not park
	// the check forever
	if elapsed < 0 {
		return true
	}

	if remaining := int64(config.AccountCheckInterval) - elapsed; remaining > 0 {
		logrus.Infof("Public network is fine, next account check in %ds", remaining)
		return false
	}

	return true
}

// handleLogin is the entry point used by the systemd timer and by cron, so a
// run is expected to be short lived and to report failure through its exit code.
//
// With the probe pool enabled a run asks the portal in only two cases: the
// public internet did not answer, or the periodic account check came due. That
// keeps a machine that is online and correct from touching the portal at all,
// which is the whole point of the pool.
func handleLogin(config *Config) error {
	statePath := resolveStatePath(config)

	release, acquired := acquireRunLock(statePath)
	if !acquired {
		logrus.Info("A previous run is still in progress, skipping this one")
		return nil
	}
	defer release()

	if !config.ProbeEnabled {
		logrus.Info("Starting login process")
		return retryEnsureOnline(config, true)
	}

	// Registered after the lock, so the state is written while it is still held
	state := LoadState(statePath)
	defer SaveState(statePath, state)

	blocked, reason := runProbes(config, state)

	if blocked {
		logrus.Infof("Public network unreachable (%s), asking the portal", reason)
		err := retryEnsureOnline(config, true)
		if err == nil {
			// The portal was just asked, so the slow lane clock restarts here
			state.LastAccountCheck = time.Now().Unix()
		}
		return err
	}

	if !accountCheckDue(config, state) {
		return nil
	}

	logrus.Info("Public network is fine, running the periodic account check")
	err := retryEnsureOnline(config, false)
	if err == nil {
		state.LastAccountCheck = time.Now().Unix()
	}
	return err
}

// handleLogout performs logout operation
func handleLogout() error {
	logrus.Info("Starting logout process")
	return performLogout()
}

// handleInfo displays current user information
func handleInfo() error {
	logrus.Info("Getting user information")

	info, online, err := queryStatus()
	if err != nil {
		return err
	}

	if !online {
		fmt.Println("当前状态: 未登录")
		return nil
	}

	fmt.Println("当前状态: 已登录")

	if userName, ok := info["user_name"]; ok {
		fmt.Printf("Current account: %s\n", userName)
	}

	if addTime, ok := info["add_time"].(float64); ok {
		loginTime := time.Unix(int64(addTime), 0)
		fmt.Printf("Login time: %s\n", loginTime.Format("2006-01-02 15:04:05"))
	}

	if allBytes, ok := info["all_bytes"].(float64); ok {
		fmt.Printf("Session traffic: %s\n", formatBytes(allBytes))
	}

	if onlineIP, ok := info["online_ip"]; ok {
		fmt.Printf("Online IP: %s\n", onlineIP)
	}

	if userMac, ok := info["user_mac"]; ok {
		fmt.Printf("MAC address: %s\n", userMac)
	}

	if sumBytes, ok := info["sum_bytes"].(float64); ok {
		fmt.Printf("Total traffic: %s\n", formatBytes(sumBytes))
	}

	if sumSeconds, ok := info["sum_seconds"].(float64); ok {
		duration := time.Duration(sumSeconds) * time.Second
		fmt.Printf("Total time: %s\n", duration.String())
	}

	if userBalance, ok := info["user_balance"].(float64); ok {
		fmt.Printf("Balance: %.2f yuan\n", userBalance)
	}

	if userCharge, ok := info["user_charge"].(float64); ok {
		fmt.Printf("Monthly charges: %.2f yuan\n", userCharge)
	}

	return nil
}

// logUserInfo logs detailed user information
func logUserInfo(info map[string]interface{}) {
	if userName, ok := info["user_name"]; ok {
		logrus.Infof("Current account: %v", userName)
	}

	if addTime, ok := info["add_time"].(float64); ok {
		loginTime := time.Unix(int64(addTime), 0)
		logrus.Infof("Login time: %s", loginTime.Format("2006-01-02 15:04:05"))
	}

	if allBytes, ok := info["all_bytes"].(float64); ok {
		logrus.Infof("Traffic used this session: %s", formatBytes(allBytes))
	}

	if onlineIP, ok := info["online_ip"]; ok {
		logrus.Infof("Online IP: %v", onlineIP)
	}

	if userMac, ok := info["user_mac"]; ok {
		logrus.Infof("MAC address: %v", userMac)
	}

	if sumBytes, ok := info["sum_bytes"].(float64); ok {
		logrus.Infof("Total traffic used: %s", formatBytes(sumBytes))
	}

	if sumSeconds, ok := info["sum_seconds"].(float64); ok {
		duration := time.Duration(sumSeconds) * time.Second
		logrus.Infof("Total online time: %s", duration.String())
	}

	if userBalance, ok := info["user_balance"].(float64); ok {
		logrus.Infof("Account balance: %.2f yuan", userBalance)
	}

	if userCharge, ok := info["user_charge"].(float64); ok {
		logrus.Infof("Monthly charges: %.2f yuan", userCharge)
	}
}

// formatBytes converts bytes to human readable format
func formatBytes(bytes float64) string {
	if bytes >= 1024*1024*1024 {
		return fmt.Sprintf("%.2fGB", bytes/(1024*1024*1024))
	} else if bytes >= 1024*1024 {
		return fmt.Sprintf("%.2fMB", bytes/(1024*1024))
	} else if bytes >= 1024 {
		return fmt.Sprintf("%.2fKB", bytes/1024)
	}
	return fmt.Sprintf("%.0fb", bytes)
}
