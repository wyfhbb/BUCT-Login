#!/usr/bin/env bash
# 测试脚本的共用部分，由 e2e-test.sh / disconnect-test.sh source。
#
# 三条约定：
#   1. 凭据只从环境变量读，绝不写进仓库
#   2. 临时配置写在 mktemp 目录里（权限 0600），退出时连目录一起删掉
#   3. 无论脚本怎么结束——正常、报错、被信号打断——退出前都会尝试把账号登回去。
#      校园网门户在内网，掉线状态下依然可达，所以恢复不依赖公网。

set -uo pipefail   # 故意不用 -e：有几步预期就是非零退出

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d)"
BIN="$WORK/buct-login"
CONF="$WORK/config.json"
CONF_WRONG="$WORK/config-wrong.json"
STATE="$WORK/state.json"
BLACKHOLE_ADDR="127.0.0.1:18080"

banner() {
    echo
    echo "════════════════════════════════════════════════════════════"
    echo "  $*"
    echo "════════════════════════════════════════════════════════════"
}

step() { echo; echo "── $* ──"; }

show_state() {
    echo "   state.json → $(tr -d '\n ' < "$STATE" 2>/dev/null || echo '(不存在)')"
}

require_credentials() {
    : "${BUCT_USERNAME:?请先设置 BUCT_USERNAME 环境变量（学号）}"
    : "${BUCT_PASSWORD:?请先设置 BUCT_PASSWORD 环境变量（密码）}"
}

build_binary() {
    CGO_ENABLED=0 go build -o "$BIN" "$REPO_ROOT" || {
        echo "编译失败" >&2
        exit 1
    }
}

# write_configs 生成两份配置：正确账号，以及一份账号被换掉、密码不变的
write_configs() {
    umask 077

    cat > "$CONF" <<EOF
{
  "username": "$BUCT_USERNAME",
  "password": "$BUCT_PASSWORD",
  "action": "login",
  "retry_interval": 3,
  "max_retries": 2,
  "check_account": true,
  "probe_timeout": 5,
  "probe_fail_threshold": 3,
  "account_check_interval": 1800,
  "no_log": true
}
EOF

    sed 's/"username": ".*"/"username": "0000000"/' "$CONF" > "$CONF_WRONG"
}

# reset_state 清空状态文件，于是 last_account_check 归零、慢通道立刻到期。
# 这正是「没有上次核对时间就现场创建一个」的那条分支。
reset_state() { rm -f "$STATE" "$STATE.lock"; }

start_blackhole() {
    CGO_ENABLED=0 go build -o "$WORK/blackhole" "$REPO_ROOT/scripts/blackhole" || return 1
    "$WORK/blackhole" -addr "$BLACKHOLE_ADDR" &
    BLACKHOLE_PID=$!
    sleep 1
}

stop_blackhole() {
    [ -n "${BLACKHOLE_PID:-}" ] && kill "$BLACKHOLE_PID" 2>/dev/null
    BLACKHOLE_PID=""
}

restored=0
restore_and_cleanup() {
    [ "$restored" = 1 ] && return
    restored=1

    stop_blackhole

    banner "最终恢复：确保账号处于登录状态"
    for i in 1 2 3; do
        echo "   恢复尝试 $i/3"
        "$BIN" -action=login -config="$CONF" -statefile="$STATE" -probe=false -nolog && break
        sleep 3
    done

    echo
    "$BIN" -action=info -nolog

    rm -rf "$WORK"
    echo
    echo "   临时目录已删除（含明文密码的配置随之清除）"
}

# 收到信号必须恢复后立刻退出：只恢复不退出的话，脚本会从被打断的位置继续往下跑，
# 而此时临时配置已经被删掉了
on_signal() { restore_and_cleanup; exit 130; }

install_traps() {
    trap restore_and_cleanup EXIT
    trap on_signal INT TERM PIPE HUP
}

confirm_destructive() {
    [ "${YES:-}" = "1" ] && return 0

    echo "⚠️  这个脚本会真的把校园网账号登出，期间会断网数秒。"
    echo "   脚本结束前会自动登录回去，但请不要在传大文件的时候跑。"
    read -r -p "   确认继续？输入 yes: " answer
    [ "$answer" = "yes" ] || { echo "已取消"; exit 1; }
}
