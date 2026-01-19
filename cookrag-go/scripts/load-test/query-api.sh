#!/bin/bash
# 查询接口压测脚本
# 测试 POST /api/v1/query 接口

HOST="${HOST:-http://localhost:8080}"
CONCURRENCY="${CONCURRENCY:-10}"
DURATION="${DURATION:-30s}"
REQUESTS="${REQUESTS:-1000}"

# 查询请求模板
QUERY_TEMPLATE='{"query":"什么是红烧肉？"}'

echo "========================================="
echo "查询接口压测"
echo "========================================="
echo "目标: $HOST/api/v1/query"
echo "并发数: $CONCURRENCY"
echo "持续时间: $DURATION"
echo "请求数: $REQUESTS"
echo "========================================="

# 创建临时文件存储请求体
REQUEST_FILE=$(mktemp)
echo "$QUERY_TEMPLATE" > "$REQUEST_FILE"
trap "rm -f $REQUEST_FILE" EXIT

# 检查是否安装了 hey
if command -v hey &> /dev/null; then
    hey -n "$REQUESTS" -c "$CONCURRENCY" -m POST -H "Content-Type: application/json" -D "$REQUEST_FILE" "$HOST/api/v1/query"
# 检查是否安装了 wrk (使用 lua 脚本)
elif command -v wrk &> /dev/null; then
    # 创建 wrk lua 脚本
    LUA_SCRIPT=$(mktemp)
    cat > "$LUA_SCRIPT" << 'EOF'
wrk.method = "POST"
wrk.body   = '{"query":"什么是红烧肉？"}'
wrk.headers["Content-Type"] = "application/json"
EOF
    trap "rm -f $REQUEST_FILE $LUA_SCRIPT" EXIT
    wrk -t"$CONCURRENCY" -c"$CONCURRENCY" -d"$DURATION" -s "$LUA_SCRIPT" "$HOST/api/v1/query"
# 检查是否安装了 ab
elif command -v ab &> /dev/null; then
    ab -n "$REQUESTS" -c "$CONCURRENCY" -p "$REQUEST_FILE" -T "application/json" "$HOST/api/v1/query"
else
    echo "错误: 未找到压测工具"
    echo "请安装以下工具之一:"
    echo "  hey:   go install github.com/rakyll/hey@latest"
    echo "  wrk:   brew install wrk"
    echo "  ab:    brew install httpd"
    exit 1
fi
