package router

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/core/retrieval"
	"cookrag-go/internal/models"
)

// QueryRouterConfig 路由器配置
type QueryRouterConfig struct {
	ComplexityThreshold float64 // 复杂度阈值
	EntityMinCount      int     // 实体最小数量
	EnableGraphRAG      bool    // 是否启用图RAG
	EnableHybrid        bool    // 是否启用混合检索
}

// DefaultQueryRouterConfig 默认配置
func DefaultQueryRouterConfig() *QueryRouterConfig {
	return &QueryRouterConfig{
		ComplexityThreshold: 0.5,
		EntityMinCount:      1,
		EnableGraphRAG:      true,
		EnableHybrid:        true,
	}
}

// QueryRouter 智能路由器
type QueryRouter struct {
	config          *QueryRouterConfig
	vectorRetriever *retrieval.VectorRetriever
	bm25Retriever   *retrieval.BM25Retriever
	graphRetriever  *retrieval.GraphRetriever
	hybridRetriever *retrieval.HybridRetriever
}

// NewQueryRouter 创建查询路由器
func NewQueryRouter(
	config *QueryRouterConfig,
	vectorRetriever *retrieval.VectorRetriever,
	bm25Retriever *retrieval.BM25Retriever,
	graphRetriever *retrieval.GraphRetriever,
	hybridRetriever *retrieval.HybridRetriever,
) *QueryRouter {
	if config == nil {
		config = DefaultQueryRouterConfig()
	}

	return &QueryRouter{
		config:          config,
		vectorRetriever: vectorRetriever,
		bm25Retriever:   bm25Retriever,
		graphRetriever:  graphRetriever,
		hybridRetriever: hybridRetriever,
	}
}

// Route 智能路由
func (r *QueryRouter) Route(ctx context.Context, query string) (*models.RetrievalResult, error) {
	startTime := time.Now()

	log.Infof("🚦 Routing query: %s", query)

	// 分析查询
	analysis := r.analyzeQuery(query)
	log.Infof("📊 Query analysis: complexity=%.2f, entities=%d, strategy=%s",
		analysis.Complexity, analysis.RelationshipIntensity, analysis.RecommendedStrategy)

	// 根据分析结果路由到不同的检索器
	var result *models.RetrievalResult
	var err error

	switch analysis.RecommendedStrategy {
	case "graph":
		log.Infof("🕸️  Routing to Graph RAG")
		result, err = r.graphRetriever.Retrieve(ctx, query)

	case "hybrid":
		log.Infof("🔀 Routing to Hybrid Retrieval")
		result, err = r.hybridRetriever.AdaptiveRetrieval(ctx, query, analysis.Complexity)

	case "vector":
		log.Infof("🔍 Routing to Vector Retrieval")
		result, err = r.vectorRetriever.Retrieve(ctx, query)

	case "bm25":
		log.Infof("📝 Routing to BM25 Retrieval")
		docs, _ := r.bm25Retriever.Retrieve(ctx, query, 10)
		result = &models.RetrievalResult{
			Documents: docs,
			Strategy:  "bm25",
			Query:     query,
		}

	default:
		log.Infof("🔀 Routing to Hybrid (default)")
		result, err = r.hybridRetriever.Retrieve(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("retrieval failed: %w", err)
	}

	// 添加查询分析信息到结果
	result.Query = query
	result.Latency = float64(time.Since(startTime).Milliseconds())

	log.Infof("✅ Routing completed: strategy=%s, results=%d, latency=%.2fms",
		result.Strategy, len(result.Documents), result.Latency)

	return result, nil
}

// analyzeQuery 分析查询特征
func (r *QueryRouter) analyzeQuery(query string) *models.QueryAnalysis {
	analysis := &models.QueryAnalysis{
		Query: query,
	}

	// 1. 计算查询复杂度
	analysis.Complexity = r.calculateComplexity(query)

	// 2. 检测实体关系强度
	analysis.RelationshipIntensity = r.detectRelationshipIntensity(query)

	// 3. 计算置信度
	analysis.Confidence = r.calculateConfidence(analysis)

	// 4. 推荐检索策略
	analysis.RecommendedStrategy = r.recommendStrategy(analysis)

	return analysis
}

// calculateComplexity 计算查询复杂度
func (r *QueryRouter) calculateComplexity(query string) float64 {
	complexity := 0.0

	// 1. 查询长度（归一化）
	lengthScore := float64(len(query)) / 100.0
	if lengthScore > 1.0 {
		lengthScore = 1.0
	}
	complexity += lengthScore * 0.2

	// 2. 关键词数量
	words := strings.Fields(query)
	keywordScore := float64(len(words)) / 20.0
	if keywordScore > 1.0 {
		keywordScore = 1.0
	}
	complexity += keywordScore * 0.3

	// 3. 特殊字符和符号
	specialChars := regexp.MustCompile(`[？?！!，,、;；]`).FindAllString(query, -1)
	specialScore := float64(len(specialChars)) / 5.0
	if specialScore > 1.0 {
		specialScore = 1.0
	}
	complexity += specialScore * 0.2

	// 4. 逻辑词检测
	logicWords := []string{"和", "或", "但是", "因为", "所以", "如果", "那么", "and", "or", "but", "because"}
	for _, word := range logicWords {
		if strings.Contains(strings.ToLower(query), word) {
			complexity += 0.1
		}
	}

	if complexity > 1.0 {
		complexity = 1.0
	}

	return complexity
}

// detectRelationshipIntensity 检测关系强度（是否需要图检索）
func (r *QueryRouter) detectRelationshipIntensity(query string) float64 {
	intensity := 0.0

	// 1. 通用关系词检测
	relationWords := []string{
		"关联", "关系", "联系", "依赖", "相关", "连接",
		"related", "relationship", "connection", "link", "associate",
	}
	for _, word := range relationWords {
		if strings.Contains(strings.ToLower(query), word) {
			intensity += 0.3
		}
	}

	// 2. 菜谱场景关系词（新增）
	recipeRelationWords := []string{
		// 食材相关
		"食材", "配料", "主料", "辅料", "代替", "替代", "替换",
		"用...做", "用...可以", "还有什么", "类似",
		// 分类相关
		"菜系", "属于什么菜", "分类", "类型",
		// 关联查询
		"还能", "也可以", "其他的", "相关的",
		// 组合查询
		"和", "搭配", "一起", "含有", "包含",
	}
	for _, word := range recipeRelationWords {
		if strings.Contains(query, word) {
			intensity += 0.25
		}
	}

	// 3. 多实体检测（简单的名词短语检测）
	// 使用正确的中文Unicode范围
	entityPattern := regexp.MustCompile(`[\x{4e00}-\x{9fa5}]{2,4}|[a-zA-Z]{3,}`)
	entities := entityPattern.FindAllString(query, -1)
	entityScore := float64(len(entities)) / 5.0
	if entityScore > 1.0 {
		entityScore = 1.0
	}
	intensity += entityScore * 0.5

	// 4. 层级关系词
	hierarchyWords := []string{
		"包含", "属于", "部分", "子类", "父类",
		"contain", "include", "part of", "subclass", "parent",
	}
	for _, word := range hierarchyWords {
		if strings.Contains(strings.ToLower(query), word) {
			intensity += 0.2
		}
	}

	// 5. 菜谱特定模式（新增）
	// "用A可以做B" -> 图检索
	if regexp.MustCompile(`用.+做.*菜`).MatchString(query) {
		intensity += 0.4
	}
	// "A和B能做什么" -> 图检索
	if regexp.MustCompile(`.+和.+能.*做`).MatchString(query) {
		intensity += 0.4
	}
	// "和...类似的" -> 图检索
	if strings.Contains(query, "类似") || strings.Contains(query, "相似") {
		intensity += 0.3
	}

	if intensity > 1.0 {
		intensity = 1.0
	}

	return intensity
}

// calculateConfidence 计算置信度
func (r *QueryRouter) calculateConfidence(analysis *models.QueryAnalysis) float64 {
	// 简单的置信度计算
	confidence := 0.7 // 基础置信度

	// 根据复杂度和关系强度调整
	if analysis.Complexity > 0.7 {
		confidence += 0.1
	}

	if analysis.RelationshipIntensity > 0.6 {
		confidence += 0.1
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// recommendStrategy 推荐检索策略
func (r *QueryRouter) recommendStrategy(analysis *models.QueryAnalysis) string {
	// 优先级1：图RAG（如果检测到强关系且启用）
	if r.config.EnableGraphRAG && analysis.RelationshipIntensity > 0.6 {
		return "graph"
	}

	// 优先级2：混合检索（如果查询复杂且启用）
	if r.config.EnableHybrid && analysis.Complexity > r.config.ComplexityThreshold {
		return "hybrid"
	}

	// 优先级3：向量检索（语义理解，优先使用）
	// 降低阈值，让更多查询使用向量检索，因为向量检索效果更好
	if analysis.Complexity > 0.0 {
		return "vector"
	}

	// 默认：BM25（几乎不会用到，除非空查询）
	return "bm25"
}

// BatchRoute 批量路由
func (r *QueryRouter) BatchRoute(ctx context.Context, queries []string) ([]*models.RetrievalResult, error) {
	log.Infof("🚦 Batch routing %d queries", len(queries))

	results := make([]*models.RetrievalResult, 0, len(queries))

	for _, query := range queries {
		result, err := r.Route(ctx, query)
		if err != nil {
			log.Warnf("⚠️  Query failed: %s, error: %v", query, err)
			continue
		}
		results = append(results, result)
	}

	log.Infof("✅ Batch routing completed: %d/%d successful",
		len(results), len(queries))

	return results, nil
}

// GetStats 获取路由器统计信息
func (r *QueryRouter) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"complexity_threshold": r.config.ComplexityThreshold,
		"entity_min_count":     r.config.EntityMinCount,
		"enable_graph_rag":     r.config.EnableGraphRAG,
		"enable_hybrid":        r.config.EnableHybrid,
		"strategy":             "intelligent_routing",
	}
}
