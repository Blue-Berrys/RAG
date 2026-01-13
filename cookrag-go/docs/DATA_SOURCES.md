# 数据获取指南 - 大批量数据导入方案

## 🎯 推荐数据集来源

### 1. 中文菜谱数据集（推荐用于本项目）

#### 选项1: GitHub开源数据集
```bash
# 中文菜谱数据集
https://github.com/Andreas2021/Chinese-recipes-dataset

# 菜谱JSON数据
https://github.com/richardzitran/chinese-cooking-recipes

# 美食菜谱大全
https://github.com/meilic/recipe-dataset
```

#### 选项2: Kaggle数据集
```bash
# Recipe1M+ 大规模菜谱数据集（100万+菜谱）
https://www.kaggle.com/datasets/paulmoise/predictions-of-chef-cooking-time

# Food.com 菜谱数据集
https://www.kaggle.com/datasets/shuyangli94/food-com-recipes-and-user-interactions

# 中文菜谱数据
https://www.kaggle.com/datasets (搜索 "chinese recipes")
```

#### 选项3: Hugging Face数据集
```python
from datasets import load_dataset

# 加载菜谱数据集
dataset = load_dataset("recipe_nl", "recipes")
# 或
dataset = load_dataset("food_dataloader")
```

### 2. 通用文本数据集（用于测试）

#### 中文文本数据集
```bash
# Wikipedia中文数据
https://dumps.wikimedia.org/zhwiki/latest/

# 中文问答数据集
https://github.com/chiLi0905/NLP-Chinese-DataSet

# 中文文本分类数据集
https://github.com/sketu/jieba-wordline
```

#### 英文文本数据集
```bash
# SQuAD问答数据集
https://rajpurkar.github.io/SQuAD-explorer/

# MS MARCO
https://microsoft.github.io/msmarco/

# Wikipedia数据
https://dumps.wikimedia.org/enwiki/latest/
```

### 3. 爬虫获取数据

#### 使用Python爬虫
```python
import requests
from bs4 import BeautifulSoup
import json

# 示例：爬取菜谱网站
def scrape_recipes(url):
    response = requests.get(url)
    soup = BeautifulSoup(response.content, 'html.parser')

    recipes = []
    # 提取菜谱信息
    # ...

    return recipes

# 保存为JSON
with open('recipes.json', 'w', encoding='utf-8') as f:
    json.dump(recipes, f, ensure_ascii=False, indent=2)
```

#### 推荐爬取的网站
- 下厨房 (https://www.xiachufang.com/)
- 豆果美食 (https://www.douguo.com/)
- 美食杰 (https://www.meishij.net/)

## 📥 数据格式转换

### 1. 从JSON转换
我们已经支持JSON格式，确保你的数据符合以下格式：

```json
[
  {
    "name": "菜名",
    "category": "分类",
    "cuisine": "菜系",
    "tags": ["标签1", "标签2"],
    "ingredients": ["食材1", "食材2"],
    "steps": ["步骤1", "步骤2"]
  }
]
```

### 2. 从CSV转换
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
        "ingredients": row['食材'].split(','),
        "steps": row['步骤'].split('|')
    }
    recipes.append(recipe)

# 保存为JSON
with open('recipes.json', 'w', encoding='utf-8') as f:
    json.dump(recipes, f, ensure_ascii=False, indent=2)
```

### 3. 从Markdown转换
```python
import json
import os
import re

def parse_markdown_recipes(md_file):
    recipes = []
    with open(md_file, 'r', encoding='utf-8') as f:
        content = f.read()

    # 解析Markdown格式的菜谱
    # 根据实际格式调整解析逻辑
    # ...

    return recipes

# 批量转换
recipes = parse_markdown_recipes('菜谱大全.md')
with open('recipes.json', 'w', encoding='utf-8') as f:
    json.dump(recipes, f, ensure_ascii=False, indent=2)
```

## 🚀 使用导入工具

### 1. 基础导入

```bash
# 1. 准备数据文件
# 将数据文件放在 data/recipes/recipes.json

# 2. 启动依赖服务
docker-compose -f deployments/docker/docker-compose.yml up -d

# 3. 运行导入程序
go run cmd/import/main.go
```

### 2. 自定义导入

创建你自己的导入脚本：

```go
package main

import (
    "context"
    "log"
    "cookrag-go/pkg/data"
    "cookrag-go/internal/config"
    "cookrag-go/pkg/ml/embedding"
    "cookrag-go/pkg/storage/milvus"
)

func main() {
    // 加载配置
    cfg, _ := config.Load("config/config.yaml")

    // 初始化
    embeddingProvider, _ := embedding.NewProvider(cfg.Embedding)
    milvusClient, _ := milvus.NewClient(cfg.Milvus.Host, cfg.Milvus.Port)

    // 创建索引器
    indexer := data.NewIndexer(embeddingProvider, milvusClient, nil)

    // 加载你的数据
    loader := data.NewJSONLoader("path/to/your/data.json")
    docs, _ := loader.Load(context.Background())

    // 索引
    config := &data.IndexConfig{
        CollectionName: "my_collection",
        VectorIndex: true,
        BM25Index: true,
        BatchSize: 100,
    }

    indexer.IndexDocuments(context.Background(), docs, config)
}
```

## 📊 数据量建议

### 测试环境
- **文档数**: 100-1,000篇
- **用途**: 功能测试、开发调试
- **导入时间**: 2-10分钟

### 演示环境
- **文档数**: 1,000-10,000篇
- **用途**: 面试演示、POC
- **导入时间**: 10-60分钟

### 生产环境
- **文档数**: 100,000+篇
- **用途**: 实际应用
- **导入时间**: 1小时+（需要优化）

## 🎯 推荐的数据集

### 快速开始（内置数据）
```bash
# 项目已包含10个示例菜谱
go run cmd/import/main.go
```

### 小型数据集（100-500个菜谱）
```bash
# 下载数据集
cd data/recipes
wget https://raw.githubusercontent.com/Andreas2021/Chinese-recipes-dataset/main/recipes.json

# 导入
go run cmd/import/main.go
```

### 中型数据集（1000-5000个菜谱）
```bash
# 从Kaggle下载
# 1. 访问 https://www.kaggle.com/datasets
# 2. 搜索 "chinese recipes"
# 3. 下载数据集
# 4. 转换为JSON格式
# 5. 导入
```

### 大型数据集（10,000+菜谱）
```bash
# Recipe1M+ 数据集
https://www.kaggle.com/datasets/paulmoise/predictions-of-chef-cooking-time

# 批量导入（需要优化批处理大小）
go run cmd/import/main.go --batch-size 1000
```

## 🔧 数据预处理

### 1. 数据清洗
```python
import json
import re

def clean_text(text):
    # 去除多余空格
    text = re.sub(r'\s+', ' ', text)
    # 去除特殊字符
    text = re.sub(r'[^\w\s\u4e00-\u9fff]', '', text)
    return text.strip()

# 清洗数据
with open('raw_recipes.json', 'r', encoding='utf-8') as f:
    recipes = json.load(f)

for recipe in recipes:
    recipe['name'] = clean_text(recipe['name'])
    recipe['steps'] = [clean_text(step) for step in recipe['steps']]

with open('cleaned_recipes.json', 'w', encoding='utf-8') as f:
    json.dump(recipes, f, ensure_ascii=False, indent=2)
```

### 2. 数据增强（可选）
```python
# 为菜谱添加更多元数据
for recipe in recipes:
    # 添加难度等级
    if len(recipe['steps']) > 5:
        recipe['difficulty'] = '困难'
    elif len(recipe['steps']) > 3:
        recipe['difficulty'] = '中等'
    else:
        recipe['difficulty'] = '简单'

    # 添加时间估算
    recipe['time_estimate'] = len(recipe['steps']) * 5  # 分钟
```

## 📈 性能优化建议

### 1. 批量处理优化
```go
// 调整批量大小
config := &data.IndexConfig{
    BatchSize: 100,  // 根据API限制调整
}
```

### 2. 并发处理
```go
// 使用goroutine并发处理
func batchProcess(docs []models.Document, batchSize int) {
    for i := 0; i < len(docs); i += batchSize {
        end := i + batchSize
        if end > len(docs) {
            end = len(docs)
        }
        batch := docs[i:end]

        // 并发处理
        go processBatch(batch)
    }
}
```

### 3. 错误处理和重试
```go
// 添加重试机制
for retry := 0; retry < 3; retry++ {
    err := indexer.IndexDocuments(ctx, docs, config)
    if err == nil {
        break
    }
    log.Warnf("Retry %d: %v", retry+1, err)
    time.Sleep(time.Second * time.Duration(retry+1))
}
```

## 🎉 总结

**快速开始**：
```bash
# 1. 内置数据（10个菜谱）
go run cmd/import/main.go

# 2. 下载更多数据
# 访问推荐的数据源网站
# 下载数据并转换为JSON格式
# 放到 data/recipes/ 目录

# 3. 重新导入
go run cmd/import/main.go
```

**推荐数据源**：
1. GitHub开源数据集（免费、易获取）
2. Kaggle数据集（高质量、有标注）
3. Hugging Face（格式标准、易于使用）
4. 自行爬取（定制化、符合需求）

现在你可以轻松获取大量数据来测试你的RAG系统了！🚀
