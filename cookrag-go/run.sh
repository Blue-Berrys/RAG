#!/bin/bash

# CookRAG-Go 启动脚本

set -e

echo "🚀 CookRAG-Go 启动脚本"
echo "========================"

# 检查.env文件
if [ ! -f ".env" ]; then
    echo "❌ 错误: .env 文件不存在"
    echo "请先复制 .env.example 到 .env 并填入你的配置"
    echo ""
    echo "  cp .env.example .env"
    echo "  nano .env  # 编辑填入你的API密钥"
    exit 1
fi

# 加载环境变量
echo "📝 加载环境变量..."
set -a  # 自动导出所有变量
source .env
set +a

# 检查必需的环境变量
if [ -z "$ZHIPU_API_KEY" ]; then
    echo "❌ 错误: ZHIPU_API_KEY 未设置"
    echo "请在 .env 文件中设置你的智谱API密钥"
    exit 1
fi

echo "✅ 环境变量加载完成"
echo ""

# 检查依赖服务
echo "🔍 检查依赖服务..."

# 检查Milvus
if ! nc -z localhost 19530 2>/dev/null; then
    echo "⚠️  警告: Milvus未运行 (localhost:19530)"
    echo "请启动: cd deployments/docker && docker-compose up -d"
fi

# 检查Neo4j
if ! nc -z localhost 7687 2>/dev/null; then
    echo "⚠️  警告: Neo4j未运行 (localhost:7687)"
    echo "请启动: cd deployments/docker && docker-compose up -d"
fi

# 检查Redis
if ! nc -z localhost 6379 2>/dev/null; then
    echo "⚠️  警告: Redis未运行 (localhost:6379)"
    echo "请启动: cd deployments/docker && docker-compose up -d"
fi

echo ""
echo "🎯 启动 CookRAG-Go..."
echo "========================"
echo ""

# 运行程序
go run cmd/demo/main.go
