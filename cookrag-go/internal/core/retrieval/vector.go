package retrieval

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/models"
	"cookrag-go/internal/observability"
	"cookrag-go/pkg/ml/embedding"
	"cookrag-go/pkg/storage/cache"
	"cookrag-go/pkg/storage/milvus"
)

// VectorRetrieverConfig 向量检索配置
type VectorRetrieverConfig struct {
	CollectionName string // Milvus集合名称
	VectorField    string // 向量字段名
	TextField      string // 文本字段名
	MetadataField  string // 元数据字段名
	TopK           int    // 返回结果数量
	UseCache       bool   // 是否使用缓存
	CacheTTL       time.Duration // 缓存过期时间
}

// DefaultVectorRetrieverConfig 默认配置
func DefaultVectorRetrieverConfig() *VectorRetrieverConfig {
	return &VectorRetrieverConfig{
		CollectionName: "cookrag_documents",
		VectorField:    "vector",
		TextField:      "text",
		MetadataField:  "metadata",
		TopK:           10,
		UseCache:       true,
		CacheTTL:       5 * time.Minute,
	}
}

// VectorRetriever 向量检索器
type VectorRetriever struct {
	config          *VectorRetrieverConfig
	embeddingProvider embedding.Provider
	milvusClient    *milvus.Client
	cache           cache.Cache
}

// NewVectorRetriever 创建向量检索器
func NewVectorRetriever(
	config *VectorRetrieverConfig,
	embeddingProvider embedding.Provider,
	milvusClient *milvus.Client,
	cacheClient cache.Cache,
) *VectorRetriever {
	if config == nil {
		config = DefaultVectorRetrieverConfig()
	}

	return &VectorRetriever{
		config:          config,
		embeddingProvider: embeddingProvider,
		milvusClient:    milvusClient,
		cache:           cacheClient,
	}
}

// Retrieve 向量检索
func (r *VectorRetriever) Retrieve(ctx context.Context, query string) (*models.RetrievalResult, error) {
	// 创建链路追踪 span
	span := observability.GlobalTracer.StartSpan(ctx, "vector_retrieve", map[string]interface{}{
		"query": query,
		"top_k": r.config.TopK,
	})
	defer span.End()

	startTime := time.Now()

	// 1. 生成查询向量（创建子 span）
	log.Infof("🔤 Embedding query: %s", query)
	embeddingSpan := observability.GlobalTracer.StartSpan(ctx, "embedding_api", map[string]interface{}{
		"query": query,
	})
	embeddingStart := time.Now()
	queryEmbedding, err := r.embeddingProvider.Embed(ctx, query)
	embeddingSpan.AddMetadata("duration_ms", float64(time.Since(embeddingStart).Milliseconds()))
	if err != nil {
		embeddingSpan.SetError(err)
		embeddingSpan.End()
		span.SetError(err)
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}
	embeddingSpan.End()

	// 2. 检查缓存
	if r.config.UseCache && r.cache != nil {
		cacheKey := r.getCacheKey(query)
		var cachedResult models.RetrievalResult
		cacheCheckStart := time.Now()
		if err := r.cache.Get(ctx, cacheKey, &cachedResult); err == nil {
			cacheHit := true
			span.AddMetadata("cache_hit", cacheHit)
			span.AddMetadata("cache_latency_ms", float64(time.Since(cacheCheckStart).Milliseconds()))
			log.Infof("💨 Cache hit for query: %s", query)
			cachedResult.Latency = float64(time.Since(startTime).Milliseconds())
			return &cachedResult, nil
		}
		span.AddMetadata("cache_hit", false)
	}

	// 3. 执行向量搜索（创建子 span）
	log.Infof("🔍 Searching in Milvus collection: %s", r.config.CollectionName)
	searchSpan := observability.GlobalTracer.StartSpan(ctx, "milvus_search", map[string]interface{}{
		"collection": r.config.CollectionName,
		"top_k": r.config.TopK,
	})
	searchStart := time.Now()
	searchResults, err := r.milvusClient.Search(
		ctx,
		r.config.CollectionName,
		[][]float32{queryEmbedding},
		r.config.VectorField,
		[]string{r.config.TextField, r.config.MetadataField},
		r.config.TopK,
	)
	searchSpan.AddMetadata("duration_ms", float64(time.Since(searchStart).Milliseconds()))
	if err != nil {
		searchSpan.SetError(err)
		searchSpan.End()
		span.SetError(err)
		return nil, fmt.Errorf("milvus search failed: %w", err)
	}
	searchSpan.End()

	// 4. 转换结果
	documents := make([]models.Document, 0, len(searchResults))
	for _, result := range searchResults {
		doc := models.Document{
			ID:    fmt.Sprintf("doc_%d", result.ID),
			Score: result.Score,
		}

		// 提取文本和元数据
		if text, ok := result.Fields[r.config.TextField].(string); ok {
			doc.Content = text
		}

		if metadata, ok := result.Fields[r.config.MetadataField].(map[string]interface{}); ok {
			doc.Metadata = metadata
		}

		documents = append(documents, doc)
	}

	result := &models.RetrievalResult{
		Documents: documents,
		Strategy:  "vector",
		Query:     query,
		Latency:   float64(time.Since(startTime).Milliseconds()),
	}

	// 5. 缓存结果
	if r.config.UseCache && r.cache != nil {
		cacheKey := r.getCacheKey(query)
		if err := r.cache.Set(ctx, cacheKey, result, r.config.CacheTTL); err != nil {
			log.Warnf("Failed to cache result: %v", err)
		}
	}

	span.AddMetadata("result_count", len(documents))
	span.AddMetadata("latency_ms", result.Latency)
	log.Infof("✅ Vector retrieval completed: %d results in %.2fms",
		len(documents), result.Latency)

	return result, nil
}

// RetrieveBatch 批量向量检索
func (r *VectorRetriever) RetrieveBatch(ctx context.Context, queries []string) ([]*models.RetrievalResult, error) {
	startTime := time.Now()

	log.Infof("🔤 Batch embedding %d queries", len(queries))

	// 批量生成查询向量
	queryEmbeddings, err := r.embeddingProvider.EmbedBatch(ctx, queries)
	if err != nil {
		return nil, fmt.Errorf("failed to embed queries: %w", err)
	}

	// 批量搜索
	searchResults, err := r.milvusClient.Search(
		ctx,
		r.config.CollectionName,
		queryEmbeddings,
		r.config.VectorField,
		[]string{r.config.TextField, r.config.MetadataField},
		r.config.TopK,
	)

	if err != nil {
		return nil, fmt.Errorf("milvus batch search failed: %w", err)
	}

	// 按查询分组结果
	resultsPerQuery := len(searchResults) / len(queries)
	results := make([]*models.RetrievalResult, 0, len(queries))

	for i := 0; i < len(queries); i++ {
		start := i * resultsPerQuery
		end := start + resultsPerQuery

		documents := make([]models.Document, 0, resultsPerQuery)
		for j := start; j < end; j++ {
			if j >= len(searchResults) {
				break
			}

			result := searchResults[j]
			doc := models.Document{
				ID:    fmt.Sprintf("doc_%d", result.ID),
				Score: result.Score,
			}

			if text, ok := result.Fields[r.config.TextField].(string); ok {
				doc.Content = text
			}

			if metadata, ok := result.Fields[r.config.MetadataField].(map[string]interface{}); ok {
				doc.Metadata = metadata
			}

			documents = append(documents, doc)
		}

		results = append(results, &models.RetrievalResult{
			Documents: documents,
			Strategy:  "vector_batch",
			Query:     queries[i],
			Latency:   float64(time.Since(startTime).Milliseconds()),
		})
	}

	log.Infof("✅ Batch vector retrieval completed: %d queries, avg %.2fms",
		len(results), float64(time.Since(startTime).Milliseconds())/float64(len(queries)))

	return results, nil
}

// IndexDocuments 索引文档
func (r *VectorRetriever) IndexDocuments(ctx context.Context, documents []models.Document) error {
	log.Infof("📝 Indexing %d documents to Milvus", len(documents))

	// 批量生成文档向量
	texts := make([]string, len(documents))
	for i, doc := range documents {
		texts[i] = doc.Content
	}

	embeddings, err := r.embeddingProvider.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to embed documents: %w", err)
	}

	// 准备Milvus数据
	ids := make([]int64, len(documents))
	metadataList := make([]map[string]interface{}, len(documents))

	for i, doc := range documents {
		ids[i] = int64(i)
		metadataList[i] = doc.Metadata
	}

	// 批量插入
	err = r.milvusClient.Insert(
		ctx,
		r.config.CollectionName,
		ids,
		embeddings,
		texts,
		metadataList,
	)

	if err != nil {
		return fmt.Errorf("failed to insert documents: %w", err)
	}

	// 刷新集合
	if err := r.milvusClient.Flush(ctx, r.config.CollectionName); err != nil {
		return fmt.Errorf("failed to flush collection: %w", err)
	}

	log.Infof("✅ Indexed %d documents successfully", len(documents))
	return nil
}

// getCacheKey 生成缓存key
func (r *VectorRetriever) getCacheKey(query string) string {
	return fmt.Sprintf("vector:%s", query)
}

// GetStats 获取检索器统计信息
func (r *VectorRetriever) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats, err := r.milvusClient.GetCollectionStats(ctx, r.config.CollectionName)
	if err != nil {
		return nil, err
	}

	stats["top_k"] = r.config.TopK
	stats["use_cache"] = r.config.UseCache
	stats["collection_name"] = r.config.CollectionName

	return stats, nil
}
