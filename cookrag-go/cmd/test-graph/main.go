package main

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/config"
	"cookrag-go/internal/core/router"
	"cookrag-go/internal/core/retrieval"
	"cookrag-go/pkg/ml/embedding"
	"cookrag-go/pkg/storage/cache"
	"cookrag-go/pkg/storage/milvus"
	"cookrag-go/pkg/storage/neo4j"
)

func main() {
	log.SetLevel(log.InfoLevel)
	log.SetReportTimestamp(true)
	log.SetTimeFormat(time.Kitchen)

	// 加载配置
	cfg, _ := config.Load("config/config.yaml")

	// 初始化各个组件
	embeddingProvider, _ := embedding.NewProvider(embedding.Config{
		Provider: cfg.Embedding.Provider,
		APIKey:   cfg.Embedding.APIKey,
		Model:    cfg.Embedding.Model,
		Timeout:  cfg.Embedding.Timeout,
	})

	milvusClient, _ := milvus.NewClient(cfg.Milvus.Host, cfg.Milvus.Port)

	neo4jClient, _ := neo4j.NewClient(cfg.Neo4j.URI, cfg.Neo4j.Username, cfg.Neo4j.Password, cfg.Neo4j.Database)

	redisClient, _ := cache.NewRedisClient(cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password, cfg.Redis.DB)

	// 创建检索器
	vectorRetriever := retrieval.NewVectorRetriever(retrieval.DefaultVectorRetrieverConfig(), embeddingProvider, milvusClient, redisClient)
	bm25Retriever := retrieval.NewBM25Retriever(retrieval.DefaultBM25Config())
	graphRetriever := retrieval.NewGraphRetriever(retrieval.DefaultGraphRetrieverConfig(), neo4jClient)
	hybridRetriever := retrieval.NewHybridRetriever(retrieval.DefaultHybridRetrieverConfig(), vectorRetriever, bm25Retriever)

	// 创建路由器
	queryRouter := router.NewQueryRouter(
		router.DefaultQueryRouterConfig(),
		vectorRetriever,
		bm25Retriever,
		graphRetriever,
		hybridRetriever,
	)

	ctx := context.Background()

	// 测试查询 - 这些查询应该触发图检索
	graphQueries := []string{
		"用鸡蛋能做哪些菜？",        // 食材关系查询
		"西红柿和鸡蛋搭配能做什么？",   // 组合查询
		"和红烧肉类似的菜有哪些？",    // 相似查询
		"川菜里有哪些辣的菜？",       // 分类关系查询
	}

	log.Infof("\n========================================")
	log.Infof("🧪 Testing Graph-based Queries")
	log.Infof("========================================")

	for _, query := range graphQueries {
		log.Infof("\n----------------------------------------")
		log.Infof("🔍 Query: %s", query)

		result, err := queryRouter.Route(ctx, query)
		if err != nil {
			log.Errorf("❌ Error: %v", err)
			continue
		}

		log.Infof("✅ Strategy: %s", result.Strategy)
		log.Infof("   Results: %d", len(result.Documents))
		log.Infof("   Latency: %.2fms", result.Latency)

		// 如果使用了图检索，显示一些细节
		if result.Strategy == "graph" {
			log.Infof("   🕸️  Graph retrieval triggered!")
		}
	}

	// 清理
	milvusClient.Close(ctx)
	neo4jClient.Close(ctx)
	redisClient.Close()
}
