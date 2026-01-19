#!/bin/bash
# 综合压测脚本
# 包含多个场景的压测测试

HOST="${HOST:-http://localhost:8080}"
RESULTS_DIR="${RESULTS_DIR:-./load-test-results}"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# 创建结果目录
mkdir -p "$RESULTS_DIR"

echo "========================================="
echo "CookRAG-Go API 综合压测"
echo "========================================="
echo "目标: $HOST"
echo "结果目录: $RESULTS_DIR"
echo "========================================="

# 检查压测工具
if ! command -v hey &> /dev/null && ! command -v wrk &> /dev/null && ! command -v ab &> /dev/null; then
    echo "错误: 未找到压测工具"
    echo "请安装: go install github.com/rakyll/hey@latest"
    exit 1
fi

# 使用优先级: hey > wrk > ab
use_hey() {
    hey "$@" 2>&1 | tee "$RESULTS_DIR/hey_${TIMESTAMP}.txt"
}

use_wrk() {
    wrk "$@" 2>&1 | tee "$RESULTS_DIR/wrk_${TIMESTAMP}.txt"
}

use_ab() {
    ab "$@" 2>&1 | tee "$RESULTS_DIR/ab_${TIMESTAMP}.txt"
}

# 场景1: 健康检查 - 低并发
echo ""
echo "场景1: 健康检查 - 低并发 (10并发, 30秒)"
echo "-----------------------------------------"
if command -v hey &> /dev/null; then
    use_hey -n 1000 -c 10 -m GET "$HOST/api/v1/health"
elif command -v wrk &> /dev/null; then
    use_wrk -t10 -c10 -d30s "$HOST/api/v1/health"
else
    use_ab -n 1000 -c 10 "$HOST/api/v1/health"
fi

# 场景2: 健康检查 - 中等并发
echo ""
echo "场景2: 健康检查 - 中等并发 (50并发, 30秒)"
echo "-----------------------------------------"
if command -v hey &> /dev/null; then
    use_hey -n 5000 -c 50 -m GET "$HOST/api/v1/health"
elif command -v wrk &> /dev/null; then
    use_wrk -t50 -c50 -d30s "$HOST/api/v1/health"
else
    use_ab -n 5000 -c 50 "$HOST/api/v1/health"
fi

# 场景3: 健康检查 - 高并发
echo ""
echo "场景3: 健康检查 - 高并发 (100并发, 30秒)"
echo "-----------------------------------------"
if command -v hey &> /dev/null; then
    use_hey -n 10000 -c 100 -m GET "$HOST/api/v1/health"
elif command -v wrk &> /dev/null; then
    use_wrk -t100 -c100 -d30s "$HOST/api/v1/health"
else
    use_ab -n 10000 -c 100 "$HOST/api/v1/health"
fi

# 场景4: 查询接口 - 低并发
echo ""
echo "场景4: 查询接口 - 低并发 (10并发, 100请求)"
echo "-----------------------------------------"
REQUEST_FILE=$(mktemp)
echo '{"query":"什么是红烧肉？"}' > "$REQUEST_FILE"
trap "rm -f $REQUEST_FILE" EXIT

if command -v hey &> /dev/null; then
    use_hey -n 100 -c 10 -m POST -H "Content-Type: application/json" -D "$REQUEST_FILE" "$HOST/api/v1/query"
elif command -v wrk &> /dev/null; then
    LUA_SCRIPT=$(mktemp)
    cat > "$LUA_SCRIPT" << 'EOF'
wrk.method = "POST"
wrk.body   = '{"query":"什么是红烧肉？"}'
wrk.headers["Content-Type"] = "application/json"
EOF
    trap "rm -f $REQUEST_FILE $LUA_SCRIPT" EXIT
    use_wrk -t10 -c10 -d30s -s "$LUA_SCRIPT" "$HOST/api/v1/query"
else
    use_ab -n 100 -c 10 -p "$REQUEST_FILE" -T "application/json" "$HOST/api/v1/query"
fi

# 场景5: 指标接口 - 中等并发
echo ""
echo "场景5: 指标接口 - 中等并发 (20并发, 30秒)"
echo "-----------------------------------------"
if command -v hey &> /dev/null; then
    use_hey -n 2000 -c 20 -m GET "$HOST/api/v1/metrics"
elif command -v wrk &> /dev/null; then
    use_wrk -t20 -c20 -d30s "$HOST/api/v1/metrics"
else
    use_ab -n 2000 -c 20 "$HOST/api/v1/metrics"
fi

echo ""
echo "========================================="
echo "压测完成！"
echo "结果保存在: $RESULTS_DIR"
echo "========================================="
