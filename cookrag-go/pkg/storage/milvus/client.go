package milvus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// Client Milvus客户端封装
type Client struct {
	client  client.Client
	timeout time.Duration
}

// SearchResult 搜索结果
type SearchResult struct {
	ID     int64                  `json:"id"`
	Score  float32                `json:"score"`
	Fields map[string]interface{} `json:"fields"`
}

// NewClient 创建Milvus客户端
func NewClient(host, port string) (*Client, error) {
	addr := fmt.Sprintf("%s:%s", host, port)

	c, err := client.NewGrpcClient(context.Background(), addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Milvus: %w", err)
	}

	log.Printf("✅ Connected to Milvus: %s", addr)

	return &Client{
		client:  c,
		timeout: 30 * time.Second,
	}, nil
}

// Close 关闭连接
func (c *Client) Close(ctx context.Context) error {
	return c.client.Close()
}

// HasCollection 检查集合是否存在
func (c *Client) HasCollection(ctx context.Context, collectionName string) (bool, error) {
	collections, err := c.client.ListCollections(ctx)
	if err != nil {
		return false, err
	}

	for _, coll := range collections {
		if coll.Name == collectionName {
			return true, nil
		}
	}

	return false, nil
}

// CreateCollection 创建集合
func (c *Client) CreateCollection(ctx context.Context, collectionName string, dimension int) error {
	log.Printf("📦 Creating Milvus collection: %s (dimension: %d)", collectionName, dimension)

	// Milvus Schema 定义（类似 MySQL 的表结构）
	// Collection: 集合，相当于数据库中的表
	// Field: 字段，定义集合中的列
	// DataType: 数据类型（Int64/FloatVector/VarChar/JSON）
	schema := &entity.Schema{
		// 集合基本信息
		CollectionName: collectionName,                // 集合名称（类似表名）
		Description:    "CookRAG document collection", // 集合描述
		AutoID:         false,                         // 不自动生成ID，使用文档ID作为主键

		// 字段定义（类似表结构）
		Fields: []*entity.Field{
			// ========== 主键字段 ==========
			{
				Name:       "id",                  // 字段名：文档ID
				DataType:   entity.FieldTypeInt64, // 数据类型：64位整数
				PrimaryKey: true,                  // 设置为主键（必须唯一）
				AutoID:     false,                 // 不自动生成ID（手动指定）
			},

			// ========== 向量字段 ==========
			// 存储 embedding 向量，用于语义相似度搜索
			{
				Name:     "vector",                    // 字段名：向量数据
				DataType: entity.FieldTypeFloatVector, // 数据类型：浮点数向量
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", dimension), // 向量维度（如1024）
				},
			},

			// ========== 文本字段 ==========
			// 存储原始文档内容（如整个菜谱的 Markdown 文本）
			// 用途：检索后显示给用户、作为上下文传给 LLM
			// 特点：非结构化长文本，人类可读
			// 示例："# 红烧肉的做法\n红烧肉是一道经典的中国菜，主要食材是五花肉..."
			{
				Name:     "text",                  // 字段名：原始文本
				DataType: entity.FieldTypeVarChar, // 数据类型：可变长字符串
				TypeParams: map[string]string{
					"max_length": "65535", // 最大长度：65535字符
				},
			},

			// ========== 元数据字段 ==========
			// 存储文档的结构化属性信息（键值对形式）
			// 用途：按分类筛选、显示菜名、难度等级等
			// 特点：JSON 格式，机器可读的结构化数据
			// 示例：{"file": "meat_dish/红烧肉.md", "category": "肉菜", "dish": "红烧肉", "difficulty": "★★★"}
			{
				Name:     "metadata",           // 字段名：元数据（JSON格式）
				DataType: entity.FieldTypeJSON, // 数据类型：JSON对象
			},
		},
	}

	if err := c.client.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	log.Printf("✅ Collection created: %s", collectionName)
	return nil
}

// CreateIndex 创建索引
func (c *Client) CreateIndex(ctx context.Context, collectionName, fieldName string, idxType string, params map[string]string) error {
	log.Printf("📇 Creating index on %s.%s (type: %s)", collectionName, fieldName, idxType)
	// fieldName 是指定要在哪个字段上创建索引

	// Milvus 索引说明：
	// 索引用于加速向量相似度搜索，没有索引的话就是暴力搜索（FLAT）
	// IVF_FLAT: 基于倒排文件的索引，平衡速度和精度（推荐）
	// HNSW: 基于图的索引，速度更快但内存占用更大
	// L2: 欧几里得距离的平方（最常用）
	// IP: 内积（Inner Product）
	// COSINE: 余弦相似度
	// nlist: 聚类中心点数量，影响检索速度和精度（通常设为 sqrt(数据量)）
	idx, err := entity.NewIndexIvfFlat(
		entity.L2, // 距离度量类型：L2距离（欧几里得距离的平方）
		128,       // nlist参数：聚类中心点数量，影响索引性能
	)
	if err != nil {
		return fmt.Errorf("failed to create index config: %w", err)
	}

	if err := c.client.CreateIndex(ctx, collectionName, fieldName, idx, false); err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	log.Printf("✅ Index created on %s", fieldName)
	return nil
}

// Insert 插入数据
func (c *Client) Insert(ctx context.Context, collectionName string, ids []int64, embeddings [][]float32, texts []string, metadata []map[string]interface{}) error {
	log.Printf("📝 Inserting %d documents into %s", len(ids), collectionName)

	// 准备ID列
	idCol := entity.NewColumnInt64("id", ids)

	// 准备向量列
	vectorCol := entity.NewColumnFloatVector("vector", len(embeddings[0]), embeddings)

	// 准备文本列
	textCol := entity.NewColumnVarChar("text", texts)

	// 准备metadata列 - 转换为JSON字节
	metadataBytes := make([][]byte, len(metadata))
	for i, meta := range metadata {
		metaBytes, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataBytes[i] = metaBytes
	}
	metadataCol := entity.NewColumnJSONBytes("metadata", metadataBytes)

	// 插入数据
	_, err := c.client.Insert(
		ctx,
		collectionName,
		"", // partitionName
		idCol,
		vectorCol,
		textCol,
		metadataCol,
	)

	if err != nil {
		return fmt.Errorf("failed to insert: %w", err)
	}

	log.Printf("✅ Inserted %d documents", len(ids))
	return nil
}

// Flush 刷新数据
func (c *Client) Flush(ctx context.Context, collectionName string) error {
	return c.client.Flush(ctx, collectionName, true)
}

// LoadCollection 加载集合到内存
// Milvus 说明：
// 1. 数据默认存储在磁盘上，搜索前必须先加载到内存
// 2. LoadCollection 把集合的向量数据从磁盘加载到 Milvus 服务器端的内存中
// 3. 参数 false = 只加载到内存（CPU），true = 加载到 GPU 内存（需要 GPU 支持）
// 4. 没有返回值：这是异步操作，只是触发加载过程，实际加载在后台进行
// 5. 必须在搜索前调用，否则搜索会报错或返回空结果
//
// 数据流向：磁盘（持久化存储） → 内存（快速访问） → 搜索时直接读取
//
// 类比：就像看书前要先从书架把书拿到桌子上，才能快速翻阅
func (c *Client) LoadCollection(ctx context.Context, collectionName string) error {
	log.Printf("⏳ Loading collection: %s", collectionName)

	if err := c.client.LoadCollection(ctx, collectionName, false); err != nil {
		return fmt.Errorf("failed to load collection: %w", err)
	}

	log.Printf("✅ Collection loaded: %s", collectionName)
	return nil
}

// Search 向量搜索
func (c *Client) Search(ctx context.Context, collectionName string, vectors [][]float32, vectorField string, outputFields []string, topK int) ([]*SearchResult, error) {
	log.Printf("🔍 Searching in %s (top_k: %d)", collectionName, topK)

	// 准备搜索向量
	vectorsData := make([]entity.Vector, len(vectors))
	for i, vec := range vectors {
		vectorsData[i] = entity.FloatVector(vec)
	}

	// 执行搜索
	// nprobe 参数说明：
	// IVF_FLAT 索引把向量空间分成多个聚类（nlist=128 表示分成 128 个聚类）
	// nprobe 指定搜索时检查多少个聚类（值越大，搜索越精确，但速度越慢）
	//
	// 权衡关系：
	// nprobe = 1   → 最快，精度最低（只搜索 1 个聚类）
	// nprobe = 10  → 平衡（搜索最近的 10 个聚类）
	// nprobe = 128 → 最慢，精度最高（搜索所有聚类，等同于暴力搜索）
	//
	// 经验值：nprobe 通常设为 nlist 的 1/10 到 1/2
	// 这里 nlist=128, nprobe=10，比较合理
	sp, err := entity.NewIndexIvfFlatSearchParam(10) // nprobe: 搜索聚类数量
	if err != nil {
		return nil, fmt.Errorf("failed to create search param: %w", err)
	}

	searchResult, err := c.client.Search(
		ctx,
		collectionName,          // 集合名称
		[]string{},              // partitions: 指定搜索哪些分区（空数组=搜索所有分区）
		                        // 分区示例：[]string{"川菜", "湘菜"} 只搜索这些分区
		                        // 常见用法：[]string{} 搜索全部
		"",                      // expr: 标量过滤表达式（类似 SQL 的 WHERE 子句）
		                        // 示例："metadata[\"difficulty\"] == \"简单\"" 只查简单菜谱
		                        // 示例："metadata[\"category\"] == \"川菜\"" 只查川菜
		                        // 常见用法："" 不过滤，搜索全部数据
		outputFields,             // 输出哪些字段（如 ["text", "metadata"]）
		vectorsData,              // 搜索向量（用户查询的 embedding）
		vectorField,              // 在哪个字段上搜索（通常是 "vector"）
		entity.L2,                // metric type: 距离度量类型（L2/IP/COSINE）
		topK,                     // 返回最相似的 K 个结果
		sp,                       // search param: 搜索参数（如 nprobe=10）
	)

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 解析结果
	results := make([]*SearchResult, 0)
	for _, res := range searchResult {
		for i := 0; i < res.ResultCount; i++ {
			// 获取ID
			idField := res.IDs.(*entity.ColumnInt64)
			if idField == nil || i >= idField.Len() {
				continue
			}
			id := idField.Data()[i]

			// 获取分数
			if res.Scores == nil || i >= len(res.Scores) {
				continue
			}
			score := res.Scores[i]

			fields := make(map[string]interface{})

			// 提取字段数据
			for _, field := range outputFields {
				col := res.Fields.GetColumn(field)
				if col == nil {
					continue
				}

				switch field {
				case "text":
					if textData, ok := col.(*entity.ColumnVarChar); ok && textData != nil && i < textData.Len() {
						fields[field] = textData.Data()[i]
					}
				case "metadata":
					// JSON字段作为字节返回
					if jsonData, ok := col.(*entity.ColumnJSONBytes); ok && jsonData != nil && i < jsonData.Len() {
						var metadata map[string]interface{}
						json.Unmarshal(jsonData.Data()[i], &metadata)
						fields[field] = metadata
					}
				}
			}

			results = append(results, &SearchResult{
				ID:     id,
				Score:  score,
				Fields: fields,
			})
		}
	}

	log.Printf("✅ Search completed: %d results", len(results))
	return results, nil
}

// GetCollectionStats 获取集合统计信息
func (c *Client) GetCollectionStats(ctx context.Context, collectionName string) (map[string]interface{}, error) {
	stats, err := c.client.GetCollectionStatistics(ctx, collectionName)
	if err != nil {
		return nil, err
	}

	// 转换为map[string]interface{}
	result := make(map[string]interface{})
	for k, v := range stats {
		result[k] = v
	}
	return result, nil
}

// DeleteCollection 删除集合
func (c *Client) DeleteCollection(ctx context.Context, collectionName string) error {
	log.Printf("🗑️  Deleting collection: %s", collectionName)

	if err := c.client.DropCollection(ctx, collectionName); err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}

	log.Printf("✅ Collection deleted: %s", collectionName)
	return nil
}
