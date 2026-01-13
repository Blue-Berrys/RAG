# 数据导入完整指南

## 🎯 快速开始

### 一键导入（推荐）

```bash
# 1. 启动依赖服务
make docker-up

# 2. 导入示例数据（内置10个菜谱）
make import-data

# 3. 运行演示程序
make demo
```

## 📚 数据来源选项

### 选项1: 使用内置示例数据（最快）

**无需下载，直接运行**：

```bash
go run cmd/import/main.go
```

**数据内容**：
- 10个经典中式菜谱
- 包括：红烧肉、宫保鸡丁、麻婆豆腐等
- 每个菜谱包含：菜名、分类、菜系、做法

### 选项2: 使用示例数据文件

项目已包含示例数据文件：

```bash
data/recipes/recipes.json
```

包含10个完整菜谱数据：

```json
{
  "name": "红烧肉",
  "category": "肉类",
  "cuisine": "中式",
  "tags": ["经典", "家常菜", "猪肉"],
  "ingredients": ["五花肉 500g", "冰糖 30g", ...],
  "steps": ["五花肉切块，冷水下锅焯水...", ...]
}
```

### 选项3: 下载开源数据集

#### GitHub数据集

```bash
# 1. 克隆数据集仓库
cd data/recipes
git clone https://github.com/Andreas2021/Chinese-recipes-dataset.git

# 2. 转换格式（如果需要）
# 使用 Python 脚本转换

# 3. 导入
go run ../../cmd/import/main.go
```

#### Kaggle数据集

```bash
# 1. 访问 Kaggle 网站
https://www.kaggle.com/datasets

# 2. 搜索 "chinese recipes" 或 "recipe"

# 3. 下载数据集

# 4. 转换为JSON格式（见下方转换脚本）

# 5. 导入
go run cmd/import/main.go
```

## 🔄 数据格式转换

### 从CSV转换

假设你有一个CSV文件 `recipes.csv`：

```csv
菜名,分类,菜系,食材,步骤
红烧肉,肉类,中式,"五花肉,冰糖,酱油","1. 焯水 2. 炒糖色 3. 焖煮"
宫保鸡丁,肉类,川菜,"鸡肉,花生,辣椒","1. 腌制 2. 炸花生 3. 炒制"
```

**Python转换脚本**：

```python
import pandas as pd
import json

# 读取CSV
df = pd.read_csv('recipes.csv')

# 转换为JSON格式
recipes = []
for _, row in df.iterrows():
    recipe = {
        "name": row['菜名'],
        "category": row['分类'],
        "cuisine": row['菜系'],
        "tags": [],
        "ingredients": row['食材'].split(','),
        "steps": row['步骤'].split('.')
    }
    recipes.append(recipe)

# 保存为JSON
with open('recipes.json', 'w', encoding='utf-8') as f:
    json.dump(recipes, f, ensure_ascii=False, indent=2)

print(f"✅ 转换完成：{len(recipes)} 个菜谱")
```

### 从Excel转换

```python
import pandas as pd
import json

# 读取Excel
df = pd.read_excel('菜谱大全.xlsx', sheet_name='菜谱')

# 转换
recipes = []
for _, row in df.iterrows():
    recipe = {
        "name": row['菜名'],
        "category": row['分类'],
        "cuisine": row['菜系'],
        "tags": row.get('标签', '').split(',') if pd.notna(row.get('标签')) else [],
        "ingredients": str(row['食材']).split('\n') if pd.notna(row['食材']) else [],
        "steps": str(row['步骤']).split('\n') if pd.notna(row['步骤']) else []
    }
    recipes.append(recipe)

# 保存
with open('recipes.json', 'w', encoding='utf-8') as f:
    json.dump(recipes, f, ensure_ascii=False, indent=2)
```

### 从爬虫数据转换

```python
import json
import requests
from bs4 import BeautifulSoup

def scrape_recipes(url):
    """爬取菜谱网站"""
    response = requests.get(url)
    soup = BeautifulSoup(response.content, 'html.parser')

    recipes = []
    # 根据网站结构调整选择器
    recipe_items = soup.find_all('div', class_='recipe-item')

    for item in recipe_items:
        recipe = {
            "name": item.find('h3').text.strip(),
            "category": item.find('span', class_='category').text.strip(),
            "cuisine": "中式",
            "tags": [],
            "ingredients": [],
            "steps": []
        }
        recipes.append(recipe)

    return recipes

# 爬取并保存
recipes = scrape_recipes('https://example.com/recipes')
with open('scraped_recipes.json', 'w', encoding='utf-8') as f:
    json.dump(recipes, f, ensure_ascii=False, indent=2)
```

## 🚀 导入流程详解

### 完整导入流程

```bash
# 1. 准备环境
# 1.1 配置API Key（智谱AI，完全免费）
cp .env.example .env
echo "ZHIPU_API_KEY=your_api_key_here" > .env

# 1.2 启动依赖服务（Milvus、Neo4j、Redis）
docker-compose -f deployments/docker/docker-compose.yml up -d

# 1.3 检查服务状态
docker-compose -f deployments/docker/docker-compose-compose.yml ps

# 2. 下载依赖
go mod download

# 3. 导入数据
go run cmd/import/main.go
```

### 导入过程说明

**程序会自动完成以下步骤**：

1. ✅ 加载配置文件
2. ✅ 初始化Embedding提供者（智谱AI）
3. ✅ 连接Milvus、Neo4j
4. ✅ 加载数据文件（或使用内置数据）
5. ✅ 创建向量索引（调用智谱AI生成向量）
6. ✅ 创建BM25索引
7. ✅ 验证索引（显示统计信息）
8. ✅ 测试检索（运行示例查询）

**预期输出**：

```
🚀 Starting CookRAG-Go Data Importer...
✅ Config loaded
✅ Embedding provider initialized: zhipu
✅ Milvus client connected
✅ Neo4j client connected

📚 Loading data...
✅ Loaded 10 documents

📊 Starting indexing...
🔤 Creating vector index...
📦 Creating Milvus collection: cookrag_documents
🔤 Generating embeddings for 10 documents...
📝 Inserting documents into Milvus...
✅ Inserted 10 documents

📝 Creating BM25 index...
📝 Indexing 10 documents with BM25
✅ BM25 indexing completed: 10 docs

✅ Indexing completed: 10 documents in 2m 30s

🔍 Testing retrieval...
🔍 Query: 红烧肉怎么做？
✅ BM25 found 3 results
  [1] Score: 2.1534, Content: 红烧肉是一道经典的中国菜...
  [2] Score: 1.8231, Content: 糖醋排骨是经典酸甜口味菜肴...

🎉 Data import completed successfully!
```

## 📊 数据量参考

| 数据量 | 文档数 | 导入时间 | Embedding调用 | 适用场景 |
|--------|--------|----------|---------------|----------|
| **小型** | 10-100 | 1-5分钟 | 1-10次 | 快速测试 |
| **中型** | 100-1,000 | 5-30分钟 | 10-100次 | 功能演示 |
| **大型** | 1,000-10,000 | 30分钟-2小时 | 100-1,000次 | 生产环境 |
| **超大型** | 10,000+ | 2小时+ | 1,000+次 | 企业应用 |

**注意**：
- 智谱AI免费版有调用频率限制
- 大批量导入建议分批处理
- 可以调整批量大小优化速度

## 🛠️ 故障排查

### 问题1: API Key错误

```
Error: ZHIPU_API_KEY environment variable not set
```

**解决**：
```bash
# 检查.env文件
cat .env

# 确保包含：
ZHIPU_API_KEY=your_actual_api_key_here
```

### 问题2: Milvus连接失败

```
Error: failed to connect to Milvus
```

**解决**：
```bash
# 检查Docker服务
docker-compose -f deployments/docker/docker-compose.yml ps

# 查看Milvus日志
docker-compose -f deployments/docker/docker-compose.yml logs milvus-standalone

# 重启服务
docker-compose -f deployments/docker/docker-compose.yml restart
```

### 问题3: Embedding调用失败

```
Error: failed to generate embeddings
```

**解决**：
```bash
# 检查API Key是否有效
# 访问 https://open.bigmodel.cn/ 验证

# 检查网络连接
ping open.bigmodel.cn

# 如果网络问题，可以：
# 1. 使用代理
# 2. 更换其他Embedding提供商（百度、阿里等）
```

### 问题4: 内存不足

```
Error: out of memory
```

**解决**：
```bash
# 调整批量大小
# 编辑 cmd/import/main.go
indexConfig := &data.IndexConfig{
    BatchSize: 5,  # 减小批量大小
}

# 或者分批导入
# 将大数据集分成多个小文件
```

## 🎯 下一步

数据导入成功后：

```bash
# 1. 运行完整演示
go run cmd/demo/main.go

# 2. 测试HTTP API
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"query": "红烧肉怎么做？"}'

# 3. 查看监控指标
curl http://localhost:8080/api/v1/metrics

# 4. 添加更多数据
# 将新数据放到 data/recipes/ 目录
# 重新运行导入程序
```

## 📖 更多数据源

详细的数据获取指南请参考：

```bash
docs/DATA_SOURCES.md
```

包含：
- 推荐的数据集网站
- 数据格式转换脚本
- 爬虫示例代码
- 批量导入优化技巧

## 🎉 总结

现在你可以：

✅ 使用内置的10个示例菜谱快速测试
✅ 下载开源数据集进行演示
✅ 转换自己的数据格式
✅ 批量导入大量数据
✅ 验证索引效果

**快速开始命令**：

```bash
make import-data  # 导入数据
make demo         # 运行演示
```

祝你使用愉快！🚀
