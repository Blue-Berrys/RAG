package retrieval

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/models"
)

// HybridRetrieverConfig 混合检索配置
type HybridRetrieverConfig struct {
	VectorWeight  float64 // 向量检索权重 (0-1)
	BM25Weight    float64 // BM25检索权重 (0-1)
	TopK          int     // 返回结果数量
	RRFK          int     // RRF常数 (通常60)
	RRF           int     // RRF常数 (别名,与RRFK相同)
}

// DefaultHybridRetrieverConfig 默认配置
func DefaultHybridRetrieverConfig() *HybridRetrieverConfig {
	return &HybridRetrieverConfig{
		VectorWeight: 0.7,
		BM25Weight:   0.3,
		TopK:         10,
		RRFK:         60,
		RRF:          60,
	}
}

// HybridRetriever 混合检索器
type HybridRetriever struct {
	config         *HybridRetrieverConfig
	vectorRetriever *VectorRetriever
	bm25Retriever   *BM25Retriever
}

// NewHybridRetriever 创建混合检索器
func NewHybridRetriever(
	config *HybridRetrieverConfig,
	vectorRetriever *VectorRetriever,
	bm25Retriever *BM25Retriever,
) *HybridRetriever {
	if config == nil {
		config = DefaultHybridRetrieverConfig()
	}

	return &HybridRetriever{
		config:          config,
		vectorRetriever: vectorRetriever,
		bm25Retriever:   bm25Retriever,
	}
}

// Retrieve 混合检索
func (r *HybridRetriever) Retrieve(ctx context.Context, query string) (*models.RetrievalResult, error) {
	startTime := time.Now()

	log.Infof("🔀 Hybrid retrieval: query='%s', vector_weight=%.2f, bm25_weight=%.2f",
		query, r.config.VectorWeight, r.config.BM25Weight)

	// 并行执行向量检索和BM25检索
	type retrievalResult struct {
		Result *models.RetrievalResult
		Error  error
	}

	vectorResultCh := make(chan retrievalResult, 1)
	bm25ResultCh := make(chan retrievalResult, 1)

	// 向量检索
	go func() {
		result, err := r.vectorRetriever.Retrieve(ctx, query)
		vectorResultCh <- retrievalResult{Result: result, Error: err}
	}()

	// BM25检索
	go func() {
		docs, err := r.bm25Retriever.Retrieve(ctx, query, r.config.TopK * 2)
		result := &models.RetrievalResult{
			Documents: docs,
			Strategy:  "bm25",
		}
		bm25ResultCh <- retrievalResult{Result: result, Error: err}
	}()

	// 等待两个检索完成
	vectorRes := <-vectorResultCh
	bm25Res := <-bm25ResultCh

	if vectorRes.Error != nil {
		return nil, fmt.Errorf("vector retrieval failed: %w", vectorRes.Error)
	}

	if bm25Res.Error != nil {
		return nil, fmt.Errorf("BM25 retrieval failed: %w", bm25Res.Error)
	}

	// RRF融合
	fusedDocuments := r.reciprocalRankFusion(
		vectorRes.Result.Documents,
		bm25Res.Result.Documents,
	)

	// 截取top-k
	if len(fusedDocuments) > r.config.TopK {
		fusedDocuments = fusedDocuments[:r.config.TopK]
	}

	result := &models.RetrievalResult{
		Documents: fusedDocuments,
		Strategy:  "hybrid",
		Query:     query,
		Latency:   float64(time.Since(startTime).Milliseconds()),
	}

	log.Infof("✅ Hybrid retrieval completed: %d results in %.2fms",
		len(fusedDocuments), result.Latency)

	return result, nil
}

// reciprocalRankFusion RRF融合算法
func (r *HybridRetriever) reciprocalRankFusion(
	vectorDocs []models.Document,
	bm25Docs []models.Document,
) []models.Document {
	// 记录每个文档的RRF分数
	type docScore struct {
		Doc   models.Document
		Score float64
	}

	scores := make(map[string]*docScore)

	// 处理向量检索结果
	for rank, doc := range vectorDocs {
		rrfScore := r.config.VectorWeight * float64(r.config.RRF) / float64(r.config.RRF+rank+1)

		if existing, exists := scores[doc.ID]; exists {
			existing.Score += rrfScore
		} else {
			scores[doc.ID] = &docScore{
				Doc:   doc,
				Score: rrfScore,
			}
		}
	}

	// 处理BM25检索结果
	for rank, doc := range bm25Docs {
		rrfScore := r.config.BM25Weight * float64(r.config.RRF) / float64(r.config.RRF+rank+1)

		if existing, exists := scores[doc.ID]; exists {
			existing.Score += rrfScore
		} else {
			scores[doc.ID] = &docScore{
				Doc:   doc,
				Score: rrfScore,
			}
		}
	}

	// 转换为切片并排序
	resultList := make([]*docScore, 0, len(scores))
	for _, item := range scores {
		resultList = append(resultList, item)
	}

	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].Score > resultList[j].Score
	})

	// 返回融合后的文档列表
	fusedDocuments := make([]models.Document, 0, len(resultList))
	for _, item := range resultList {
		doc := item.Doc
		doc.Score = float32(item.Score) // 更新分数
		fusedDocuments = append(fusedDocuments, doc)
	}

	return fusedDocuments
}

// AdaptiveRetrieval 自适应检索（根据查询复杂度调整策略）
func (r *HybridRetriever) AdaptiveRetrieval(
	ctx context.Context,
	query string,
	complexity float64,
) (*models.RetrievalResult, error) {
	log.Infof("🧠 Adaptive retrieval: query='%s', complexity=%.2f", query, complexity)

	// 根据查询复杂度动态调整权重
	var vectorWeight, bm25Weight float64

	if complexity < 0.3 {
		// 简单查询：偏向BM25
		vectorWeight = 0.3
		bm25Weight = 0.7
	} else if complexity > 0.7 {
		// 复杂查询：偏向向量检索
		vectorWeight = 0.8
		bm25Weight = 0.2
	} else {
		// 中等查询：默认权重
		vectorWeight = r.config.VectorWeight
		bm25Weight = r.config.BM25Weight
	}

	log.Infof("📊 Adaptive weights: vector=%.2f, bm25=%.2f", vectorWeight, bm25Weight)

	// 创建临时配置
	adaptiveConfig := &HybridRetrieverConfig{
		VectorWeight: vectorWeight,
		BM25Weight:   bm25Weight,
		TopK:         r.config.TopK,
		RRFK:         r.config.RRF,
	}

	// 使用自适应配置创建临时检索器
	adaptiveRetriever := &HybridRetriever{
		config:          adaptiveConfig,
		vectorRetriever: r.vectorRetriever,
		bm25Retriever:   r.bm25Retriever,
	}

	return adaptiveRetriever.Retrieve(ctx, query)
}

// QueryExpansion 查询扩展
func (r *HybridRetriever) QueryExpansion(ctx context.Context, query string) ([]string, error) {
	log.Infof("🔍 Query expansion for: %s", query)

	// 简单实现：分词后生成变体
	// 实际应用中可以使用LLM生成相关查询
	terms := r.bm25Retriever.Tokenize(query)

	if len(terms) == 0 {
		return []string{query}, nil
	}

	// 生成查询变体
	queries := []string{query} // 原始查询

	// 添加部分查询（用于召回增强）
	if len(terms) > 2 {
		for i := 0; i < len(terms)-1; i++ {
			partialQuery := fmt.Sprintf("%s %s", terms[i], terms[i+1])
			queries = append(queries, partialQuery)
		}
	}

	log.Infof("✅ Generated %d query variations", len(queries))
	return queries, nil
}

// GetStats 获取检索器统计信息
func (r *HybridRetriever) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"vector_weight": r.config.VectorWeight,
		"bm25_weight":   r.config.BM25Weight,
		"top_k":         r.config.TopK,
		"rrf_k":         r.config.RRF,
		"strategy":      "hybrid_rrf",
	}
}
