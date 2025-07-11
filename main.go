package main

import (
	"buct-login/utils"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

func initLogger(logFile string, quiet bool) error {
	// 设置日志格式
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// 设置日志级别
	if quiet {
		logrus.SetLevel(logrus.ErrorLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	// 配置日志输出
	if logFile != "" {
		// 打开或创建日志文件，追加模式
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("无法创建日志文件 %s: %v", logFile, err)
		}

		// 同时输出到文件和控制台
		multiWriter := io.MultiWriter(os.Stdout, file)
		logrus.SetOutput(multiWriter)

		// logrus.Info("日志将保存到: ", logFile)
		fmt.Println("日志将保存到: ", logFile)
	} else {
		// 只输出到控制台
		logrus.SetOutput(os.Stdout)
	}

	return nil
}

func handleByte(bytes float64) string {
	if bytes >= 1024*1024*1024 {
		return fmt.Sprintf("%.2fGB", bytes/(1024*1024*1024))
	} else if bytes >= 1024*1024 {
		return fmt.Sprintf("%.2fMB", bytes/(1024*1024))
	} else if bytes >= 1024 {
		return fmt.Sprintf("%.2fKB", bytes/1024)
	}
	return fmt.Sprintf("%.0fb", bytes)
}

func handleLogin(username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("用户名和密码不能为空")
	}

	ip, loginStatus, err := utils.GetStatus()
	if err != nil {
		logrus.Error("获取状态失败: ", err)
		return err
	}

	if ip == "" {
		logrus.Error("无法获取IP地址")
		return fmt.Errorf("无法获取IP地址")
	}

	logrus.Info("本地IP: ", ip)

	if loginStatus {
		logrus.Info("已经登录")
		return nil
	}

	logrus.Info("开始登录...")
	res, err := utils.Login(username, password, "20", ip)
	if err != nil {
		logrus.Error("登录请求失败: ", err)
		return err
	}

	if resStatus, ok := res["res"]; ok && resStatus == "ok" {
		logrus.Info("登录成功")
	} else if ecode, ok := res["ecode"]; ok && ecode == "E2901" {
		logrus.Error("登录失败，请检查账户或密码")
		return fmt.Errorf("登录失败，请检查账户或密码")
	} else if errorMsg, ok := res["error"]; ok {
		logrus.Error("登录错误: ", errorMsg)
		return fmt.Errorf("登录错误: %v", errorMsg)
	} else {
		logrus.Error("未知错误: ", res)
		return fmt.Errorf("未知错误: %v", res)
	}

	return nil
}

func handleLookup() (string, error) {
	info, err := utils.GetUserInfo()
	if err != nil {
		logrus.Error("获取用户信息失败: ", err)
		return "error", err
	}

	if errorMsg, exists := info["error"]; exists {
		if errorMsg == "not_online_error" {
			logrus.Info("未登录")
			return "not_login", nil
		} else if errorMsg != "ok" {
			logrus.Error("错误: ", errorMsg)
			return "error", nil
		}
	}

	// 显示用户信息
	if userName, ok := info["user_name"]; ok {
		logrus.Info("当前账户: ", userName)
	}

	if addTime, ok := info["add_time"].(float64); ok {
		loginTime := time.Unix(int64(addTime), 0)
		fmt.Printf("登录时间: %s\n", loginTime.Format("2006-01-02 15:04:05"))
	}

	if allBytes, ok := info["all_bytes"].(float64); ok {
		fmt.Printf("登入后使用流量: %s\n", handleByte(allBytes))
	}

	if onlineIP, ok := info["online_ip"]; ok {
		fmt.Printf("在线IP: %s\n", onlineIP)
	}

	if userMac, ok := info["user_mac"]; ok {
		fmt.Printf("当前MAC: %s\n", userMac)
	}

	if sumBytes, ok := info["sum_bytes"].(float64); ok {
		fmt.Printf("已使用流量: %s\n", handleByte(sumBytes))
	}

	if sumSeconds, ok := info["sum_seconds"].(float64); ok {
		hours := int(sumSeconds) / 3600
		minutes := (int(sumSeconds) % 3600) / 60
		seconds := int(sumSeconds) % 60
		fmt.Printf("已使用时长: %02d:%02d:%02d\n", hours, minutes, seconds)
	}

	if userBalance, ok := info["user_balance"].(float64); ok {
		fmt.Printf("用户余额: %.2f元\n", userBalance)
	}

	if userCharge, ok := info["user_charge"].(float64); ok {
		fmt.Printf("本月已使用金额: %.2f元\n", userCharge)
	}

	return "already_login", nil
}

func handleLogout() error {
	ip, loginStatus, err := utils.GetStatus()
	if err != nil {
		logrus.Error("获取状态失败: ", err)
		return err
	}

	if ip == "" {
		logrus.Error("无法获取IP地址")
		return fmt.Errorf("无法获取IP地址")
	}

	if !loginStatus {
		logrus.Info("尚未登录")
		return nil
	}

	logrus.Info("本地IP: ", ip)
	logrus.Info("开始登出...")

	ret, err := utils.Logout(ip, "20")
	if err != nil {
		logrus.Error("登出请求失败: ", err)
		return err
	}

	if errorMsg, ok := ret["error"]; ok {
		if errorMsg == "ok" {
			logrus.Info("登出成功")
		} else {
			logrus.Error("登出错误: ", errorMsg)
			return fmt.Errorf("登出错误: %v", errorMsg)
		}
	} else {
		logrus.Error("未知响应: ", ret)
		return fmt.Errorf("未知响应: %v", ret)
	}

	return nil
}

// 监听模式：检查登录状态，如果未登录则自动登录
func handleMonitor(username, password string) error {
	for {
		status, err := handleLookup()
		if err != nil {
			logrus.Error("检查状态失败: ", err)
			time.Sleep(30 * time.Second)
			continue
		}

		if status == "not_login" {
			logrus.Info("检测到未登录状态，尝试自动登录...")
			err := handleLogin(username, password)
			if err != nil {
				logrus.Error("自动登录失败: ", err)
			}
		} else {
			logrus.Info("当前已登录")
		}

		// 每分钟检查一次
		time.Sleep(60 * time.Second)
	}
}

func showUsage() {
	fmt.Println("校园网登录工具")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  buct-login -action=login -username=学号 -password=密码")
	fmt.Println("  buct-login -action=info")
	fmt.Println("  buct-login -action=logout")
	fmt.Println("  buct-login -action=monitor -username=学号 -password=密码")
	fmt.Println("")
	fmt.Println("参数:")
	fmt.Println("  -action     操作类型: login|info|logout|monitor")
	fmt.Println("  -username   用户名 (login和monitor模式必需)")
	fmt.Println("  -password   密码 (login和monitor模式必需)")
	fmt.Println("  -logfile    日志文件路径 (默认: buct-login.log)")
	fmt.Println("  -nolog      不保存日志文件，只输出到控制台")
	fmt.Println("  -quiet      静默模式，减少日志输出")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  buct-login -action=login -username=yourschoolID -password=yourpassword")
	fmt.Println("  buct-login -action=monitor -username=yourschoolID -password=yourpassword -quiet")
	fmt.Println("  buct-login -action=info -logfile=./logs/network.log")
	fmt.Println("  buct-login -action=login -username=yourschoolID -password=yourpassword -nolog")
}

func main() {
	var (
		action   = flag.String("action", "", "操作类型: login|info|logout|monitor")
		username = flag.String("username", "", "用户名")
		password = flag.String("password", "", "密码")
		logFile  = flag.String("logfile", "buct-login.log", "日志文件路径")
		noLog    = flag.Bool("nolog", false, "不保存日志文件")
		quiet    = flag.Bool("quiet", false, "静默模式")
	)

	flag.Parse()

	// 初始化日志
	var logPath string
	if !*noLog {
		logPath = *logFile
	}

	err := initLogger(logPath, *quiet)
	if err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}

	if *action == "" {
		showUsage()
		os.Exit(1)
	}

	switch *action {
	case "login":
		err = handleLogin(*username, *password)
	case "info":
		_, err = handleLookup()
	case "logout":
		err = handleLogout()
	case "monitor":
		err = handleMonitor(*username, *password)
	default:
		fmt.Printf("未知操作: %s\n", *action)
		showUsage()
		os.Exit(1)
	}

	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}
