#!/usr/bin/env bash
# 破坏性端到端测试：⚠️ 会真的把账号登出，期间断网数秒。
#
#   BUCT_USERNAME=你的学号 BUCT_PASSWORD=你的密码 ./scripts/disconnect-test.sh
#   加 YES=1 可跳过确认提示（供 CI 或无人值守使用）
#
# 覆盖：错账号检测 → 登出 → 用错账号登录失败 → 探测发现掉线 → 自动重登。
# 恢复挂在 trap 上，脚本被 Ctrl-C 或 kill 掉也会先把账号登回去。

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

require_credentials
confirm_destructive
install_traps
build_binary
write_configs
reset_state      # last_account_check 归零 → 慢通道立刻到期

banner "场景 1：起始状态"
"$BIN" -action=info -nolog | head -3

banner "场景 2：错账号检测 ⚠️ 这一步会真的断网"
step "配置里的账号换成 0000000（密码不变），状态为空所以慢通道立刻到期"
"$BIN" -action=login -config="$CONF_WRONG" -statefile="$STATE" -account-check-interval=1
echo "   exit=$?（预期非零：E2901 凭据错误不重试）"

step "确认当前确实已掉线"
"$BIN" -action=info -nolog | head -3

banner "场景 3：掉线自动恢复（核心）"
step "换回正确配置：探测应当失败或被劫持 → 查门户 → 未登录 → 登录"
"$BIN" -action=login -config="$CONF" -statefile="$STATE"
echo "   exit=$?"

step "确认已恢复在线"
"$BIN" -action=info -nolog | head -3

banner "场景 4：恢复之后再跑一次（应回到只探测的快路径）"
"$BIN" -action=login -config="$CONF" -statefile="$STATE"
echo "   exit=$?"
show_state
