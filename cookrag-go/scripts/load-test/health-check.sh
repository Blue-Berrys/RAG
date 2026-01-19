#!/bin/bash
# 健康检查压测脚本
# 测试 GET /api/v1/health 接口

HOST="${HOST:-http://localhost:8080}"
CONCURRENCY="${CONCURRENCY:-10}"
DURATION="${DURATION:-30s}"

echo "========================================="
echo "健康检查压测"
echo "========================================="
echo "目标: $HOST/api/v1/health"
echo "并发数: $CONCURRENCY"
echo "持续时间: $DURATION"
echo "========================================="

# 检查是否安装了 hey
if command -v hey &> /dev/null; then
    hey -n 1000 -c "$CONCURRENCY" -m GET "$HOST/api/v1/health"
# 检查是否安装了 wrk
elif command -v wrk &> /dev/null; then
    wrk -t"$CONCURRENCY" -c"$CONCURRENCY" -d"$DURATION" "$HOST/api/v1/health"
# 检查是否安装了 ab (Apache Bench)
elif command -v ab &> /dev/null; then
    ab -n 1000 -c "$CONCURRENCY" "$HOST/api/v1/health"
else
    echo "错误: 未找到压测工具"
    echo "请安装以下工具之一:"
    echo "  hey:   go install github.com/rakyll/hey@latest"
    echo "  wrk:   brew install wrk"
    echo "  ab:    brew install httpd"
    exit 1
fi
