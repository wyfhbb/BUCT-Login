# BUCT 校园网登录工具

一个用于自动登录校园网的命令行工具，通过 systemd 定时服务实现开机自动登录与掉线重连。

## 功能特性

- 🔐 自动登录校园网
- 📊 查看用户信息和流量统计
- ⏱️ systemd 定时服务，开机自动生效，可配置检查频率与重试策略
- 👤 账号不匹配检测：发现在线账号不是自己的，自动登出后重新登录（可关闭）
- 📝 详细的日志记录
- ⚙️ 支持配置文件和命令行参数

> 旧版本的 `-action=monitor` 常驻监控模式已移除：进程重启或开机后会丢失，改由 systemd 托管更可靠。

## 自行编译

```bash
git clone https://github.com/wyfhbb/BUCT-Login.git
cd BUCT-Login
go build -o buct-login .
```

注意：项目已拆分为多个源文件，请使用 `go build .`（不要再用 `go build main.go`）。

## 基本用法

### 登录

```bash
./buct-login -action=login -username=你的学号 -password=你的密码
```

已在线时不会重复登录；若开启了账号检测且在线账号不是配置的账号，会先登出再用配置的账号登录。

### 查看信息

```bash
./buct-login -action=info
```

### 登出

```bash
./buct-login -action=logout
```

## systemd 定时服务（推荐）

用 systemd 定时器替代常驻监控进程：定时触发一次性登录检查，掉线时自动补登，开机后自动恢复。

### 安装（系统级，开机即生效）

```bash
sudo ./buct-login -action=install-service \
  -username=你的学号 -password=你的密码 \
  -check-interval=60 -retry-interval=5 -max-retries=3
```

安装过程会：

1. 把配置（含账号密码）写入 `/etc/buct-login/config.json`，权限 `0600`
2. 生成 `/etc/systemd/system/buct-login.service`（`Type=oneshot`，执行 `-action=login`）
3. 生成 `/etc/systemd/system/buct-login.timer`（`OnBootSec=30s` + `OnUnitActiveSec=检查间隔`）
4. 执行 `systemctl daemon-reload` 与 `systemctl enable --now buct-login.timer`

建议先把可执行文件放到固定位置（如 `sudo cp buct-login /usr/local/bin/`）再安装，单元文件会记录当前可执行文件的绝对路径。

### 安装（用户级，无需 root）

```bash
./buct-login -action=install-service -user \
  -username=你的学号 -password=你的密码 -check-interval=60
```

配置写入 `~/.config/buct-login/config.json`，单元写入 `~/.config/systemd/user/`。用户级服务默认只在登录会话存在时运行，如需开机即生效：

```bash
sudo loginctl enable-linger $USER
```

### 查看状态与日志

```bash
./buct-login -action=service-status          # 系统级
./buct-login -action=service-status -user    # 用户级

sudo journalctl -u buct-login.service -f     # 实时日志
systemctl start buct-login.service           # 立即执行一次检查
```

### 卸载

```bash
sudo ./buct-login -action=uninstall-service        # 系统级
./buct-login -action=uninstall-service -user       # 用户级
```

卸载会停用并删除单元文件，配置文件保留。

### 修改频率 / 重试参数

重新执行一次 `install-service` 即可覆盖安装：

```bash
sudo ./buct-login -action=install-service -check-interval=300 -max-retries=5
```

（账号密码可省略，会从已有配置文件读取。）

## 账号不匹配检测

开启后（默认开启），每次登录检查会把在线账号与配置中的 `username` 对比：

- 一致 → 不做任何操作
- 不一致 → 先登出，等待 2 秒后用配置的账号登录

匹配规则忽略大小写；若配置里的用户名不带 `@` 后缀，则在线账号的运营商后缀会被忽略（`2021001` 与 `2021001@cmcc` 视为同一账号）。

关闭该功能：

```bash
./buct-login -action=login -check-account=false
```

或在配置文件中设置 `"check_account": false`。

## 配置文件

生成配置文件模板：

```bash
./buct-login -generate-config=config.json
```

使用配置文件：

```bash
./buct-login -config=config.json
```

未指定 `-config` 时，按以下顺序自动查找：`./config.json` → `~/.config/buct-login/config.json` → `/etc/buct-login/config.json`。

配置文件示例：

```json
{
  "username": "your_student_id",
  "password": "your_password",
  "action": "login",
  "check_interval": 60,
  "retry_interval": 5,
  "max_retries": 3,
  "check_account": true,
  "log_file": "/var/log/buct-login.log",
  "quiet": false,
  "no_log": false
}
```

命令行参数优先级高于配置文件（只有显式写出的参数才会覆盖）。

## 命令行参数


| 参数                | 说明                                                                                  | 默认值         |
| ------------------- | ------------------------------------------------------------------------------------- | -------------- |
| `-action`           | 操作类型:`login`/`info`/`logout`/`install-service`/`uninstall-service`/`service-status` | -              |
| `-username`         | 用户名                                                                                  | -              |
| `-password`         | 密码                                                                                    | -              |
| `-config`           | 配置文件路径                                                                            | 自动查找       |
| `-generate-config`  | 生成配置文件模板                                                                        | -              |
| `-check-interval`   | systemd 检查间隔(秒)，最小 10                                                            | 60             |
| `-retry-interval`   | 单次运行内的重试间隔(秒)                                                                 | 5              |
| `-max-retries`      | 单次运行的最大尝试次数（含首次）                                                          | 3              |
| `-check-account`    | 账号不匹配检测                                                                          | true           |
| `-user`             | 操作用户级 systemd 而非系统级                                                            | false          |
| `-logfile`          | 日志文件路径                                                                            | buct-login.log |
| `-nolog`            | 不保存日志文件                                                                          | false          |
| `-quiet`            | 静默模式                                                                                | false          |
| `-version`          | 显示版本号                                                                              | -              |

账号密码错误（`E2901`）时不会重试，直接以非零退出码结束，便于 systemd 记录失败。

## 注意事项

- 首次使用请确保网络环境正确
- 配置文件包含明文密码，程序会以 `0600` 权限写入，请勿提交到版本库（`.gitignore` 已忽略 `config.json`）
- 安装为系统级服务后，可执行文件不要移动或删除，否则单元文件中的路径会失效
- 服务日志同时写入 journal 与配置的日志文件，排查问题优先看 `journalctl -u buct-login.service`
- 注意：该工具仅供学习和研究使用，请勿用于非法用途。使用前请确保遵守学校的网络使用规定。

## License

MIT License
