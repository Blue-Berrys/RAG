# CookRAG-Go 开发文档

## 目录

1. [项目概述](#项目概述)
2. [系统架构](#系统架构)
3. [技术栈](#技术栈)
4. [目录结构](#目录结构)
5. [核心模块详解](#核心模块详解)
6. [可观测性](#可观测性)
7. [数据流程](#数据流程)
8. [配置说明](#配置说明)
9. [开发指南](#开发指南)
10. [部署说明](#部署说明)
11. [API 接口](#api-接口)

---

## 项目概述

**CookRAG-Go** 是一个企业级的多模态检索增强生成（RAG）系统，专门为菜谱知识问答设计。

### 核心特性

| 特性 | 描述 |
|------|------|
| **混合检索（默认）** | **向量语义理解 + BM25关键词精确匹配，RRF算法融合** |
| **智能路由** | 关系查询自动切换图检索，其他使用混合检索 |
| **知识图谱** | 342 份菜谱 → 537 个食材实体 + 341 个菜品实体 + 3449 条关系 |
| **中文优化** | jieba 分词、停用词过滤、标点符号处理 |
| **LLM 生成** | 智谱 AI GLM-4-flash 模型（免费） |

### 测试结果

```
📊 系统性能指标：
- 平均检索延迟: ~125ms
- 错误率: 0%
- 成功率: 100% (4/4 查询)
- 策略分布: 混合检索 100%
- BM25索引: 342 文档, 8633 唯一词
- 图谱规模: 341 菜品 + 537 食材 + 3449 关系
```

---

## 系统架构

### 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                      用户查询 (Query)                         │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │  智能查询路由 (Router)  │
              └──────────┬───────────┘
                         │
         ┌───────────────┴───────────────┐
         │                               │
         ▼                               ▼
    ┌─────────┐                   ┌─────────┐
    │ 混合检索 │                   │   图谱   │
    │ (默认)   │                   │  检索   │
    └────┬────┘                   └────┬────┘
         │                              │
    ┌────┴────┐                        │
    │         │                        │
    ▼         ▼                        │
┌─────────┐ ┌─────────┐                │
│  向量   │ │  BM25   │                │
│ 语义理解 │ │关键词匹配│                │
└────┬────┘ └────┬────┘                │
     │           │                     │
     └─────┬─────┘                     │
           │ (RRF融合)                  │
           └──────────────┬─────────────┘
                          │
                         ▼
              ┌──────────────────────┐
              │   上下文构建 (Context) │
              └──────────┬───────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │   LLM 答案生成         │
              │   (Zhipu GLM-4)      │
              └──────────┬───────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │      最终答案           │
              └──────────────────────┘
```

**核心特性：**
- **默认混合检索**：结合向量语义理解和BM25关键词精确匹配
- **智能路由**：关系查询自动切换到图检索
- **RRF融合**：自动平衡不同检索源的结果

### 模块职责

| 模块 | 职责 | 文件位置 |
|------|------|----------|
| **配置管理** | 加载 YAML 配置，环境变量替换 | `internal/config/` |
| **查询路由** | 分析查询，选择检索策略 | `internal/core/router/` |
| **向量检索** | Milvus 语义搜索 | `internal/core/retrieval/vector.go` |
| **BM25 检索** | 倒排索引全文搜索 | `internal/core/retrieval/bm25.go` |
| **图检索** | Neo4j 多跳关系查询 | `internal/core/retrieval/graph.go` |
| **混合检索** | RRF 算法融合多种结果 | `internal/core/retrieval/hybrid.go` |
| **知识图谱** | 实体提取、图谱构建 | `internal/kg/` |
| **LLM 生成** | 智谱 AI 对话生成 | `pkg/ml/llm/` |
| **Embedding** | 文本向量化 | `pkg/ml/embedding/` |
| **存储层** | Milvus/Neo4j/Redis | `pkg/storage/` |

---

## 技术栈

### 后端框架

| 组件 | 技术 | 版本 |
|------|------|------|
| 语言 | Go | 1.21+ |
| Web 框架 | 标准库 `net/http` | - |
| 配置管理 | Viper | - |

### AI/ML

| 组件 | 技术 | 说明 |
|------|------|------|
| AI 框架 | [CloudWeGo Eino](https://github.com/cloudwego/eino) | 字节跳动开源的 LLM 应用开发框架 |
| Embedding | Zhipu AI `embedding-2` | 1024 维向量，通过 eino OpenAI 兼容接口调用 |
| LLM | Zhipu AI `glm-4-flash` | 完全免费，通过 eino OpenAI 兼容接口调用 |
| 中文分词 | jieba-go | `github.com/yanyiwu/gojieba` |

### 数据库

| 数据库 | 用途 | 连接方式 |
|--------|------|----------|
| **Milvus** | 向量数据库 | `localhost:19530` |
| **Neo4j** | 图数据库 | `bolt://localhost:7687` |
| **Redis** | 缓存 | `localhost:6379` |

### 依赖库

```bash
# 核心依赖
github.com/charmbracelet/log           # 日志
github.com/spf13/viper                # 配置管理
github.com/neo4j/neo4j-go-driver/v5  # Neo4j 客户端
github.com/milvus-io/milvus-sdk-go/v2 # Milvus 客户端
github.com/redis/go-redis/v9          # Redis 客户端
github.com/yanyiwu/gojieba            # jieba 分词

# AI 框架 (CloudWeGo Eino)
github.com/cloudwego/eino                          # LLM 应用开发框架核心
github.com/cloudwego/eino-ext                       # Eino 扩展组件
github.com/cloudwego/eino-ext/components/model/openai      # OpenAI 兼容 ChatModel
github.com/cloudwego/eino-ext/components/embedding/openai # OpenAI 兼容 Embedding
```

---

## 目录结构

```
cookrag-go/
├── cmd/                          # 命令行工具
│   ├── demo/                     # 主演示程序
│   │   └── main.go               # 完整 RAG 流程演示
│   ├── build-graph/              # 知识图谱构建工具
│   │   └── main.go
│   ├── test-graph/               # 图检索测试工具
│   │   └── main.go
│   └── server/                   # HTTP API 服务器
│       └── main.go
│
├── internal/                     # 内部包（不对外暴露）
│   ├── api/                      # HTTP API
│   │   ├── server/               # 服务器实现
│   │   │   └── server.go
│   │   └── handlers/             # 请求处理器
│   │       └── query.go
│   ├── config/                   # 配置管理
│   │   └── config.go             # 配置加载和解析
│   ├── core/                     # 核心业务逻辑
│   │   ├── retrieval/            # 检索模块
│   │   │   ├── vector.go        # 向量检索
│   │   │   ├── bm25.go          # BM25 全文检索
│   │   │   ├── graph.go         # 知识图谱检索
│   │   │   └── hybrid.go        # 混合检索
│   │   └── router/               # 智能查询路由
│   │       └── router.go         # 路由逻辑
│   ├── kg/                       # 知识图谱
│   │   ├── extractor.go          # 实体提取器
│   │   └── builder.go            # 图谱构建器
│   ├── models/                   # 数据模型
│   │   └── document.go           # 文档、检索结果模型
│   └── observability/            # 可观测性
│       ├── metrics.go            # Prometheus 指标
│       └── tracing.go           # 链路追踪
│
├── pkg/                          # 公共库（可对外使用）
│   ├── ml/                       # 机器学习模块
│   │   ├── embedding/            # Embedding 服务
│   │   │   ├── provider.go       # 提供者接口
│   │   │   └── zhipu.go          # 智谱 AI 实现
│   │   └── llm/                  # LLM 服务
│   │       ├── provider.go       # 提供者接口
│   │       └── zhipu.go          # 智谱 AI 实现
│   ├── storage/                  # 存储客户端
│   │   ├── cache/                # Redis 缓存
│   │   │   └── redis.go
│   │   ├── milvus/               # Milvus 向量数据库
│   │   │   └── client.go
│   │   └── neo4j/                # Neo4j 图数据库
│   │       └── client.go         # 包含 CreateNode, CreateRelation 等
│   └── monitoring/               # 监控
│       └── metrics.go            # Prometheus 指标收集
│
├── config/                       # 配置文件
│   └── config.yaml               # 主配置文件
│
├── docs/dishes/                 # 菜谱数据（342 份 Markdown 文件）
│   ├── meat_dish/                # 肉菜
│   ├── vegetable_dish/           # 素菜
│   ├── soup/                     # 汤羹
│   ├── aquatic/                  # 水产
│   └── ...
│
├── deployments/                 # 部署配置
│   └── docker/                   # Docker Compose
│       └── docker-compose.yml
│
├── .env.example                  # 环境变量示例
├── go.mod                        # Go 模块依赖
├── go.sum                        # 依赖校验和
├── Makefile                      # 构建脚本
├── run.sh                        # 快速启动脚本
└── README.md                     # 项目说明
```

---

## 核心模块详解

### 1. 智能查询路由 (`router.go`)

**职责**: 根据查询的复杂度和关系强度，自动选择最优的检索策略。

#### 路由决策流程

```
查询输入
    │
    ▼
┌─────────────────┐
│ 1. 计算复杂度    │ → 查询长度、关键词数、特殊字符
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 2. 检测关系强度  │ → 关系词、实体数量、层级关系
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 3. 计算置信度    │ → 基于复杂度和关系强度
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 4. 推荐策略      │
│                 │
│ if 关系强度 > 0.6:        │
│     → 图检索 (Graph)      │
│ else:                     │
│     → 混合检索 (Hybrid)    │ ← 默认策略
│         ├── 向量语义       │
│         └── BM25关键词     │
└──────────────────────────┘
```

**混合检索优势：**
- ✅ **向量检索**：理解语义，能找到相关但不完全相同的内容
- ✅ **BM25检索**：精确关键词匹配，擅长专有名词、ID号
- ✅ **RRF融合**：自动平衡两种检索结果，提供最佳召回率和精确度

#### 关键函数

```go
// Route 智能路由主函数
func (r *QueryRouter) Route(ctx context.Context, query string) (*models.RetrievalResult, error)

// detectRelationshipIntensity 检测关系强度
// 返回值 0-1，越高表示越需要图检索
func (r *QueryRouter) detectRelationshipIntensity(query string) float64

// calculateComplexity 计算查询复杂度
// 返回值 0-1，越高表示查询越复杂
func (r *QueryRouter) calculateComplexity(query string) float64
```

#### 菜谱场景特定规则

```go
// 菜谱关系词（新增）
recipeRelationWords := []string{
    "食材", "配料", "主料", "辅料", "代替", "替代", "替换",
    "用...做", "还有什么", "类似",
    "菜系", "属于什么菜", "分类", "类型",
    "还能", "也可以", "其他的", "相关的",
    "和", "搭配", "一起", "含有", "包含",
}

// 菜谱特定模式
if regexp.MustCompile(`用.+做.*菜`).MatchString(query) {
    intensity += 0.4  // "用A做B" → 图检索
}
if regexp.MustCompile(`.+和.+能.*做`).MatchString(query) {
    intensity += 0.4  // "A和B能做什么" → 图检索
}
```

### 2. 向量检索 (`vector.go`)

**职责**: 使用 Milvus 进行语义相似度搜索。

#### 工作流程

```
查询文本
    │
    ▼
┌─────────────────┐
│ Embedding 向量化  │ → 调用 Zhipu embedding-2 API
└────────┬────────┘
         │
         ▼ 1024 维向量
┌─────────────────┐
│ Milvus 相似度搜索│ → IVF_FLAT 索引，L2 距离
└────────┬────────┘
         │
         ▼ Top-K 结果
┌─────────────────┐
│  Redis 缓存检查  │ → 相同查询直接返回
└────────┬────────┘
         │
         ▼
    返回文档列表
```

#### 关键代码

```go
type VectorRetriever struct {
    config           *VectorRetrieverConfig
    embeddingProvider embedding.Provider
    milvusClient     *milvus.Client
    redisClient      *cache.Client
}

// Retrieve 执行向量检索
func (r *VectorRetriever) Retrieve(ctx context.Context, query string) (*models.RetrievalResult, error)
```

### 3. BM25 全文检索 (`bm25.go`)

**职责**: 基于倒排索引的关键词匹配搜索。

#### BM25 算法

```
Score(D, Q) = Σ IDF(qi) × (f(qi, D) × (k1 + 1)) / (f(qi, D) + k1 × (1 - b + b × |D| / avgdl))

其中:
- f(qi, D): 词项 qi 在文档 D 中的频率
- |D|: 文档 D 的长度
- avgdl: 平均文档长度
- k1: 词频饱和参数 (默认 1.5)
- b: 长度惩罚参数 (默认 0.75)
- IDF(qi): 逆文档频率
```

#### 中文分词集成

```go
type BM25Retriever struct {
    config    *BM25Config
    index     *InvertedIndex
    tokenizer *gojieba.Jieba  // jieba 分词器
}

// Tokenize 使用 jieba 进行中文分词
func (r *BM25Retriever) Tokenize(text string) []string {
    words := r.tokenizer.Cut(text, true)  // HMM=true 搜索模式

    // 停用词过滤
    stopWords := map[string]bool{
        "的": true, "了": true, "在": true, "是": true,
        "之": true, "与": true, "及": true, "等": true,
        // ...
    }

    filtered := make([]string, 0)
    for _, word := range words {
        if !stopWords[word] && len(word) > 1 && !isPunctuation(word) {
            filtered = append(filtered, word)
        }
    }
    return filtered
}
```

### 4. 知识图谱检索 (`graph.go`)

**职责**: 基于 Neo4j 的多跳关系查询。

#### 图谱模式

```
(菜品Dish) ──[包含]──> (食材Ingredient)
    │
    ├──[属于]──> (分类Category)
    ├──[菜系]──> (菜系Cuisine)
    ├──[难度]──> (难度Difficulty)
    └──[使用]──> (工具Tool)
```

#### 多跳查询

```go
// MultiHopSearch 多跳搜索
// entities: 提取的实体列表（食材、菜品名）
// maxDepth: 最大跳数（默认 2）
func (c *Client) MultiHopSearch(ctx context.Context, entities []string, maxDepth int) (*Subgraph, error)

// Cypher 查询示例
MATCH path = (start)-[*1..2]-(related)
WHERE start.name IN $entities
RETURN elementId(start), start.name, labels(start),
       elementId(related), related.name, labels(related),
       type(last(relationships(path))) AS relation_type
```

#### 实体提取

```go
// 使用 jieba 分词 + Neo4j 查询
func (c *Client) ExtractEntities(ctx context.Context, query string) ([]string, error) {
    jieba := gojieba.NewJieba()
    defer jieba.Free()
    words := jieba.CutForSearch(query, true)

    // 过滤停用词，提取候选实体
    queryParts := filterStopWords(words)

    // 在 Neo4j 中查找匹配的节点
    cypher := `
        MATCH (entity:Ingredient) WHERE entity.name IN $queryParts
        RETURN DISTINCT entity.name
    `
}
```

### 5. 知识图谱构建 (`internal/kg/`)

#### 实体提取器 (`extractor.go`)

**功能**: 从 Markdown 菜谱中自动提取实体和关系。

```go
// ExtractFromRecipe 从菜谱提取实体和关系
func (e *RecipeExtractor) ExtractFromRecipe(content, category, dishName string) *ExtractedData

// 提取的实体类型
type EntityType string
const (
    EntityDish        EntityType = "Dish"        // 菜品
    EntityIngredient  EntityType = "Ingredient"  // 食材
    EntityCategory    EntityType = "Category"    // 分类
    EntityCuisine     EntityType = "Cuisine"     // 菜系
    EntityDifficulty  EntityType = "Difficulty"  // 难度
    EntityTool        EntityType = "Tool"        // 工具
)

// 提取的关系类型
type RelationType string
const (
    RelationContains    RelationType = "包含"     // Dish -> Ingredient
    RelationBelongsTo   RelationType = "属于"     // Dish -> Category
    RelationCuisine     RelationType = "菜系"     // Dish -> Cuisine
    RelationDifficulty  RelationType = "难度"     // Dish -> Difficulty
    RelationUsesTool    RelationType = "使用"     // Dish -> Tool
)
```

#### 图谱构建器 (`builder.go`)

**功能**: 将提取的实体和关系导入 Neo4j。

```go
// BuildFromDocuments 从文档构建知识图谱
func (b *GraphBuilder) BuildFromDocuments(ctx context.Context, documents []Document) (*BuildStats, error)

// 构建统计
type BuildStats struct {
    TotalDishes      int    // 菜品数量
    TotalIngredients  int    // 食材数量
    TotalCategories   int    // 分类数量
    TotalRelations    int    // 关系数量
    BuildDuration     time.Duration
}
```

#### Neo4j 索引优化

在构建图谱前，系统会自动创建索引以加速查询：

```go
// createIndexes 创建索引
// Neo4j 索引用途：加速节点属性查询（类似 MySQL 索引）
// 例如：MATCH (n:Dish {name: '红烧肉'}) 会直接通过索引定位，而不是扫描所有节点
func (b *GraphBuilder) createIndexes(ctx context.Context)
```

**创建的索引**：

| 标签 | 属性 | 用途 |
|------|------|------|
| `Dish` | `name` | 加速按菜名查询 |
| `Ingredient` | `name` | 加速按食材查询 |
| `Category` | `name` | 加速按分类查询 |
| `Cuisine` | `name` | 加速按菜系查询 |
| `Difficulty` | `name` | 加速按难度查询 |

**Neo4j 索引语法**：
```cypher
-- Neo4j 5.x 语法（系统使用的）
CREATE INDEX IF NOT EXISTS FOR (n:Dish) ON (n.name)

-- 查询性能对比
-- 无索引：扫描所有节点（全节点扫描）O(N)
-- 有索引：直接定位到目标节点 O(log N)
```

#### 使用方式

```bash
# 构建知识图谱
go run cmd/build-graph/main.go
```

**输出**:
```
🕸️  CookRAG Knowledge Graph Builder
✅ Loaded 342 documents
🕸️  Starting knowledge graph construction...
🔨 Creating 911 unique entities...
🔗 Creating 3449 relations...
✅ Knowledge graph built successfully!

📊 Stats:
   Dishes:      341
   Ingredients: 537
   Categories:  11
   Relations:   3449
```

### 6. LLM 生成 (`pkg/ml/llm/zhipu.go`)

**职责**: 通过 eino 框架调用智谱 AI GLM-4-flash 模型生成答案。

#### 技术实现

使用 **CloudWeGo Eino** 框架的 OpenAI 兼容接口：

```go
// 1. 创建 ChatModel（通过 eino 框架）
chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    APIKey:     apiKey,
    BaseURL:    "https://open.bigmodel.cn/api/paas/v4",  // 智谱 OpenAI 兼容接口
    Model:      "glm-4-flash",
    ByAzure:    false,
})

// 2. 构造 eino 标准消息格式
messages := []*schema.Message{
    schema.UserMessage(prompt),
}

// 3. 调用生成
response, err := chatModel.Generate(ctx, messages)
```

    // 3. 提取内容
    return resp.Content, nil
}
```

### 7. 可观测性 (`internal/observability/`)

#### 7.1 链路追踪 (`tracing.go`)

**职责**: 对整个 RAG 流程进行分布式链路追踪，记录每个操作的耗时和元数据。

**作用**:
- **性能分析**: 追踪每个操作的耗时，定位性能瓶颈
- **错误定位**: 快速定位是哪个检索器或 LLM 调用失败
- **调用链理解**: 查看完整的请求链路：路由 → 检索 → LLM 生成
- **参数调试**: 记录查询参数、权重配置、结果数量等

**已集成的模块**:

| 模块 | Span 名称 | 追踪的元数据 |
|------|-----------|--------------|
| **QueryRouter** | `query_route` | complexity, relationship_intensity, recommended_strategy, result_count, latency_ms |
| **VectorRetriever** | `vector_retrieve` + 子 span | top_k, cache_hit, result_count, latency_ms |
| | `embedding_api` (子 span) | duration_ms |
| | `milvus_search` (子 span) | duration_ms |
| **BM25Retriever** | `bm25_retrieve` | query, top_k, term_count, result_count, latency_ms |
| **GraphRetriever** | `graph_retrieve` | query, max_depth, entity_count, node_count, relation_count, result_count, latency_ms |
| **HybridRetriever** | `hybrid_retrieve` | query, vector_weight, bm25_weight, top_k, rrf_k, result_count, vector_result_count, bm25_result_count, latency_ms |
| **LLM Generator** | `llm_generate_answer` | query, doc_count, provider, latency_ms, answer_length, prompt_length |
| **Zhipu LLM** | `zhipu_llm_generate` | model, prompt_length, latency_ms, response_length |
| **Zhipu LLM Stream** | `zhipu_llm_stream` | model, prompt_length, chunk_count, total_length |

**使用方式**:

```go
// 创建链路追踪 span
span := observability.GlobalTracer.StartSpan(ctx, "operation_name", map[string]interface{}{
    "query": query,
    "top_k": topK,
})
defer span.End()

// 添加元数据
span.AddMetadata("result_count", len(results))
span.AddMetadata("latency_ms", float64(latency))

// 错误处理
if err != nil {
    span.SetError(err)
    return err
}
```

**子 Span**:

对于复杂的操作，可以创建子 span 进行更细粒度的追踪：

```go
// 主 span
span := observability.GlobalTracer.StartSpan(ctx, "vector_retrieve", ...)

// 子 span (如: 调用 Embedding API)
embeddingSpan := observability.GlobalTracer.StartSpan(ctx, "embedding_api", ...)
// ... 执行操作 ...
embeddingSpan.End()

// 继续主操作
// ...
span.End()
```

#### 7.2 监控指标 (`metrics.go`)

**职责**: 通过 Prometheus 收集系统运行指标。

**指标类型**:

```go
// 计数器 (Counter)
metrics.QueryCounter.Inc()

// 直方图 (Histogram)
metrics.QueryLatency.Observe(duration)

// 仪表盘 (Gauge)
metrics.ActiveQueries.Inc()
defer metrics.ActiveQueries.Dec()
```

**导出的指标**:

| 指标名称 | 类型 | 描述 |
|---------|------|------|
| `rag_queries_total` | Counter | 总查询次数 |
| `rag_query_latency_ms` | Histogram | 查询延迟分布 |
| `rag_active_queries` | Gauge | 当前活跃查询数 |
| `rag_retrieval_errors_total` | Counter | 检索错误次数 |
| `rag_llm_generation_duration_ms` | Histogram | LLM 生成耗时 |

---

## 数据流程

### 完整的 RAG 流程

```
用户查询: "红烧肉怎么做？"
    │
    ├─> [查询路由] 分析: complexity=0.10, entities=0.2, strategy=hybrid
    │
    ├─> [混合检索 - Hybrid]
    │   ├─> [BM25检索]
    │   │   ├─> jieba分词: ["红烧肉", "怎么", "做"]
    │   │   └─> 倒排索引匹配
    │   │
    │   ├─> [向量检索]
    │   │   ├─> Embedding: "红烧肉怎么做？" → [0.23, -0.45, ..., 0.67] (1024维)
    │   │   ├─> Milvus 搜索: top_k=10, metric=L2
    │   │   └─> 返回 10 个相关文档
    │   │
    │   └─> [RRF融合]
    │       ├─> 向量权重: 70%, BM25权重: 30%
    │       ├─> 自适应权重: 根据查询复杂度动态调整
    │       └─> 返回融合后的 Top-10 文档
    │
    ├─> [上下文构建]
    │   └─> 格式化检索结果为 LLM prompt
    │
    ├─> [LLM 生成]
    │   ├─> 模型: glm-4-flash
    │   ├─> 输入: prompt + 检索上下文
    │   └─> 输出: 详细菜谱步骤
    │
    └─> [返回答案]
        └─> "红烧肉的做法如下：..."
```

### 知识图谱检索流程

```
用户查询: "用鸡蛋能做哪些菜？"
    │
    ├─> [查询路由] 分析: complexity=0.11, entities=0.6, strategy=graph
    │
    ├─> [实体提取]
    │   ├─> jieba 分词: ["用", "鸡蛋", "能", "做", "哪些", "菜"]
    │   ├─> 停用词过滤: ["鸡蛋"]
    │   └─> Neo4j 匹配: 找到 8 个相关实体
    │
    ├─> [多跳搜索]
    │   ├─> Cypher: MATCH (鸡蛋)-[*1..2]-(related)
    │   ├─> 结果: 200 nodes, 100 relations
    │   └─> 返回相关菜品节点
    │
    └─> [返回结果]
        └─> 返回 10 个包含鸡蛋的菜品
```

---

## 配置说明

### 环境变量

创建 `.env` 文件：

```bash
# 复制示例配置
cp .env.example .env

# 编辑 .env，填入你的 API 密钥
export ZHIPU_API_KEY="your_api_key_here"

# Neo4j 配置
export NEO4J_USERNAME="neo4j"
export NEO4J_PASSWORD="your_password"

# Redis 配置（如果没有密码，留空）
export REDIS_PASSWORD=""
```

### 配置文件 (`config/config.yaml`)

```yaml
# Embedding配置
embedding:
  provider: "zhipu"
  model: "embedding-2"
  api_key: "${ZHIPU_API_KEY}"
  timeout: 30

# Milvus向量数据库
milvus:
  host: "localhost"
  port: "19530"
  dimension: 1024
  index_type: "IVF_FLAT"
  metric_type: "L2"

# Neo4j图数据库
neo4j:
  uri: "bolt://localhost:7687"
  database: "neo4j"

# Redis缓存
redis:
  host: "localhost"
  port: "6379"

# LLM配置
llm:
  provider: "zhipu"
  model: "glm-4"
  api_key: "${ZHIPU_API_KEY}"
  temperature: 0.1
  max_tokens: 2048
```

---

## 开发指南

### 快速开始

```bash
# 1. 克隆项目
git clone <repo_url>
cd cookrag-go

# 2. 安装依赖
go mod tidy

# 3. 配置环境变量
cp .env.example .env
# 编辑 .env 文件

# 4. 启动依赖服务
cd deployments/docker
docker-compose up -d

# 5. 运行演示
bash run.sh
```

### 构建知识图谱

```bash
go run cmd/build-graph/main.go
```

### 运行测试

```bash
# 测试向量检索
go run cmd/demo/main.go

# 测试图检索
go run cmd/test-graph/main.go
```

### 添加新的检索策略

1. 在 `internal/core/retrieval/` 创建新文件
2. 实现 `Retrieve` 方法

```go
package retrieval

type MyRetriever struct {
    config *MyConfig
}

func (r *MyRetriever) Retrieve(ctx context.Context, query string) (*models.RetrievalResult, error) {
    // 实现检索逻辑
    return &models.RetrievalResult{
        Documents: docs,
        Strategy:  "my_strategy",
        Query:     query,
    }, nil
}
```

3. 在 `router.go` 中添加路由规则

---

## 部署说明

### Docker 部署

```bash
cd deployments/docker
docker-compose up -d
```

**服务清单**:
- Milvus: 向量数据库
- Neo4j: 图数据库
- Redis: 缓存
- Etcd: Milvus 配置存储
- Minio: Milvus 对象存储

### 启动 API 服务

```bash
go run cmd/server/main.go
```

API 默认运行在 `http://localhost:8080`

---

## API 接口

### POST /api/query

查询接口，返回检索结果和 LLM 生成的答案。

**请求**:
```json
{
  "query": "红烧肉怎么做？",
  "top_k": 10,
  "use_llm": true
}
```

**响应**:
```json
{
  "strategy": "hybrid",
  "documents": [
    {
      "id": "doc_123",
      "content": "# 湖南家常红烧肉的做法...",
      "score": 0.8523,
      "metadata": {
        "file": "meat_dish/红烧肉.md",
        "category": "meat_dish",
        "dish": "红烧肉"
      }
    }
  ],
  "answer": "红烧肉的做法如下：...",
  "latency": 125
}
```

### GET /api/health

健康检查接口。

**响应**:
```json
{
  "status": "healthy",
  "components": {
    "milvus": "connected",
    "neo4j": "connected",
    "redis": "connected",
    "llm": "connected"
  }
}
```

---

## 常见问题

### Q: 如何切换 Embedding 提供商？

修改 `config/config.yaml`:

```yaml
embedding:
  provider: "zhipu"  # 目前只支持 zhipu
```

### Q: 如何调整 BM25 参数？

在 `internal/core/retrieval/bm25.go`:

```go
config := &BM25Config{
    K1: 1.5,  // 词频饱和参数
    B:  0.75, // 长度惩罚参数
}
```

### Q: 知识图谱数据在哪里？

图谱数据存储在 Neo4j 中，使用 `cmd/build-graph` 构建：

```bash
go run cmd/build-graph/main.go
```

数据来源：`docs/dishes/` 目录下的 342 份 Markdown 文件。

### Q: 如何添加新菜谱？

1. 在 `docs/dishes/` 对应的分类目录下创建 `.md` 文件
2. 遵循现有文件格式：
   ```markdown
   # 菜名

   简介

   预估烹饪难度：★★★

   ## 必备原料和工具
   - 食材1
   - 食材2

   ## 计算
   - 食材1 100 g

   ## 操作
   1. 步骤1
   2. 步骤2
   ```
3. 重新运行 demo 或构建图谱

### Q: 检索结果分数为 0 是怎么回事？

**症状**: 混合检索返回的文档 `Score` 为 0.0000

**原因**: 有多种可能原因：

1. **Milvus 集合未正确加载**
   - 插入数据后需要 `Flush` 然后 `LoadCollection`
   - 否则数据在磁盘上，搜索不到

2. **Redis 缓存中保存了旧数据**
   - 清空 Redis: `redis-cli FLUSHALL`

3. **Milvus 文档 ID 冲突**
   - 如果使用简单索引（0, 1, 2...）可能导致 ID 重复
   - 解决方案：使用唯一 ID（时间戳 + 索引）

**解决方案**:

1. **清空并重建 Milvus 集合**:
   ```bash
   go run scripts/cleanup-milvus.go
   ```

2. **清空 Redis 缓存**:
   ```bash
   redis-cli FLUSHALL
   ```

3. **重新运行 demo**:
   ```bash
   source .env && go run cmd/demo/main.go
   ```

**验证修复**: 检查日志中的 DEBUG 输出
```
🐛 DEBUG vector[0]: ID=doc_1769174444563, rrf_score=0.2951  ← 分数 > 0 即正常
```

### Q: RRF 分数为什么只有 0.295？不是应该是 0.7 吗？

**原因**: 系统使用了**自适应权重**（AdaptiveRetrieval），根据查询复杂度动态调整：

| 查询复杂度 | vector 权重 | bm25 权重 | 最高分数 |
|-----------|------------|----------|---------|
| 简单 (< 0.3) | 0.30 | 0.70 | ~0.295 |
| 中等 (0.3-0.7) | 0.70 | 0.30 | ~0.688 |
| 复杂 (> 0.7) | 0.80 | 0.20 | ~0.784 |

**如果想要固定权重**（不自适应），修改 `router.go` 的 `recommendStrategy` 函数：

```go
func (r *QueryRouter) recommendStrategy(analysis *models.QueryAnalysis) string {
    if r.config.EnableGraphRAG && analysis.RelationshipIntensity > 0.6 {
        return "graph"
    }

    // 直接使用固定权重的 Hybrid，不使用 Adaptive
    if r.config.EnableHybrid {
        return "hybrid"  // vector=0.7, bm25=0.3
    }

    return "vector"
}
```

---

## 性能优化建议

### 1. 向量检索优化

```go
// 调整 Milvus 搜索参数
searchParams := entity.NewIndexIvfFlatSearchParam(10)  // nlist 参数
```

### 2. Redis 缓存

```go
// 相同查询直接返回缓存
if cached, found := r.redisClient.Get(ctx, cacheKey); found {
    return cached, nil
}
```

### 3. 批量 Embedding

```go
// 批量处理提高效率
embeddings, err := r.embeddingProvider.EmbedBatch(ctx, texts)
```

---

## 总结

CookRAG-Go 是一个功能完整的企业级 RAG 系统，具有以下优势：

1. **混合检索架构**: 采用业界领先的向量+BM25混合检索，RRF算法融合结果
2. **智能路由**: 关系查询自动切换图检索，其他使用混合检索
3. **中文优化**: jieba 分词、停用词过滤、标点符号处理
4. **现代RAG最佳实践**: 符合GraphRAG、Elasticsearch、Milvus等顶尖系统标准
5. **可扩展性**: 模块化设计，易于添加新功能
6. **生产就绪**: 完善的错误处理、日志、监控

### 混合检索优势详解

**为什么需要混合检索？**

| 检索方式 | 擅长 | 不擅长 |
|---------|------|--------|
| **向量检索** | 语义理解、模糊匹配、相似概念 | 专有名词、产品ID、罕见词 |
| **BM25检索** | 精确关键词匹配、ID号、低频词 | 语义相似性、同义词 |

**RRF融合算法：**

```
混合得分 = 向量权重 × (K / (K + 向量排名)) + BM25权重 × (K / (K + BM25排名))
```

其中 K=60，默认权重：向量70%、BM25 30%，并根据查询复杂度自适应调整。

适用于需要复杂检索和知识关联的场景，如菜谱推荐、技术文档问答、产品知识库等。
