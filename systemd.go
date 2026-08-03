package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	serviceUnit = "buct-login.service"
	timerUnit   = "buct-login.timer"
)

// systemdEnv describes where the units, the config and the log of one
// installation live. A user scope installation runs under `systemctl --user`
// and needs no root, a system scope one is started at boot for the whole machine.
type systemdEnv struct {
	UserScope  bool
	UnitDir    string
	ConfigPath string
	LogPath    string
	StatePath  string
	BinaryPath string
}

// newSystemdEnv resolves all paths needed to install or remove the units
func newSystemdEnv(userScope bool, configPath string) (*systemdEnv, error) {
	env := &systemdEnv{UserScope: userScope}

	if userScope {
		dir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("cannot locate user config directory: %v", err)
		}
		env.UnitDir = filepath.Join(dir, "systemd", "user")
		env.ConfigPath = filepath.Join(dir, "buct-login", "config.json")

		stateDir, err := userStateDir()
		if err != nil {
			return nil, fmt.Errorf("cannot locate state directory: %v", err)
		}
		env.LogPath = filepath.Join(stateDir, "buct-login", "buct-login.log")
		env.StatePath = filepath.Join(stateDir, "buct-login", "state.json")
	} else {
		env.UnitDir = "/etc/systemd/system"
		env.ConfigPath = "/etc/buct-login/config.json"
		env.LogPath = "/var/log/buct-login.log"
		env.StatePath = "/var/lib/buct-login/state.json"
	}

	if configPath != "" {
		abs, err := filepath.Abs(configPath)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve config path: %v", err)
		}
		env.ConfigPath = abs
	}

	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot locate the current executable: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	env.BinaryPath = exe

	return env, nil
}

func (e *systemdEnv) servicePath() string {
	return filepath.Join(e.UnitDir, serviceUnit)
}

func (e *systemdEnv) timerPath() string {
	return filepath.Join(e.UnitDir, timerUnit)
}

// quoteUnitArg quotes a path when systemd would otherwise split it on spaces
func quoteUnitArg(value string) string {
	if strings.ContainsAny(value, " \t\"'\\") {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}

// renderService builds the oneshot unit that performs a single login run
func (e *systemdEnv) renderService(config *Config) string {
	var b strings.Builder

	// Give one run enough time to walk the probe pool and then exhaust its
	// retries before systemd kills it
	timeout := config.RetryInterval*effectiveAttempts(config) + runProbeBudget(config) + 60

	b.WriteString("[Unit]\n")
	b.WriteString("Description=BUCT campus network login\n")
	b.WriteString("Documentation=https://github.com/wyfhbb/BUCT-Login\n")
	if !e.UserScope {
		b.WriteString("Wants=network-online.target\n")
		b.WriteString("After=network-online.target\n")
	}
	b.WriteString("StartLimitIntervalSec=0\n")
	b.WriteString("\n")

	b.WriteString("[Service]\n")
	b.WriteString("Type=oneshot\n")
	b.WriteString(fmt.Sprintf("ExecStart=%s -action=login -config=%s\n",
		quoteUnitArg(e.BinaryPath), quoteUnitArg(e.ConfigPath)))
	b.WriteString(fmt.Sprintf("TimeoutStartSec=%d\n", timeout))
	if !e.UserScope {
		b.WriteString("NoNewPrivileges=true\n")
		b.WriteString("PrivateTmp=true\n")
	}

	// No [Install] section: the unit is meant to be started by the timer only

	return b.String()
}

// renderTimer builds the timer that replaces the old built-in monitor loop:
// it survives reboots and keeps firing at the configured interval
func (e *systemdEnv) renderTimer(config *Config) string {
	var b strings.Builder

	b.WriteString("[Unit]\n")
	b.WriteString(fmt.Sprintf("Description=BUCT campus network login check every %ds\n", config.CheckInterval))
	b.WriteString("\n")

	b.WriteString("[Timer]\n")
	if e.UserScope {
		b.WriteString("OnStartupSec=30s\n")
	} else {
		b.WriteString("OnBootSec=30s\n")
	}
	b.WriteString(fmt.Sprintf("OnUnitActiveSec=%ds\n", config.CheckInterval))
	b.WriteString("AccuracySec=1s\n")
	b.WriteString(fmt.Sprintf("Unit=%s\n", serviceUnit))
	b.WriteString("\n")

	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=timers.target\n")

	return b.String()
}

// systemctl runs systemctl in the right scope with output attached to the terminal
func systemctl(userScope bool, args ...string) error {
	if userScope {
		args = append([]string{"--user"}, args...)
	}

	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// requireSystemctl makes sure systemd is available on this machine
func requireSystemctl() error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl not found, this system does not use systemd")
	}
	return nil
}

// requireWritableScope makes sure the unit directory of the scope can be written
func requireWritableScope(userScope bool) error {
	if !userScope && os.Geteuid() != 0 {
		return fmt.Errorf("系统级服务需要 root 权限，请使用 sudo，或加上 -user 安装用户级服务")
	}
	return nil
}

// installService writes the systemd units and enables the timer
func installService(config *Config, userScope bool, configPath string) error {
	if err := requireSystemctl(); err != nil {
		return err
	}
	if err := requireWritableScope(userScope); err != nil {
		return err
	}

	env, err := newSystemdEnv(userScope, configPath)
	if err != nil {
		return err
	}

	if config.Username == "" || config.Password == "" {
		return fmt.Errorf("安装服务需要账号密码，请使用 -username 和 -password，或先用 -config 提供配置文件")
	}

	if config.CheckInterval < 10 {
		return fmt.Errorf("检查间隔过短: %ds (最小 10s)", config.CheckInterval)
	}

	if config.RetryInterval < 1 || config.MaxRetries < 1 {
		return fmt.Errorf("重试间隔和最大尝试次数必须大于 0 (当前: %ds / %d 次)",
			config.RetryInterval, config.MaxRetries)
	}

	if config.ProbeEnabled {
		if config.ProbeTimeout < 1 {
			return fmt.Errorf("探测超时必须大于 0 (当前: %ds)", config.ProbeTimeout)
		}
		if config.ProbeFailThreshold < 1 {
			return fmt.Errorf("探测失败阈值必须大于 0 (当前: %d)", config.ProbeFailThreshold)
		}
		for _, target := range probeURLs(config) {
			if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
				return fmt.Errorf("网站池地址必须以 http:// 或 https:// 开头: %s", target)
			}
		}
	}

	if config.AccountCheckInterval < 0 {
		return fmt.Errorf("账号核对间隔不能为负数 (当前: %ds)", config.AccountCheckInterval)
	}

	if attempts := effectiveAttempts(config); attempts < config.MaxRetries {
		fmt.Printf("提示: max-retries=%d 配合 %ds 的重试间隔会让单次运行超过 %ds，实际只会尝试 %d 次\n",
			config.MaxRetries, config.RetryInterval, maxRunSeconds, attempts)
	}

	if worst := config.RetryInterval*effectiveAttempts(config) + runProbeBudget(config); worst >= config.CheckInterval {
		fmt.Printf("提示: 单次运行最长约 %ds，超过了 %ds 的检查间隔，下一次检查会顺延\n",
			worst, config.CheckInterval)
	}

	// The service is started by the timer, the action is fixed in the unit file
	serviceConfig := *config
	serviceConfig.Action = "login"

	// A relative log path would land in the service working directory, so pin
	// it to a location that exists for the chosen scope
	if !serviceConfig.NoLog && !filepath.IsAbs(serviceConfig.LogFile) {
		serviceConfig.LogFile = env.LogPath
	}

	// The state file is written by every run, so it has to be pinned the same
	// way: the working directory of the unit is not a place to keep it
	if !filepath.IsAbs(serviceConfig.StateFile) {
		serviceConfig.StateFile = env.StatePath
	}

	if err := SaveConfig(&serviceConfig, env.ConfigPath); err != nil {
		return err
	}
	fmt.Printf("配置文件: %s\n", env.ConfigPath)

	if err := os.MkdirAll(env.UnitDir, 0755); err != nil {
		return fmt.Errorf("failed to create unit directory: %v", err)
	}

	if err := os.WriteFile(env.servicePath(), []byte(env.renderService(&serviceConfig)), 0644); err != nil {
		return fmt.Errorf("failed to write service unit: %v", err)
	}
	fmt.Printf("服务单元: %s\n", env.servicePath())

	if err := os.WriteFile(env.timerPath(), []byte(env.renderTimer(&serviceConfig)), 0644); err != nil {
		return fmt.Errorf("failed to write timer unit: %v", err)
	}
	fmt.Printf("定时器单元: %s\n", env.timerPath())

	if err := systemctl(userScope, "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload failed: %v", err)
	}

	if err := systemctl(userScope, "enable", "--now", timerUnit); err != nil {
		return fmt.Errorf("systemctl enable --now %s failed: %v", timerUnit, err)
	}

	fmt.Println("")
	fmt.Printf("安装完成: 每 %d 秒检查一次，单次最多尝试 %d 次，重试间隔 %d 秒\n",
		serviceConfig.CheckInterval, serviceConfig.MaxRetries, serviceConfig.RetryInterval)

	if serviceConfig.ProbeEnabled {
		pool := probeURLs(&serviceConfig)
		fmt.Printf("网站池探测: 已开启，共 %d 个站点轮询，连续 %d 个无响应才去查门户\n",
			len(pool), serviceConfig.ProbeFailThreshold)
		for i, target := range pool {
			fmt.Printf("  %d. %s\n", i+1, target)
		}
		fmt.Printf("状态文件: %s\n", serviceConfig.StateFile)
	} else {
		fmt.Println("网站池探测: 已关闭 (每次检查都会直接访问校园网门户)")
	}

	if serviceConfig.CheckAccount {
		fmt.Println("账号不匹配检测: 已开启 (发现其他账号在线会先登出再登录)")
		if serviceConfig.ProbeEnabled {
			if serviceConfig.AccountCheckInterval > 0 {
				fmt.Printf("  网络正常时每 %d 秒查一次门户核对账号\n", serviceConfig.AccountCheckInterval)
			} else {
				fmt.Println("  网络正常时不核对账号 (account_check_interval = 0)")
			}
		}
	} else {
		fmt.Println("账号不匹配检测: 已关闭")
	}

	scopeFlag := ""
	if userScope {
		scopeFlag = "--user "
		fmt.Println("")
		fmt.Println("提示: 用户级服务只在登录会话存在时运行，若需开机即生效请执行:")
		fmt.Printf("  sudo loginctl enable-linger %s\n", os.Getenv("USER"))
	} else if strings.HasPrefix(env.BinaryPath, "/home/") {
		fmt.Println("")
		fmt.Printf("提示: 可执行文件位于 %s，请勿移动或删除，建议复制到 /usr/local/bin 后重新安装\n", env.BinaryPath)
	}

	fmt.Println("")
	fmt.Println("常用命令:")
	fmt.Printf("  systemctl %sstatus %s\n", scopeFlag, timerUnit)
	fmt.Printf("  systemctl %sstart %s        # 立即执行一次\n", scopeFlag, serviceUnit)
	fmt.Printf("  journalctl %s-u %s -f\n", scopeFlag, serviceUnit)

	return nil
}

// uninstallService disables the timer and removes the units, the config file is kept
func uninstallService(userScope bool, configPath string) error {
	if err := requireSystemctl(); err != nil {
		return err
	}
	if err := requireWritableScope(userScope); err != nil {
		return err
	}

	env, err := newSystemdEnv(userScope, configPath)
	if err != nil {
		return err
	}

	// The units may already be gone or never have been enabled, so failures here
	// are reported but do not stop the cleanup
	if err := systemctl(userScope, "disable", "--now", timerUnit); err != nil {
		fmt.Printf("提示: 停用 %s 失败 (可能未安装): %v\n", timerUnit, err)
	}

	for _, path := range []string{env.timerPath(), env.servicePath()} {
		if err := os.Remove(path); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove %s: %v", path, err)
			}
			continue
		}
		fmt.Printf("已删除: %s\n", path)
	}

	if err := systemctl(userScope, "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload failed: %v", err)
	}
	_ = systemctl(userScope, "reset-failed")

	fmt.Println("卸载完成")
	if _, err := os.Stat(env.ConfigPath); err == nil {
		fmt.Printf("配置文件已保留: %s\n", env.ConfigPath)
	}

	return nil
}

// serviceStatus shows the timer state and the last runs
func serviceStatus(userScope bool) error {
	if err := requireSystemctl(); err != nil {
		return err
	}

	_ = systemctl(userScope, "status", timerUnit, "--no-pager")
	fmt.Println("")
	_ = systemctl(userScope, "status", serviceUnit, "--no-pager")
	fmt.Println("")
	return systemctl(userScope, "list-timers", timerUnit, "--no-pager")
}
