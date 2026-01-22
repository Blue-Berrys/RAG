package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// initLoggingWithFile 初始化日志配置（同时输出到终端和文件）
func initLoggingWithFile() (*os.File, error) {
	log.SetLevel(log.InfoLevel)
	log.SetReportTimestamp(true)
	log.SetTimeFormat(time.Kitchen)

	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logFileName := fmt.Sprintf("test-graph-%s.log", time.Now().Format("2006-01-02"))
	logFilePath := filepath.Join(logDir, logFileName)

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)

	log.Infof("📝 Log file: %s", logFilePath)

	return logFile, nil
}

func main() {
	logFile, err := initLoggingWithFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

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
