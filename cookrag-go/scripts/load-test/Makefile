# CookRAG-Go Load Testing Makefile
# 使用方法: make <target>

# 默认配置
HOST ?= http://localhost:8080
CONCURRENCY ?= 50
DURATION ?= 30s
REQUESTS ?= 1000
RESULTS_DIR ?= ./load-test-results

# 颜色输出
COLOR_RESET = \033[0m
COLOR_GREEN = \033[32m
COLOR_YELLOW = \033[33m
COLOR_BLUE = \033[34m

.PHONY: help test-health test-query test-all quick-bench locust locust-headless clean

# 默认目标
help:
	@echo "$(COLOR_BLUE)CookRAG-Go 压测命令$(COLOR_RESET)"
	@echo ""
	@echo "使用方法: make <target> [VAR=value]"
	@echo ""
	@echo "可用目标:"
	@echo "  $(COLOR_GREEN)help$(COLOR_RESET)           - 显示此帮助信息"
	@echo "  $(COLOR_GREEN)test-health$(COLOR_RESET)    - 压测健康检查接口"
	@echo "  $(COLOR_GREEN)test-query$(COLOR_RESET)     - 压测查询接口"
	@echo "  $(COLOR_GREEN)test-all$(COLOR_RESET)       - 运行综合压测"
	@echo "  $(COLOR_GREEN)quick-bench$(COLOR_RESET)    - 快速基准测试"
	@echo "  $(COLOR_GREEN)locust$(COLOR_RESET)         - 启动 Locust Web UI"
	@echo "  $(COLOR_GREEN)locust-headless$(COLOR_RESET) - 无头模式运行 Locust"
	@echo "  $(COLOR_GREEN)clean$(COLOR_RESET)          - 清理测试结果"
	@echo ""
	@echo "环境变量:"
	@echo "  HOST=$(HOST)"
	@echo "  CONCURRENCY=$(CONCURRENCY)"
	@echo "  DURATION=$(DURATION)"
	@echo "  REQUESTS=$(REQUESTS)"
	@echo "  RESULTS_DIR=$(RESULTS_DIR)"
	@echo ""
	@echo "示例:"
	@echo "  make test-health CONCURRENCY=100"
	@echo "  make test-all RESULTS_DIR=./my-results"

test-health:
	@echo "$(COLOR_YELLOW)压测健康检查接口...$(COLOR_RESET)"
	@HOST=$(HOST) CONCURRENCY=$(CONCURRENCY) DURATION=$(DURATION) ./health-check.sh

test-query:
	@echo "$(COLOR_YELLOW)压测查询接口...$(COLOR_RESET)"
	@HOST=$(HOST) CONCURRENCY=$(CONCURRENCY) REQUESTS=$(REQUESTS) ./query-api.sh

test-all:
	@echo "$(COLOR_YELLOW)运行综合压测...$(COLOR_RESET)"
	@HOST=$(HOST) RESULTS_DIR=$(RESULTS_DIR) ./stress-test.sh

quick-bench:
	@echo "$(COLOR_YELLOW)运行快速基准测试...$(COLOR_RESET)"
	@HOST=$(HOST) ITERATIONS=100 ./quick-bench.sh

locust:
	@echo "$(COLOR_YELLOW)启动 Locust Web UI...$(COLOR_RESET)"
	@echo "打开浏览器访问: http://localhost:8088"
	@locust -f locustfile.py --host=$(HOST)

locust-headless:
	@echo "$(COLOR_YELLOW)运行 Locust 无头模式...$(COLOR_RESET)"
	@echo "用户数: $(CONCURRENCY), 生成速率: 10, 持续时间: $(DURATION)"
	@locust -f locustfile.py --host=$(HOST) --headless --users $(CONCURRENCY) --spawn-rate 10 -t $(DURATION) --html $(RESULTS_DIR)/locust_report.html

clean:
	@echo "$(COLOR_YELLOW)清理测试结果...$(COLOR_RESET)"
	@rm -rf $(RESULTS_DIR)
	@rm -f *.txt
	@echo "$(COLOR_GREEN)清理完成$(COLOR_RESET)"

# 检查服务器是否运行
check-server:
	@echo "$(COLOR_YELLOW)检查服务器状态...$(COLOR_RESET)"
	@curl -s $(HOST)/api/v1/health || echo "$(COLOR_YELLOW)服务器未运行或无法访问$(COLOR_RESET)"

# 安装压测工具
install-tools:
	@echo "$(COLOR_YELLOW)安装压测工具...$(COLOR_RESET)"
	@echo "安装 hey..."
	@go install github.com/rakyll/hey@latest || echo "$(COLOR_YELLOW)hey 安装失败 (需要 Go)$(COLOR_RESET)"
	@echo "安装 locust..."
	@pip3 install locust || echo "$(COLOR_YELLOW)locust 安装失败 (需要 Python)$(COLOR_RESET)"
	@echo "$(COLOR_GREEN)安装完成$(COLOR_RESET)"
	@echo "提示: 使用 'brew install wrk' 安装 wrk"
	@echo "提示: 使用 'brew install httpd' 安装 ab"
