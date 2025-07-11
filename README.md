# BUCT-Login

## 项目简介

BUCT-Login 是一款专为北京化工大学校园网设计的命令行认证工具。它支持一键登录、账户信息查询、注销和自动掉线重连等功能；其特点是适合部署在服务器上，定时执行登录操作，保持网络连接；也适合部署在一些低功耗的边缘设备上，监控校园网状态，实现校园网的自动化管理和监控。

本工具旨在帮助用户简化校园网的登录流程，提高网络连接的稳定性和使用体验。

## 功能特性

- **一键登录校园网**
  支持通过命令行快速登录北京化工大学校园网，自动检测本地 IP 并完成认证流程。
- **自动监控与掉线重连**
  提供监控模式，定时检测校园网连接状态，若检测到掉线会自动尝试重新登录，保障网络持续在线。
- **账户信息查询**
  可实时查询当前账户的登录状态、在线 IP、MAC 地址、已用流量、在线时长、账户余额、本月已用金额等详细信息。
- **命令行参数灵活配置**
  支持多种命令行参数，用户可根据实际需求自定义操作类型、账号密码、日志路径、输出模式等。
- **详细日志记录**
  所有操作均支持日志记录，日志内容包括操作时间、类型、结果和错误信息。可自定义日志文件路径，或选择仅在控制台输出，支持静默模式减少日志输出。
- **一键注销**
  支持一键安全注销校园网，释放当前网络会话。
- **跨平台支持**
  基于 Go 语言开发，支持 Windows、Linux、macOS 等主流操作系统，适合服务器、树莓派等多种设备部署。
- **易于集成与自动化**
  可结合定时任务（如 Windows 任务计划、Linux crontab）实现自动化登录和监控，提升网络使用体验。

## 安装与环境要求

### 系统要求

- **操作系统**：Windows、Linux、macOS 等支持 Go 语言的操作系统
- **Go 版本**：1.23.2 或更高版本
- **网络环境**：需要连接到北京化工大学校园网或相关网络环境

### 依赖包

项目使用 Go Modules 管理依赖，主要依赖包包括：

- `github.com/sirupsen/logrus` - 日志记录库
- `golang.org/x/sys` - 系统调用接口
- `golang.org/x/crypto` - 加密算法库
- `golang.org/x/term` - 终端接口

### 安装步骤

#### 1. 安装 Go 环境

访问 [Go 官网](https://golang.org/dl/) 下载并安装适合你操作系统的 Go 版本。

验证安装：

```bash
go version
```

#### 2. 克隆项目

```bash
git clone https://github.com/your-username/BUCT-Login.git
cd BUCT-Login
```

#### 3. 下载依赖

```bash
go mod tidy
```

#### 4. 编译项目

```bash
# Windows
go build -o buct-login.exe main.go

# Linux/macOS
go build -o buct-login main.go
```

#### 5. 验证安装

```bash
# Windows
.\buct-login.exe

# Linux/macOS
./buct-login
```

如果看到帮助信息，说明安装成功。

### 环境配置

#### 网络配置

- 确保设备已连接到北京化工大学校园网
- 项目默认使用校园网认证服务器：`202.4.130.95`

#### 权限要求

- 程序需要网络访问权限
- 日志文件写入权限（如果使用日志功能）

### 快速开始

安装完成后，可以尝试以下命令：

```bash
# 查看帮助信息
./buct-login

# 查询当前登录状态（无需账号密码）
./buct-login -action=info

# 登录校园网（需要替换为你的学号和密码）
./buct-login -action=login -username=你的学号 -password=你的密码
```

### 注意事项

- 首次运行前请确保已连接到校园网
- 使用前请确保遵守学校的网络使用规定
- 建议在测试环境中先验证功能正常后再部署到生产环境

## 使用方法

### 命令行参数

| 参数 | 说明 | 是否必需 | 示例 |
|------|------|----------|------|
| `-action` | 操作类型：login/info/logout/monitor | 必需 | `-action=login` |
| `-username` | 用户名（学号） | login/monitor 必需 | `-username=2021000000` |
| `-password` | 密码 | login/monitor 必需 | `-password=yourpassword` |
| `-logfile` | 日志文件路径 | 可选 | `-logfile=./logs/network.log` |
| `-nolog` | 不生成日志文件，仅控制台输出 | 可选 | `-nolog` |
| `-quiet` | 静默模式，仅输出错误日志 | 可选 | `-quiet` |

### 基本操作

#### 1. 登录校园网

```bash
# 基本登录
./buct-login -action=login -username=你的学号 -password=你的密码

# 登录并指定日志文件
./buct-login -action=login -username=你的学号 -password=你的密码 -logfile=./logs/login.log

# 登录但不生成日志文件
./buct-login -action=login -username=你的学号 -password=你的密码 -nolog
```

#### 2. 查询账户信息

```bash
# 查询当前登录状态和账户信息
./buct-login -action=info

# 查询信息并保存到指定日志文件
./buct-login -action=info -logfile=./logs/info.log
```

查询结果包括：

- 当前账户名
- 登录时间
- 在线 IP 地址
- 当前 MAC 地址
- 登入后使用流量
- 已使用流量
- 已使用时长
- 用户余额
- 本月已使用金额

#### 3. 注销校园网

```bash
# 注销当前登录
./buct-login -action=logout

# 注销并记录日志
./buct-login -action=logout -logfile=./logs/logout.log
```

#### 4. 自动监控模式

```bash
# 基本监控（每分钟检查一次，掉线自动重连）
./buct-login -action=monitor -username=你的学号 -password=你的密码

# 静默监控（减少日志输出）
./buct-login -action=monitor -username=你的学号 -password=你的密码 -quiet

# 监控并指定日志文件
./buct-login -action=monitor -username=你的学号 -password=你的密码 -logfile=./logs/monitor.log
```

### 高级用法

#### 1. 日志管理

```bash
# 创建日志目录
mkdir -p ./logs

# 使用自定义日志文件
./buct-login -action=login -username=你的学号 -password=你的密码 -logfile=./logs/$(date +%Y%m%d).log
```

#### 2. 自动化部署

**Windows 任务计划：**

```batch
# 创建定时任务脚本
@echo off
cd /d "C:\path\to\BUCT-Login"
buct-login.exe -action=monitor -username=你的学号 -password=你的密码 -quiet
```

**Linux crontab：**

```bash
# 编辑 crontab
crontab -e

# 添加定时任务（每小时检查一次）
0 * * * * cd /path/to/BUCT-Login && ./buct-login -action=monitor -username=你的学号 -password=你的密码 -quiet
```

#### 3. 错误排查

```bash
# 查看详细日志
./buct-login -action=login -username=你的学号 -password=你的密码 -logfile=./logs/debug.log

# 测试网络连接（查询信息）
./buct-login -action=info -logfile=./logs/test.log
```

### 使用示例

#### 场景一：日常使用

```bash
# 1. 登录校园网
./buct-login -action=login -username=2021000000 -password=mypassword

# 2. 查询使用情况
./buct-login -action=info

# 3. 注销（如需要）
./buct-login -action=logout
```

#### 场景二：服务器部署

```bash
# 后台运行监控模式
nohup ./buct-login -action=monitor -username=2021000000 -password=mypassword -quiet > /dev/null 2>&1 &
```

#### 场景三：调试模式

```bash
# 启用详细日志
./buct-login -action=login -username=2021000000 -password=mypassword -logfile=./logs/debug.log
```

### 注意事项

1. **安全性**：密码会以明文形式出现在命令行中，建议在安全环境下使用
2. **网络环境**：确保设备已连接到校园网或相关网络
3. **权限要求**：程序需要网络访问和文件写入权限
4. **日志文件**：默认日志文件为 `buct-login.log`，会持续追加内容
5. **监控模式**：监控模式会持续运行，使用 Ctrl+C 可终止程序


## 常见问题

**Q: 登录失败怎么办？**
A: 检查用户名密码是否正确，确认网络连接正常，查看日志文件获取详细错误信息。

**Q: 如何查看程序运行状态？**
A: 使用 `-action=info` 命令查询当前登录状态，或查看日志文件。

**Q: 监控模式如何停止？**
A: 在运行监控模式的终端中按 Ctrl+C 即可停止程序。

**Q: 日志文件太大怎么办？**
A: 可以定期清理日志文件，或使用 `-nolog` 参数不生成日志文件。
