# BUCT 校园网登录工具

一个用于自动登录校园网的命令行工具，支持自动重连和状态监控。

## 功能特性

- 🔐 自动登录校园网
- 📊 查看用户信息和流量统计
- 🔄 自动监控和重连功能
- 📝 详细的日志记录
- ⚙️ 支持配置文件和命令行参数

## 自行编译

```bash
git clone https://github.com/wyfhbb/BUCT-Login.git
cd buct-login
go build -o buct-login main.go
```

## 基本用法

### 登录

```bash
./buct-login -action=login -username=你的学号 -password=你的密码
```

### 查看信息

```bash
./buct-login -action=info
```

### 登出

```bash
./buct-login -action=logout
```

### 监控模式（自动重连）

```bash
./buct-login -action=monitor -username=你的学号 -password=你的密码
```

## 配置文件

生成配置文件模板：

```bash
./buct-login -generate-config=config.json
```

使用配置文件：

```bash
./buct-login -config=config.json
```

配置文件示例：

```json
{
  "username": "your_student_id",
  "password": "your_password",
  "action": "monitor",
  "monitor_interval": 60,
  "retry_interval": 5,
  "log_file": "buct-login.log",
  "quiet": false,
  "no_log": false
}
```

## 命令行参数


| 参数                | 说明                                       | 默认值         |
| ------------------- | ------------------------------------------ | -------------- |
| `-action`           | 操作类型:`login`/`info`/`logout`/`monitor` | -              |
| `-username`         | 用户名                                     | -              |
| `-password`         | 密码                                       | -              |
| `-config`           | 配置文件路径                               | -              |
| `-monitor-interval` | 监控间隔(秒)                               | 60             |
| `-retry-interval`   | 重试间隔(秒)                               | 5              |
| `-logfile`          | 日志文件路径                               | buct-login.log |
| `-nolog`            | 不保存日志文件                             | false          |
| `-quiet`            | 静默模式                                   | false          |

## 示例

后台运行监控模式：

```bash
nohup ./buct-login -action=monitor -username=yourID -password=yourpass > /dev/null 2>&1 &
```

使用自定义监控间隔：

```bash
./buct-login -action=monitor -username=yourID -password=yourpass -monitor-interval=30
```

## 注意事项

- 首次使用请确保网络环境正确
- 监控模式下程序会持续运行，建议在后台执行
- 日志文件会记录详细的操作信息，便于问题排查
- 注意：该工具仅供学习和研究使用，请勿用于非法用途。使用前请确保遵守学校的网络使用规定。

## License

MIT License
