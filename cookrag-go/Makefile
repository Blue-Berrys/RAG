.PHONY: help build run test clean deps docker-up docker-down import-data demo

help:  ## 显示帮助信息
	@echo "CookRAG-Go 开发命令"
	@echo ""
	@echo "使用方法:"
	@echo "  make [命令]"
	@echo ""
	@echo "可用命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

deps:  ## 下载依赖
	@echo "📦 下载Go依赖..."
	go mod download
	@echo "✅ 依赖下载完成"

build:  ## 编译项目
	@echo "🔨 编译项目..."
	go build -o cookrag-server cmd/server/main.go
	@echo "✅ 编译完成: ./cookrag-server"

run:  ## 运行主程序
	@echo "🚀 运行CookRAG-Go服务器..."
	go run cmd/server/main.go

test:  ## 运行测试
	@echo "🧪 运行测试..."
	go test -v ./...

test-coverage:  ## 运行测试并生成覆盖率报告
	@echo "🧪 运行测试（覆盖率）..."
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ 覆盖率报告: coverage.html"

lint:  ## 代码检查
	@echo "🔍 代码检查..."
	golangci-lint run

fmt:  ## 格式化代码
	@echo "💅 格式化代码..."
	goimports -w .
	go fmt ./...

clean:  ## 清理编译文件
	@echo "🧹 清理编译文件..."
	rm -f cookrag-server
	rm -f coverage.out coverage.html
	@echo "✅ 清理完成"

docker-up:  ## 启动Docker服务
	@echo "🐳 启动Docker服务..."
	docker-compose up -d
	@echo "✅ Docker服务已启动"

docker-down:  ## 停止Docker服务
	@echo "🐳 停止Docker服务..."
	docker-compose down
	@echo "✅ Docker服务已停止"

docker-logs:  ## 查看Docker日志
	docker-compose logs -f

init: deps docker-up  ## 初始化项目（下载依赖+启动Docker）

import-data:  ## 导入示例数据
	@echo "📚 导入示例数据..."
	go run cmd/import/main.go
	@echo "✅ 数据导入完成"

demo:  ## 运行完整演示
	@echo "🎮 运行完整演示..."
	go run cmd/demo/main.go
	@echo "✅ 演示完成"

all: fmt lint test build  ## 执行所有检查并编译
