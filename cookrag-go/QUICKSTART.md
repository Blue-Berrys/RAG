# 🚀 快速开始指南

## 你需要做的2件事：

### 1️⃣ 获取智谱AI API Key（免费）

1. 访问: https://open.bigmodel.cn/usercenter/apikeys
2. 注册/登录（手机号即可）
3. 点击"创建API Key"
4. 复制你的API Key

### 2️⃣ 设置环境变量并运行

**方法A: 使用启动脚本（推荐）**

```bash
cd cookrag-go

# 编辑.env文件，填入你的API Key
echo 'export ZHIPU_API_KEY="你的API密钥"' >> .env

# 如果你安装了Neo4j，也设置密码
echo 'export NEO4J_PASSWORD="neo4j"' >> .env

# 运行
./run.sh
```

**方法B: 手动运行**

```bash
cd cookrag-go

# 设置环境变量
export ZHIPU_API_KEY="你的API密钥"
export NEO4J_PASSWORD="neo4j"  # Neo4j密码，默认是neo4j或password

# 运行
go run cmd/demo/main.go
```

## 检查是否成功

成功的输出应该包含：
```
✅ Embedding provider initialized: zhipu (dimension: 1024)
✅ Connected to Milvus: localhost:19530
✅ Connected to Neo4j: bolt://localhost:7687
✅ Redis client connected
✅ LLM provider initialized
✅ BM25 indexing completed: 342 docs, avg_len: 254.47, 8633 unique terms
```

系统会自动运行演示查询，你应该看到：
```
🔀 Routing to Hybrid Retrieval
✅ Hybrid retrieval completed: 10 results
```

这表示混合检索（向量+BM25）正在工作！

## 如果看到警告

### ⚠️ "Failed to connect to Neo4j: authentication failure"

**解决:**
```bash
# 确认Neo4j密码
export NEO4J_PASSWORD="正确的密码"

# 或者如果你没有安装Neo4j，可以暂时忽略这个警告
```

### ⚠️ "Failed to initialize LLM: ZHIPU_API_KEY environment variable not set"

**解决:**
```bash
# 确保你设置了环境变量
export ZHIPU_API_KEY="你的实际API密钥"

# 然后重新运行
go run cmd/demo/main.go
```

## 启动依赖服务（可选）

如果你想使用完整功能，需要启动数据库：

```bash
cd cookrag-go/deployments/docker
docker-compose up -d

# 等待服务启动
sleep 10

# 检查状态
docker-compose ps
```

## 测试API

服务启动后，访问 http://localhost:8080

```bash
# 测试查询接口
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"query": "红烧肉怎么做？"}'
```

## 🎓 下一步

- 阅读 [SETUP.md](SETUP.md) 了解详细配置
- 查看 [CookRAG-Go-Development-Guide.md](../CookRAG-Go-Development-Guide.md) 了解架构
- 查看 config/config.yaml 自定义配置

## ❓ 常见问题

**Q: 智谱AI真的免费吗？**
A: 是的，智谱AI对新用户完全免费，无需信用卡。

**Q: 可以不设置Neo4j吗？**
A: 可以，程序会警告但继续运行，只是无法使用图检索功能。

**Q: 如何停止程序？**
A: 按 Ctrl+C

**Q: API Key会泄露吗？**
A: 不会，只要不把.env文件提交到Git即可。.gitignore已配置忽略.env。
