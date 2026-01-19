#!/bin/bash
# 快速基准测试
# 使用 curl 进行简单的性能测试

HOST="${HOST:-http://localhost:8080}"
ITERATIONS="${ITERATIONS:-100}"

echo "========================================="
echo "快速基准测试"
echo "========================================="
echo "目标: $HOST"
echo "迭代次数: $ITERATIONS"
echo "========================================="

# 测试1: 健康检查响应时间
echo ""
echo "测试1: 健康检查端点 (/api/v1/health)"
echo "-----------------------------------------"
total_time=0
for i in $(seq 1 "$ITERATIONS"); do
    start=$(date +%s%N)
    curl -s "$HOST/api/v1/health" > /dev/null
    end=$(date +%s%N)
    elapsed=$((end - start))
    total_time=$((total_time + elapsed))
    if [ $((i % 20)) -eq 0 ]; then
        echo "完成: $i/$ITERATIONS"
    fi
done
avg_time=$((total_time / ITERATIONS / 1000000))  # 转换为毫秒
echo "平均响应时间: ${avg_time}ms"

# 测试2: 指标端点响应时间
echo ""
echo "测试2: 指标端点 (/api/v1/metrics)"
echo "-----------------------------------------"
total_time=0
for i in $(seq 1 "$ITERATIONS"); do
    start=$(date +%s%N)
    curl -s "$HOST/api/v1/metrics" > /dev/null
    end=$(date +%s%N)
    elapsed=$((end - start))
    total_time=$((total_time + elapsed))
    if [ $((i % 20)) -eq 0 ]; then
        echo "完成: $i/$ITERATIONS"
    fi
done
avg_time=$((total_time / ITERATIONS / 1000000))
echo "平均响应时间: ${avg_time}ms"

# 测试3: 查询端点响应时间
echo ""
echo "测试3: 查询端点 (/api/v1/query)"
echo "-----------------------------------------"
total_time=0
success_count=0
for i in $(seq 1 "$ITERATIONS"); do
    start=$(date +%s%N)
    response=$(curl -s -X POST -H "Content-Type: application/json" \
        -d '{"query":"什么是红烧肉？"}' \
        "$HOST/api/v1/query")
    end=$(date +%s%N)
    elapsed=$((end - start))
    total_time=$((total_time + elapsed))

    # 检查响应是否成功
    if echo "$response" | grep -q "answer\|result"; then
        success_count=$((success_count + 1))
    fi

    if [ $((i % 20)) -eq 0 ]; then
        echo "完成: $i/$ITERATIONS"
    fi
done
avg_time=$((total_time / ITERATIONS / 1000000))
success_rate=$((success_count * 100 / ITERATIONS))
echo "平均响应时间: ${avg_time}ms"
echo "成功率: ${success_rate}%"

echo ""
echo "========================================="
echo "基准测试完成"
echo "========================================="
