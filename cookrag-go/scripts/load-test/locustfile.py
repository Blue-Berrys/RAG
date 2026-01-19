"""
CookRAG-Go API Locust 压测脚本

使用方法:
1. 安装 locust: pip install locust
2. 启动服务器: go run cmd/server/main.go
3. 运行压测: locust -f locustfile.py
4. 打开浏览器: http://localhost:8088
"""

import json
import time
from locust import HttpUser, task, between


class CookRAGUser(HttpUser):
    """
    CookRAG-Go API 用户行为模拟
    """
    # 等待时间: 1-3秒之间
    wait_time = between(1, 3)

    def on_start(self):
        """压测开始时的初始化操作"""
        # 检查服务健康状态
        self.client.get("/api/v1/health")

    @task(3)
    def health_check(self):
        """健康检查 (权重: 3)"""
        with self.client.get("/api/v1/health", catch_response=True) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"Health check failed: {response.status_code}")

    @task(2)
    def get_metrics(self):
        """获取指标 (权重: 2)"""
        with self.client.get("/api/v1/metrics", catch_response=True) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"Metrics failed: {response.status_code}")

    @task(1)
    def query_rag(self):
        """RAG 查询 (权重: 1, 因为涉及 LLM 调用，较慢)"""
        query_data = {
            "query": "什么是红烧肉？"
        }

        with self.client.post(
            "/api/v1/query",
            json=query_data,
            catch_response=True,
            timeout=30  # 30秒超时
        ) as response:
            if response.status_code == 200:
                try:
                    data = response.json()
                    # 检查响应是否包含答案
                    if "answer" in data or "result" in data:
                        response.success()
                    else:
                        response.failure("No answer in response")
                except json.JSONDecodeError:
                    response.failure("Invalid JSON response")
            else:
                response.failure(f"Query failed: {response.status_code}")

    @task(1)
    def ready_check(self):
        """就绪检查 (权重: 1)"""
        with self.client.get("/api/v1/ready", catch_response=True) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"Ready check failed: {response.status_code}")


class FastUser(HttpUser):
    """
    快速用户 - 只测试健康检查和指标接口
    用于高并发轻量级测试
    """
    wait_time = between(0.5, 1.5)

    @task(5)
    def health_check(self):
        """健康检查"""
        self.client.get("/api/v1/health")

    @task(1)
    def get_metrics(self):
        """获取指标"""
        self.client.get("/api/v1/metrics")


class QueryOnlyUser(HttpUser):
    """
    仅查询用户 - 只测试 RAG 查询接口
    用于测试 LLM 生成性能
    """
    wait_time = between(3, 6)  # 更长的等待时间，因为 LLM 调用较慢

    @task
    def query_rag(self):
        """RAG 查询"""
        queries = [
            "什么是红烧肉？",
            "宫保鸡丁怎么做？",
            "麻婆豆腐的起源",
            "川菜的特点",
            "中国八大菜系"
        ]

        import random
        query_data = {
            "query": random.choice(queries)
        }

        with self.client.post(
            "/api/v1/query",
            json=query_data,
            catch_response=True,
            timeout=30
        ) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"Query failed: {response.status_code}")


# Locust 配置建议
# 在命令行运行时可以指定:
#
# 基础测试:
#   locust -f locustfile.py --host=http://localhost:8080
#
# 指定用户数和生成速率:
#   locust -f locustfile.py --host=http://localhost:8080 --users 100 --spawn-rate 10
#
# 无头模式 (不启动 Web UI):
#   locust -f locustfile.py --host=http://localhost:8080 --headless --users 100 --spawn-rate 10 -t 1m
#
# 只运行特定用户类型:
#   locust -f locustfile.py --host=http://localhost:8080 CookRAGUser
