#!/usr/bin/env bash
# 非破坏性端到端测试：全程保持在线，不会断网。
#
#   BUCT_USERNAME=你的学号 BUCT_PASSWORD=你的密码 ./scripts/e2e-test.sh
#
# 覆盖：首次运行建立核对时间、网站池轮询、慢通道到期与未到期、防重叠锁、劫持识别。
# 断网恢复的验证在 disconnect-test.sh 里。

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

require_credentials
install_traps
build_binary
write_configs
reset_state

banner "场景 1：起始状态"
"$BIN" -action=info -nolog

banner "场景 2：全新状态首次运行（没有上次核对时间 → 应当现场创建一个）"
step "运行（state.json 不存在）"
"$BIN" -action=login -config="$CONF" -statefile="$STATE"
echo "   exit=$?"
show_state

banner "场景 3：紧接着连跑 4 次（应只探测、轮询推进、完全不碰门户）"
for i in 1 2 3 4; do
    step "第 $i 次"
    "$BIN" -action=login -config="$CONF" -statefile="$STATE"
    echo "   exit=$?"
done
show_state

banner "场景 4：把核对间隔改成 1 秒，强制慢通道到期"
step "先等 2 秒，确保与上次核对拉开时间差"
sleep 2
step "应当出现 periodic account check，并核对到账号一致"
"$BIN" -action=login -config="$CONF" -statefile="$STATE" -account-check-interval=1
echo "   exit=$?"
show_state

banner "场景 5：恢复 1800 秒间隔 → 应当跳过核对"
"$BIN" -action=login -config="$CONF" -statefile="$STATE"
echo "   exit=$?"

banner "场景 6：防重叠锁（两个进程真实重叠）"
start_blackhole || { echo "黑洞监听器启动失败，跳过本场景"; }
if [ -n "${BLACKHOLE_PID:-}" ]; then
    step "进程 A：探测一个只接受连接不回数据的地址，会卡住 12 秒"
    ("$BIN" -action=login -config="$CONF" -statefile="$STATE" \
        -probe-urls="http://$BLACKHOLE_ADDR/generate_204" -probe-attempts=1 -probe-timeout=12 \
        >/dev/null 2>&1; echo "   [A] 结束 exit=$?") &
    a_pid=$!

    sleep 2
    step "进程 B：A 还在跑，B 应当被锁挡下并以 exit=0 结束"
    "$BIN" -action=login -config="$CONF" -statefile="$STATE"
    echo "   [B] exit=$?"

    wait "$a_pid"      # 只等 A，不能用裸 wait：黑洞监听器永不退出
    stop_blackhole
fi

banner "场景 7：劫持识别"
step "用一个会返回非 204 的地址冒充检测端点，应判定 intercepted 并立刻查门户"
"$BIN" -action=login -config="$CONF" -statefile="$STATE" \
    -probe-urls=http://www.baidu.com/generate_204,http://connect.rom.miui.com/generate_204 \
    -probe-attempts=3
echo "   exit=$?"
