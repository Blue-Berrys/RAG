# CookRAG-Go API 压测脚本

本目录包含用于压测 CookRAG-Go API 服务器的脚本。

## 前置条件

### 1. 启动 API 服务器

```bash
# 在 cookrag-go 根目录下
cd /Users/mac/PycharmProjects/all-in-rag/cookrag-go
go run cmd/server/main.go
```

服务器将在 `http://localhost:8080` 启动。

### 2. 安装压测工具（选择一个）

#### 选项 A: hey (推荐)
```bash
go install github.com/rakyll/hey@latest
```

#### 选项 B: wrk
```bash
brew install wrk
```

#### 选项 C: Apache Bench (ab)
```bash
brew install httpd
```

#### 选项 D: locust (Python)
```bash
pip install locust
```

## 压测脚本说明

### 1. `health-check.sh` - 健康检查压测
测试 `GET /api/v1/health` 端点。

```bash
./health-check.sh

# 自定义参数
CONCURRENCY=50 DURATION=60s ./health-check.sh
```

### 2. `query-api.sh` - 查询接口压测
测试 `POST /api/v1/query` 端点。

```bash
./query-api.sh

# 自定义参数
CONCURRENCY=20 REQUESTS=500 ./query-api.sh
```

### 3. `stress-test.sh` - 综合压测
运行多个测试场景，包含不同并发级别。

```bash
./stress-test.sh

# 指定结果目录
RESULTS_DIR=./my-results ./stress-test.sh
```

### 4. `quick-bench.sh` - 快速基准测试
使用 curl 进行简单的响应时间测试。

```bash
./quick-bench.sh

# 自定义迭代次数
ITERATIONS=50 ./quick-bench.sh
```

## API 端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/health` | GET | 健康检查 |
| `/api/v1/ready` | GET | 就绪检查 |
| `/api/v1/metrics` | GET | Prometheus 指标 |
| `/api/v1/query` | POST | RAG 查询接口 |

## 测试场景

### 场景 1: 低并发基准测试
- 并发: 10
- 请求: 1000
- 目的: 建立基准性能

### 场景 2: 中等并发测试
- 并发: 50
- 请求: 5000
- 目的: 模拟正常负载

### 场景 3: 高并发压力测试
- 并发: 100+
- 请求: 10000+
- 目的: 找出性能瓶颈

### 场景 4: 持续负载测试
- 并发: 50
- 持续时间: 5-10 分钟
- 目的: 检测内存泄漏和稳定性

## 环境变量

| 变量 | 默认值 | 描述 |
|------|--------|------|
| `HOST` | `http://localhost:8080` | API 服务器地址 |
| `CONCURRENCY` | `10` | 并发连接数 |
| `DURATION` | `30s` | 测试持续时间 |
| `REQUESTS` | `1000` | 总请求数 |
| `RESULTS_DIR` | `./load-test-results` | 结果保存目录 |

## 输出示例

### hey 工具输出
```
Summary:
  Success rate: 100.00%
  Total:        1.2345 secs
  Slowest:      0.0234 secs
  Fastest:      0.0012 secs
  Average:      0.0034 secs
  Requests/sec: 810.12

  Latency distribution:
    10%: 0.0015 secs
    25%: 0.0020 secs
    50%: 0.0030 secs
    75%: 0.0040 secs
    90%: 0.0060 secs
    95%: 0.0080 secs
    99%: 0.0150 secs
```

## 性能指标

关注以下关键指标：

1. **QPS (Queries Per Second)**: 每秒处理请求数
2. **平均响应时间**: 请求平均处理时间
3. **P95/P99 延迟**: 95%/99% 请求的响应时间
4. **成功率**: 非 5xx 响应的百分比
5. **错误率**: 失败请求的百分比

## 故障排查

### 服务器未启动
```bash
curl http://localhost:8080/api/v1/health
```

### 端口被占用
```bash
lsof -i :8080
```

### 查看服务器日志
服务器日志会显示请求详情和响应时间。

## 注意事项

1. 在生产环境前，先在测试环境验证
2. 逐步增加并发，避免突然的高负载
3. 监控服务器资源使用 (CPU、内存、网络)
4. 压测期间避免在生产环境运行
5. 确保有足够的 API 配额（如调用外部 LLM）
