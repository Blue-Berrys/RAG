#!/bin/bash

# CookRAG-Go 快速启动脚本

set -e

echo "🚀 CookRAG-Go 快速启动"
echo "=================="

# 检查Go版本
echo "1️⃣ 检查Go版本..."
if ! command -v go &> /dev/null; then
    echo "❌ Go未安装，请先安装Go 1.21+"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "✅ Go版本: $GO_VERSION"

# 检查Docker
echo "2️⃣ 检查Docker..."
if ! command -v docker &> /dev/null; then
    echo "❌ Docker未安装，请先安装Docker"
    exit 1
fi
echo "✅ Docker已安装"

# 检查.env文件
echo "3️⃣ 检查配置文件..."
if [ ! -f .env ]; then
    echo "⚠️  .env文件不存在，从模板创建..."
    cp .env.example .env
    echo "❗ 请编辑.env文件，填入你的API Key："
    echo "   ZHIPU_API_KEY=your_api_key_here"
    echo ""
    read -p "是否现在编辑？(y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        ${EDITOR:-vi} .env
    fi
fi

# 下载依赖
echo "4️⃣ 下载Go依赖..."
go mod download
echo "✅ 依赖下载完成"

# 启动Docker服务
echo "5️⃣ 启动Docker服务..."
docker-compose -f deployments/docker/docker-compose.yml up -d
echo "✅ Docker服务已启动"

# 等待服务就绪
echo "6️⃣ 等待服务就绪..."
sleep 5

# 运行测试
echo "7️⃣ 运行测试..."
go run cmd/server/main.go

echo ""
echo "🎉 启动完成！"
echo ""
echo "📝 有用的命令："
echo "  make help          - 查看所有命令"
echo "  make run           - 运行主程序"
echo "  make test          - 运行测试"
echo "  make docker-logs   - 查看Docker日志"
echo "  make docker-down   - 停止Docker服务"
