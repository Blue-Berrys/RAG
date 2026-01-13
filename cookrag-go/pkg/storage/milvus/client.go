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

	schema := &entity.Schema{
		CollectionName: collectionName,
		Description:    "CookRAG document collection",
		AutoID:         false,
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeInt64,
				PrimaryKey: true,
				AutoID:     false,
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", dimension),
				},
			},
			{
				Name:     "text",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "65535",
				},
			},
			{
				Name:     "metadata",
				DataType: entity.FieldTypeJSON,
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

	// 创建索引 - 使用IVF_FLAT
	idx, err := entity.NewIndexIvfFlat(
		entity.L2, // metric type
		128,      // nlist
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

// LoadCollection 加载集合
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
	sp, err := entity.NewIndexIvfFlatSearchParam(10) // nprobe
	if err != nil {
		return nil, fmt.Errorf("failed to create search param: %w", err)
	}

	searchResult, err := c.client.Search(
		ctx,
		collectionName,
		[]string{}, // partitions
		"",         // expr
		outputFields,
		vectorsData,
		vectorField,
		entity.L2, // metric type
		topK,
		sp, // search param
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
