package main

import (
	"buct-login/utils"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// Config represents the configuration structure
type Config struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	Action          string `json:"action"`
	MonitorInterval int    `json:"monitor_interval"` // seconds
	RetryInterval   int    `json:"retry_interval"`   // seconds
	LogFile         string `json:"log_file"`
	Quiet           bool   `json:"quiet"`
	NoLog           bool   `json:"no_log"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Action:          "login",
		MonitorInterval: 60, // 1 minute
		RetryInterval:   5,  // 5 seconds
		LogFile:         "buct-login.log",
		Quiet:           false,
		NoLog:           false,
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

// SaveConfig saves configuration to file
func SaveConfig(config *Config, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// CustomFormatter defines a Python-like log format
type CustomFormatter struct{}

// Format implements the logrus.Formatter interface
func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timestamp := entry.Time.Format("2006-01-02 15:04:05")
	level := fmt.Sprintf("%-5s", entry.Level.String())

	// Convert level to uppercase
	switch entry.Level {
	case logrus.DebugLevel:
		level = "DEBUG"
	case logrus.InfoLevel:
		level = "INFO "
	case logrus.WarnLevel:
		level = "WARN "
	case logrus.ErrorLevel:
		level = "ERROR"
	case logrus.FatalLevel:
		level = "FATAL"
	case logrus.PanicLevel:
		level = "PANIC"
	}

	msg := entry.Message

	// Add fields if they exist
	if len(entry.Data) > 0 {
		for key, value := range entry.Data {
			msg += fmt.Sprintf(" %s=%v", key, value)
		}
	}

	return []byte(fmt.Sprintf("%s %s %s\n", level, timestamp, msg)), nil
}

// initLogger initializes the logger with detailed configuration
func initLogger(logFile string, quiet bool) error {
	logrus.SetFormatter(&CustomFormatter{})

	if quiet {
		logrus.SetLevel(logrus.ErrorLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	if logFile != "" {
		dir := filepath.Dir(logFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %v", err)
		}

		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to create log file %s: %v", logFile, err)
		}

		multiWriter := io.MultiWriter(os.Stdout, file)
		logrus.SetOutput(multiWriter)

		fmt.Printf("Log will be saved to: %s\n", logFile)
	} else {
		logrus.SetOutput(os.Stdout)
	}

	return nil
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

// handleLogin performs login operation with detailed logging
func handleLogin(username, password string) error {
	logrus.Info("Starting login process")

	if username == "" || password == "" {
		logrus.Error("Username and password cannot be empty")
		return fmt.Errorf("username and password cannot be empty")
	}

	logrus.Debug("Getting current status")
	ip, loginStatus, err := utils.GetStatus()
	if err != nil {
		logrus.Errorf("Failed to get status: %v", err)
		return err
	}

	logrus.Debugf("Current status retrieved - IP: %s, LoginStatus: %v", ip, loginStatus)

	if loginStatus {
		logrus.Info("Already logged in")
		// Get detailed info for logged in user
		info, err := utils.GetUserInfo()
		if err == nil {
			logUserInfo(info)
		}
		return nil
	}

	logrus.Infof("Attempting login with IP: %s", ip)
	res, err := utils.Login(username, password, "20", ip)
	if err != nil {
		logrus.Errorf("Login request failed: %v", err)
		return err
	}

	logrus.Debugf("Login response received: %v", res)

	if resStatus, ok := res["res"]; ok && resStatus == "ok" {
		logrus.Info("Login successful")
		// Log detailed info after successful login
		time.Sleep(1 * time.Second) // Wait a moment for status to update
		info, err := utils.GetUserInfo()
		if err == nil {
			logUserInfo(info)
		}
	} else if ecode, ok := res["ecode"]; ok && ecode == "E2901" {
		logrus.Error("Login failed: invalid username or password")
		return fmt.Errorf("login failed: invalid username or password")
	} else if errorMsg, ok := res["error"]; ok {
		logrus.Errorf("Login error: %v", errorMsg)
		return fmt.Errorf("login error: %v", errorMsg)
	} else {
		logrus.Errorf("Unknown login error: %v", res)
		return fmt.Errorf("unknown login error: %v", res)
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

// handleLookup displays current user information
func handleLookup() (string, error) {
	logrus.Info("Getting user information")

	info, err := utils.GetUserInfo()
	if err != nil {
		logrus.Errorf("Failed to get user information: %v", err)
		return "error", err
	}

	if errorMsg, exists := info["error"]; exists {
		if errorMsg == "not_online_error" {
			logrus.Info("Not logged in")
			return "not_login", nil
		} else if errorMsg != "ok" {
			logrus.Errorf("Error getting user info: %v", errorMsg)
			return "error", nil
		}
	}

	// Display user information
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

	logUserInfo(info)
	return "already_login", nil
}

// handleLogout performs logout operation
func handleLogout() error {
	logrus.Info("Starting logout process")

	ip, loginStatus, err := utils.GetStatus()
	if err != nil {
		logrus.Errorf("Failed to get status: %v", err)
		return err
	}

	if ip == "" {
		logrus.Error("Cannot get IP address")
		return fmt.Errorf("cannot get IP address")
	}

	if !loginStatus {
		logrus.Info("Not logged in")
		return nil
	}

	logrus.Infof("Local IP address: %s", ip)
	logrus.Info("Attempting logout")

	ret, err := utils.Logout(ip, "20")
	if err != nil {
		logrus.Errorf("Logout request failed: %v", err)
		return err
	}

	logrus.Debugf("Logout response received: %v", ret)

	if errorMsg, ok := ret["error"]; ok {
		if errorMsg == "ok" {
			logrus.Info("Logout successful")
		} else {
			logrus.Errorf("Logout error: %v", errorMsg)
			return fmt.Errorf("logout error: %v", errorMsg)
		}
	} else {
		logrus.Errorf("Unknown logout response: %v", ret)
		return fmt.Errorf("unknown logout response: %v", ret)
	}

	return nil
}

// handleMonitor monitors login status with configurable intervals
func handleMonitor(username, password string, monitorInterval, retryInterval int) error {
	logrus.Infof("Starting monitor mode - MonitorInterval: %ds, RetryInterval: %ds", monitorInterval, retryInterval)

	for {
		status, err := handleLookup()
		if err != nil {
			logrus.Errorf("Failed to check status: %v", err)
			logrus.Infof("Retrying in %d seconds", retryInterval)
			time.Sleep(time.Duration(retryInterval) * time.Second)
			continue
		}

		if status == "not_login" {
			logrus.Info("Detected offline status, attempting auto-login")
			err := handleLogin(username, password)
			if err != nil {
				logrus.Errorf("Auto-login failed: %v", err)
				logrus.Infof("Retrying in %d seconds", retryInterval)
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			logrus.Info("Currently online")
		}

		logrus.Infof("Next check in %d seconds", monitorInterval)
		time.Sleep(time.Duration(monitorInterval) * time.Second)
	}
}

// generateConfigTemplate generates a template configuration file
func generateConfigTemplate(configPath string) error {
	config := DefaultConfig()
	config.Username = "your_student_id"
	config.Password = "your_password"

	return SaveConfig(config, configPath)
}

// showUsage displays usage information in Chinese
func showUsage() {
	fmt.Println("校园网登录工具")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  buct-login -action=login -username=学号 -password=密码")
	fmt.Println("  buct-login -action=info")
	fmt.Println("  buct-login -action=logout")
	fmt.Println("  buct-login -action=monitor -username=学号 -password=密码")
	fmt.Println("  buct-login -config=config.json")
	fmt.Println("  buct-login -generate-config=config.json")
	fmt.Println("")
	fmt.Println("参数:")
	fmt.Println("  -action            操作类型: login|info|logout|monitor")
	fmt.Println("  -username          用户名 (login和monitor模式必需)")
	fmt.Println("  -password          密码 (login和monitor模式必需)")
	fmt.Println("  -config            配置文件路径")
	fmt.Println("  -generate-config   生成配置文件模板")
	fmt.Println("  -monitor-interval  监控间隔(秒) (默认: 60)")
	fmt.Println("  -retry-interval    重试间隔(秒) (默认: 5)")
	fmt.Println("  -logfile           日志文件路径 (默认: buct-login.log)")
	fmt.Println("  -nolog             不保存日志文件，只输出到控制台")
	fmt.Println("  -quiet             静默模式，减少日志输出")
	fmt.Println("")
	fmt.Println("配置文件示例:")
	fmt.Println(`  {
    "username": "your_student_id",
    "password": "your_password",
    "action": "monitor",
    "monitor_interval": 60,
    "retry_interval": 5,
    "log_file": "logs/buct-login.log",
    "quiet": false,
    "no_log": false
  }`)
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  buct-login -action=login -username=yourschoolID -password=yourpassword")
	fmt.Println("  buct-login -action=monitor -username=yourschoolID -password=yourpassword -monitor-interval=30")
	fmt.Println("  buct-login -config=config.json")
	fmt.Println("  buct-login -generate-config=config.json")
	fmt.Println("  buct-login -action=info -logfile=./logs/network.log")
}

func main() {
	var (
		action          = flag.String("action", "", "Action type: login|info|logout|monitor")
		username        = flag.String("username", "", "Username")
		password        = flag.String("password", "", "Password")
		configPath      = flag.String("config", "", "Configuration file path")
		generateConfig  = flag.String("generate-config", "", "Generate configuration template")
		monitorInterval = flag.Int("monitor-interval", 60, "Monitor interval in seconds")
		retryInterval   = flag.Int("retry-interval", 5, "Retry interval in seconds")
		logFile         = flag.String("logfile", "buct-login.log", "Log file path")
		noLog           = flag.Bool("nolog", false, "Don't save log file")
		quiet           = flag.Bool("quiet", false, "Quiet mode")
	)

	flag.Parse()

	// Generate config template if requested
	if *generateConfig != "" {
		err := generateConfigTemplate(*generateConfig)
		if err != nil {
			fmt.Printf("Failed to generate config template: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Configuration template saved to: %s\n", *generateConfig)
		os.Exit(0)
	}

	// Load configuration
	config, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override config with command line arguments
	if *action != "" {
		config.Action = *action
	}
	if *username != "" {
		config.Username = *username
	}
	if *password != "" {
		config.Password = *password
	}
	if *monitorInterval != 60 {
		config.MonitorInterval = *monitorInterval
	}
	if *retryInterval != 5 {
		config.RetryInterval = *retryInterval
	}
	if *logFile != "buct-login.log" {
		config.LogFile = *logFile
	}
	if *noLog {
		config.NoLog = *noLog
	}
	if *quiet {
		config.Quiet = *quiet
	}

	// Initialize logger
	var logPath string
	if !config.NoLog {
		logPath = config.LogFile
	}

	err = initLogger(logPath, config.Quiet)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Log configuration
	logrus.Debugf("Configuration loaded - Action: %s, MonitorInterval: %ds, RetryInterval: %ds, LogFile: %s, Quiet: %v",
		config.Action, config.MonitorInterval, config.RetryInterval, config.LogFile, config.Quiet)

	if config.Action == "" {
		showUsage()
		os.Exit(1)
	}

	switch config.Action {
	case "login":
		err = handleLogin(config.Username, config.Password)
	case "info":
		_, err = handleLookup()
	case "logout":
		err = handleLogout()
	case "monitor":
		err = handleMonitor(config.Username, config.Password, config.MonitorInterval, config.RetryInterval)
	default:
		fmt.Printf("Unknown action: %s\n", config.Action)
		showUsage()
		os.Exit(1)
	}

	if err != nil {
		logrus.Errorf("Operation failed: %v", err)
		os.Exit(1)
	}
}
