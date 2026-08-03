package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// version is set at build time with -ldflags "-X main.version=..."
var version = "dev"

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
	} else {
		logrus.SetOutput(os.Stdout)
	}

	return nil
}

// showUsage displays usage information in Chinese
func showUsage() {
	fmt.Println("校园网登录工具")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  buct-login -action=login -username=学号 -password=密码")
	fmt.Println("  buct-login -action=info")
	fmt.Println("  buct-login -action=logout")
	fmt.Println("  buct-login -action=install-service -username=学号 -password=密码 -check-interval=60")
	fmt.Println("  buct-login -action=service-status")
	fmt.Println("  buct-login -action=uninstall-service")
	fmt.Println("  buct-login -config=config.json")
	fmt.Println("  buct-login -generate-config=config.json")
	fmt.Println("")
	fmt.Println("操作 (-action):")
	fmt.Println("  login              登录（已在线则跳过；开启账号检测时账号不匹配会先登出再登录）")
	fmt.Println("  info               查看当前登录信息")
	fmt.Println("  logout             登出")
	fmt.Println("  install-service    安装 systemd 定时服务（周期性检查并自动登录，开机自动生效）")
	fmt.Println("  uninstall-service  卸载 systemd 定时服务")
	fmt.Println("  service-status     查看 systemd 定时服务状态")
	fmt.Println("")
	fmt.Println("参数:")
	fmt.Println("  -username          用户名 (login 和 install-service 必需)")
	fmt.Println("  -password          密码 (login 和 install-service 必需)")
	fmt.Println("  -config            配置文件路径 (默认自动查找 ./config.json、~/.config/buct-login/config.json、/etc/buct-login/config.json)")
	fmt.Println("  -generate-config   生成配置文件模板")
	fmt.Println("  -check-interval    systemd 检查间隔(秒) (默认: 60)")
	fmt.Println("  -retry-interval    单次运行内的重试间隔(秒) (默认: 5)")
	fmt.Println("  -max-retries       单次运行的最大尝试次数 (默认: 3)")
	fmt.Println("  -check-account     账号不匹配检测，开启后若在线账号与配置不符则登出重登 (默认: true)")
	fmt.Println("  -user              安装/卸载/查看用户级 systemd 服务 (默认为系统级，需要 sudo)")
	fmt.Println("  -logfile           日志文件路径 (默认: buct-login.log)")
	fmt.Println("  -nolog             不保存日志文件，只输出到控制台")
	fmt.Println("  -quiet             静默模式，减少日志输出")
	fmt.Println("  -version           显示版本号")
	fmt.Println("")
	fmt.Println("网站池探测 (减少对校园网门户的访问):")
	fmt.Println("  -probe                    先探测网站池，探测失败才去查门户 (默认: true)")
	fmt.Println("  -probe-urls               网站池，逗号分隔 (默认为内置的 5 个国内 204 探测端点)")
	fmt.Println("  -probe-timeout            单次探测超时(秒) (默认: 5)")
	fmt.Println("  -probe-attempts           连续多少个站点无响应才判定断网 (默认: 3)")
	fmt.Println("  -account-check-interval   网络正常时查一次门户核对账号的间隔(秒)，0 为关闭 (默认: 1800)")
	fmt.Println("  -statefile                状态文件路径，记录轮询位置与上次账号核对时间")
	fmt.Println("")
	fmt.Println("配置文件示例:")
	fmt.Println(`  {
    "username": "your_student_id",
    "password": "your_password",
    "action": "login",
    "check_interval": 60,
    "retry_interval": 5,
    "max_retries": 3,
    "check_account": true,
    "probe_enabled": true,
    "probe_urls": [
      "http://connect.rom.miui.com/generate_204",
      "http://connectivitycheck.platform.hicloud.com/generate_204",
      "http://wifi.vivo.com.cn/generate_204",
      "http://www.qualcomm.cn/generate_204",
      "http://204.ustclug.org/"
    ],
    "probe_timeout": 5,
    "probe_fail_threshold": 3,
    "account_check_interval": 1800,
    "state_file": "/var/lib/buct-login/state.json",
    "log_file": "/var/log/buct-login.log",
    "quiet": false,
    "no_log": false
  }`)
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  buct-login -action=login -username=yourschoolID -password=yourpassword")
	fmt.Println("  sudo buct-login -action=install-service -username=yourschoolID -password=yourpassword -check-interval=120")
	fmt.Println("  buct-login -action=install-service -user -config=config.json")
	fmt.Println("  buct-login -action=install-service -username=yourschoolID -password=yourpassword -check-account=false")
	fmt.Println("  buct-login -action=login -probe-urls=http://connect.rom.miui.com/generate_204,https://www.baidu.com")
	fmt.Println("  buct-login -action=login -probe=false    # 关闭网站池，每次都直接查门户")
	fmt.Println("  sudo journalctl -u buct-login.service -f    # 查看自动登录日志")
}

// splitList turns a comma separated flag value into a clean list
func splitList(value string) []string {
	var list []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			list = append(list, item)
		}
	}
	return list
}

// isServiceAction reports whether the action manages systemd instead of the network
func isServiceAction(action string) bool {
	switch action {
	case "install-service", "uninstall-service", "service-status":
		return true
	}
	return false
}

func main() {
	var (
		action         = flag.String("action", "", "Action: login|info|logout|install-service|uninstall-service|service-status")
		username       = flag.String("username", "", "Username")
		password       = flag.String("password", "", "Password")
		configPath     = flag.String("config", "", "Configuration file path")
		generateConfig = flag.String("generate-config", "", "Generate configuration template")
		checkInterval  = flag.Int("check-interval", 60, "systemd check interval in seconds")
		retryInterval  = flag.Int("retry-interval", 5, "Retry interval in seconds")
		maxRetries     = flag.Int("max-retries", 3, "Max attempts per run")
		checkAccount   = flag.Bool("check-account", true, "Logout and re-login when another account is online")
		probe          = flag.Bool("probe", true, "Probe the site pool first and only ask the portal when it fails")
		probePool      = flag.String("probe-urls", "", "Site pool, comma separated")
		probeTimeout   = flag.Int("probe-timeout", 5, "Timeout of one probe in seconds")
		probeAttempts  = flag.Int("probe-attempts", 3, "Silent targets in a row before the portal is asked")
		accountCheck   = flag.Int("account-check-interval", 1800, "Seconds between two periodic account checks, 0 disables them")
		stateFile      = flag.String("statefile", "", "State file path (probe rotation and account check clock)")
		userScope      = flag.Bool("user", false, "Use the user systemd instance instead of the system one")
		logFile        = flag.String("logfile", "buct-login.log", "Log file path")
		noLog          = flag.Bool("nolog", false, "Don't save log file")
		quiet          = flag.Bool("quiet", false, "Quiet mode")
		showVersion    = flag.Bool("version", false, "Print version and exit")
	)

	flag.Usage = showUsage
	flag.Parse()

	if *showVersion {
		fmt.Printf("buct-login %s\n", version)
		os.Exit(0)
	}

	// Only the flags present on the command line may override the config file
	passed := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { passed[f.Name] = true })

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

	// Fall back to the well known config locations when none is given
	resolvedConfig := *configPath
	if resolvedConfig == "" {
		resolvedConfig = FindConfig()
	}

	config, err := LoadConfig(resolvedConfig)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	overrides := map[string]func(){
		"action":                 func() { config.Action = *action },
		"username":               func() { config.Username = *username },
		"password":               func() { config.Password = *password },
		"check-interval":         func() { config.CheckInterval = *checkInterval },
		"retry-interval":         func() { config.RetryInterval = *retryInterval },
		"max-retries":            func() { config.MaxRetries = *maxRetries },
		"check-account":          func() { config.CheckAccount = *checkAccount },
		"probe":                  func() { config.ProbeEnabled = *probe },
		"probe-urls":             func() { config.ProbeURLs = splitList(*probePool) },
		"probe-timeout":          func() { config.ProbeTimeout = *probeTimeout },
		"probe-attempts":         func() { config.ProbeFailThreshold = *probeAttempts },
		"account-check-interval": func() { config.AccountCheckInterval = *accountCheck },
		"statefile":              func() { config.StateFile = *stateFile },
		"logfile":                func() { config.LogFile = *logFile },
		"nolog":                  func() { config.NoLog = *noLog },
		"quiet":                  func() { config.Quiet = *quiet },
	}
	for name, apply := range overrides {
		if passed[name] {
			apply()
		}
	}

	if config.Action == "" {
		showUsage()
		os.Exit(1)
	}

	// Initialize logger, service management only ever writes to the terminal
	var logPath string
	if !config.NoLog && !isServiceAction(config.Action) {
		logPath = config.LogFile
	}

	err = initLogger(logPath, config.Quiet)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logrus.Debugf("Configuration loaded - Action: %s, ConfigFile: %s, CheckInterval: %ds, RetryInterval: %ds, MaxRetries: %d, CheckAccount: %v, LogFile: %s, Quiet: %v",
		config.Action, resolvedConfig, config.CheckInterval, config.RetryInterval,
		config.MaxRetries, config.CheckAccount, config.LogFile, config.Quiet)
	logrus.Debugf("Probe settings - Enabled: %v, Targets: %v, Timeout: %ds, FailThreshold: %d, AccountCheckInterval: %ds, StateFile: %s",
		config.ProbeEnabled, probeURLs(config), config.ProbeTimeout, config.ProbeFailThreshold,
		config.AccountCheckInterval, resolveStatePath(config))

	switch config.Action {
	case "login":
		err = handleLogin(config)
	case "info":
		err = handleInfo()
	case "logout":
		err = handleLogout()
	case "install-service":
		// Only an explicit -config pins the unit to that file, a config picked up
		// from the search paths is copied to the standard location of the scope
		err = installService(config, *userScope, *configPath)
	case "uninstall-service":
		err = uninstallService(*userScope, *configPath)
	case "service-status":
		err = serviceStatus(*userScope)
	case "monitor":
		fmt.Println("monitor 模式已移除，请改用 systemd 定时服务:")
		fmt.Println("  sudo buct-login -action=install-service -username=学号 -password=密码 -check-interval=60")
		os.Exit(1)
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
