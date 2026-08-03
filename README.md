# BUCT 校园网登录工具

一个用于自动登录校园网的命令行工具，通过 systemd 定时服务（或 crontab）实现开机自动登录与掉线重连。

## 功能特性

- 🔐 自动登录校园网
- 📊 查看用户信息和流量统计
- 🌐 **网站池探测**：平时只轮询公网站点，探测不通才去访问校园网门户，避免长期高频访问门户
- ⏱️ systemd 定时服务，开机自动生效，可配置检查频率与重试策略
- 🕒 crontab 计划任务方案，适用于没有 systemd 的系统
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

## 网站池探测（默认开启）

如果每次检查都去访问校园网登录页，长期下来对门户是持续的高频请求，既可能被限流拉黑，也容易引起信息中心注意。所以默认的检查流程改成了两级：

```
每次运行
  ├─ 从网站池里取「下一个」站点探测（每次只发 1 个请求）
  │    ├─ 通 → 结束，完全不碰校园网门户
  │    └─ 不通 → 再试后面 2 个站点
  │          └─ 连续 3 个都不通 → 才去查门户，未登录就登录
  └─ 若距上次核对账号已超过 30 分钟 → 顺便查一次门户核对账号
```

网站池是**轮询**的：第一次运行访问第 1 个站点，下一次访问第 2 个，依次轮换。位置记录在状态文件里，所以每个站点被访问的间隔 = 检查间隔 × 站点数量（默认 60 秒 × 5 = 每 5 分钟才轮到一次），对任何一方都算不上负担。

正常在线时，门户请求量从「每分钟 1 次」降到「每 30 分钟 1 次」。

### 默认网站池

默认用的是各手机厂商的联网检测端点（就是手机连 WiFi 时判断"此 WiFi 需要登录"用的那个接口），全部在国内：

| 站点                                                          | 来源      |
| ------------------------------------------------------------- | --------- |
| `http://connect.rom.miui.com/generate_204`                    | 小米      |
| `http://connectivitycheck.platform.hicloud.com/generate_204`  | 华为      |
| `http://wifi.vivo.com.cn/generate_204`                        | vivo      |
| `http://www.qualcomm.cn/generate_204`                         | 高通中国  |
| `http://204.ustclug.org/`                                     | 中科大 LUG |

选它们的原因：

1. 响应体为空、只返回 `204 No Content`，流量可以忽略不计
2. 它们本来就是给设备高频轮询用的，不存在"访问太多被拉黑"的问题
3. **能区分「被门户劫持」和「网站自己挂了」**：这些端点只会返回 204，一旦返回了 302 跳转或者一个 HTML 页面，那就是校园网门户在中间拦截，可以立刻判定掉线，不用再试其它站点

### 自定义网站池

```bash
./buct-login -action=login -probe-urls=http://connect.rom.miui.com/generate_204,https://www.baidu.com
```

或在配置文件里改 `probe_urls`。

> ⚠️ **强烈建议只用 `generate_204` 这类检测端点，不要换成普通网站。**
>
> 实测发现：掉线状态下访问 `http://connectivitycheck.platform.hicloud.com/generate_204`，BUCT 门户返回的是 **`HTTP 200` 加一个页面**，而不是 302 跳转。
>
> 这意味着如果网站池里放的是百度这类普通网站，按「返回 2xx 就算通」判定的话，**掉线时探测依然会成功，程序永远不会去重登**。只有「必须返回 204」这条更严格的判定才能识破。

其它注意事项：

- 地址里含 `204` 的会按「必须返回 204（或 200 且响应体为空）」判定，其它地址只能按「返回 2xx 即算通」判定——后者在本校园网环境下识别不了掉线
- 探测不走系统代理（`HTTP_PROXY` 等环境变量会被忽略），否则测的就是代理而不是本机到公网的链路

### 关闭网站池

```bash
./buct-login -action=login -probe=false
```

关闭后恢复旧行为：每次运行都直接查门户。

### 登错账号怎么办

这是网站池方案唯一的盲区：**如果登的是别人的账号，网络是通的**，探测永远成功，光靠探测发现不了。

而账号名只能从门户拿到，所以程序用一条「慢通道」兜底：网络正常时，每隔 `account_check_interval`（默认 1800 秒 = 30 分钟）查一次门户核对在线账号，不一致就登出重登。开销是每半小时 1 个请求，最坏 30 分钟内能发现登错账号。

```bash
./buct-login -action=login -account-check-interval=600   # 改成 10 分钟核对一次
./buct-login -action=login -account-check-interval=0     # 关闭定期核对（只在断网时才查门户）
```

上次核对时间记在状态文件里。此外，任何一次「探测失败 → 查门户」也会顺带完成核对并重置计时。

### 状态文件

轮询位置和上次核对时间需要跨进程保留（每次运行都是独立的短进程），存放位置：

| 场景                   | 默认路径                                |
| ---------------------- | --------------------------------------- |
| root / 系统级服务      | `/var/lib/buct-login/state.json`        |
| 普通用户 / 用户级服务  | `~/.local/state/buct-login/state.json`  |

可用 `-statefile` 或配置项 `state_file` 指定。同目录下还会有一个 `state.json.lock`，是防止两次运行重叠的锁文件。文件丢了不影响使用：轮询从头开始，账号核对多跑一次而已。

如果跑在 **OpenWrt 等 flash 存储**的设备上，可以把 `state_file` 指到 `/tmp/buct-login-state.json`，避免每分钟一次小文件写入（掉电丢了也只是轮询重新开始）。

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

## crontab 计划任务（非 systemd 系统）

如果系统没有 systemd（OpenWrt、Alpine、部分 NAS、macOS、WSL1 等），用 crontab 定时跑一次 `-action=login` 即可，效果和 systemd 定时器一致，**不需要任何额外功能**：单次运行本来就是「探测 → 必要时登录 → 退出」，轮询位置和账号核对时间都存在状态文件里，跨进程自动衔接。

### 1. 放好可执行文件

```bash
sudo cp buct-login /usr/local/bin/
```

### 2. 生成配置文件

```bash
mkdir -p ~/.config/buct-login
buct-login -generate-config=~/.config/buct-login/config.json
chmod 600 ~/.config/buct-login/config.json
vi ~/.config/buct-login/config.json      # 填入 username / password
```

建议在配置里显式写绝对路径，cron 的工作目录和环境变量都和登录 shell 不同：

```json
{
  "username": "你的学号",
  "password": "你的密码",
  "action": "login",
  "state_file": "/home/你的用户名/.local/state/buct-login/state.json",
  "log_file": "/home/你的用户名/.local/state/buct-login/buct-login.log"
}
```

### 3. 添加 crontab 条目

```bash
crontab -e
```

```cron
# 每分钟检查一次（探测网站池，必要时才登录）
* * * * * /usr/local/bin/buct-login -action=login -config=/home/你的用户名/.config/buct-login/config.json >/dev/null 2>&1

# 开机后也跑一次
@reboot sleep 30 && /usr/local/bin/buct-login -action=login -config=/home/你的用户名/.config/buct-login/config.json >/dev/null 2>&1
```

几点说明：

- **必须写绝对路径**：cron 的 `PATH` 很短，`buct-login`、配置文件、日志文件都要用完整路径
- **不需要 `flock`**：程序自带防重叠锁（见下一节），上一次没跑完时这一次会直接跳过
- 日志已经由程序写进 `log_file`，所以命令末尾直接 `>/dev/null 2>&1` 丢弃标准输出即可，避免 cron 发一堆本地邮件
- **配置里的 `check_interval` 对 cron 无效**，那是给 systemd 定时器用的；实际检查间隔完全由 crontab 表达式决定
- 每分钟运行 = 网站池每分钟轮换一个站点，正好是设计的节奏；如果想放慢，改成 `*/2 * * * *`（每 2 分钟）即可

### 单次运行的边界（systemd 与 cron 等价的关键）

程序内部没有任何常驻循环：一次运行就是「加锁 → 读状态 → 探测 → 必要时登录 → 写状态 → 退出」，所有循环都是有界的。systemd 定时器和 crontab 只是两种触发方式，行为完全一致。

为了让两边真正等价，有两道运行时保护：

1. **防重叠锁**：启动时对 `<状态文件>.lock` 加非阻塞文件锁，拿不到就记一行日志、以退出码 0 结束。systemd 本来就不会给同一个 unit 起第二个实例，cron 会，所以这道保护做进了程序里，cron 不必再写 `flock`。锁由内核在进程结束时释放，程序被 kill 或崩溃都不会留下死锁。
2. **单次运行时长上限**：`max_retries × retry_interval` 若会让单次运行超过 300 秒，实际尝试次数会被自动收敛并打印警告。比如 `retry_interval=60, max_retries=100`（理论上要跑 100 分钟）会被压到 3 次。`install-service` 安装时也会提示。

### 4. 验证

```bash
# 手动跑一次，看看输出
/usr/local/bin/buct-login -action=login -config=/home/你的用户名/.config/buct-login/config.json

# 看日志
tail -f ~/.local/state/buct-login/buct-login.log

# 确认 cron 条目已生效
crontab -l
```

### 其它系统的小差异

- **OpenWrt**：用 `crontab -e` 后需要 `/etc/init.d/cron restart`；`flock` 在 busybox 里路径是 `/usr/bin/flock`，没有就去掉
- **macOS**：cron 仍可用，但系统推荐 launchd；用 cron 时需要在「系统设置 → 隐私与安全性 → 完全磁盘访问权限」里给 `cron` 授权
- **Alpine**：默认是 busybox crond，需要 `rc-update add crond default && rc-service crond start`

## 账号不匹配检测

开启后（默认开启），每次登录检查会把在线账号与配置中的 `username` 对比：

- 一致 → 不做任何操作
- 不一致 → 先登出，等待 2 秒后用配置的账号登录

匹配规则忽略大小写；若配置里的用户名不带 `@` 后缀，则在线账号的运营商后缀会被忽略（`2021001` 与 `2021001@cmcc` 视为同一账号）。

开启网站池探测时，这项对比发生在两个时机：探测失败去查门户时，以及每 `account_check_interval` 秒的定期核对（详见[登错账号怎么办](#登错账号怎么办)）。

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
}
```

命令行参数优先级高于配置文件（只有显式写出的参数才会覆盖）。

旧版本的配置文件可以直接继续用：缺少的 `probe_*` 等字段会自动取默认值（即默认启用网站池探测）。

## 命令行参数


| 参数                | 说明                                                                                  | 默认值         |
| ------------------- | ------------------------------------------------------------------------------------- | -------------- |
| `-action`           | 操作类型:`login`/`info`/`logout`/`install-service`/`uninstall-service`/`service-status` | -              |
| `-username`         | 用户名                                                                                  | -              |
| `-password`         | 密码                                                                                    | -              |
| `-config`           | 配置文件路径                                                                            | 自动查找       |
| `-generate-config`  | 生成配置文件模板                                                                        | -              |
| `-check-interval`   | systemd 检查间隔(秒)，最小 10（cron 方式下无效）                                           | 60             |
| `-retry-interval`   | 单次运行内的重试间隔(秒)                                                                 | 5              |
| `-max-retries`      | 单次运行的最大尝试次数（含首次），过大时会被压到 300 秒的运行时长上限内                      | 3              |
| `-check-account`    | 账号不匹配检测                                                                          | true           |
| `-user`             | 操作用户级 systemd 而非系统级                                                            | false          |
| `-logfile`          | 日志文件路径                                                                            | buct-login.log |
| `-nolog`            | 不保存日志文件                                                                          | false          |
| `-quiet`            | 静默模式                                                                                | false          |
| `-version`          | 显示版本号                                                                              | -              |

网站池探测相关：

| 参数                      | 说明                                                    | 默认值           |
| ------------------------- | ------------------------------------------------------- | ---------------- |
| `-probe`                  | 是否启用网站池探测，关闭则每次都直接查门户                  | true             |
| `-probe-urls`             | 网站池，逗号分隔                                          | 内置 5 个 204 端点 |
| `-probe-timeout`          | 单次探测超时(秒)                                          | 5                |
| `-probe-attempts`         | 连续多少个站点无响应才判定断网                             | 3                |
| `-account-check-interval` | 网络正常时核对在线账号的间隔(秒)，0 为关闭                  | 1800             |
| `-statefile`              | 状态文件路径（轮询位置 + 上次核对时间）                     | 见[状态文件](#状态文件) |

账号密码错误（`E2901`）时不会重试，直接以非零退出码结束，便于 systemd 记录失败。

## 注意事项

- 首次使用请确保网络环境正确
- 配置文件包含明文密码，程序会以 `0600` 权限写入，请勿提交到版本库（`.gitignore` 已忽略 `config.json`）
- 安装为系统级服务后，可执行文件不要移动或删除，否则单元文件中的路径会失效
- 服务日志同时写入 journal 与配置的日志文件，排查问题优先看 `journalctl -u buct-login.service`
- 不在校园网内时（如手机热点、家里宽带），探测能通而门户显示未登录，程序会跳过登录直接退出，不会反复尝试
- 如果日志里频繁出现「intercepted」，说明门户在拦截探测请求，属于正常的掉线判定；若在**已登录**状态下仍频繁出现，可能是某个探测站点被劫持，把它从 `probe_urls` 里去掉即可
- 注意：该工具仅供学习和研究使用，请勿用于非法用途。使用前请确保遵守学校的网络使用规定。

## License

MIT License
