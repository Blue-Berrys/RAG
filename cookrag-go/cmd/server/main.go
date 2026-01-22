package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cookrag-go/internal/config"
	"cookrag-go/pkg/ml/embedding"
)

// initLoggingWithFile 初始化标准库日志，同时输出到终端和文件
func initLoggingWithFile() (*os.File, error) {
	// 创建 log 目录
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// 生成日志文件名（按日期）
	logFileName := fmt.Sprintf("server-%s.log", time.Now().Format("2006-01-02"))
	logFilePath := filepath.Join(logDir, logFileName)

	// 打开日志文件（追加模式）
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// 设置日志同时输出到终端和文件
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Printf("📝 Log file: %s", logFilePath)

	return logFile, nil
}

func main() {
	logFile, err := initLoggingWithFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	log.Println("🚀 Starting CookRAG-Go Server...")

	// 1. 加载配置
	configPath := "config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	log.Printf("✅ Config loaded from %s", configPath)
	log.Printf("📊 Server mode: %s, Port: %s", cfg.Server.Mode, cfg.Server.Port)

	// 2. 初始化Embedding Provider
	log.Printf("🔤 Initializing embedding provider: %s", cfg.Embedding.Provider)
	embeddingConfig := embedding.Config{
		Provider: cfg.Embedding.Provider,
		APIKey:   cfg.Embedding.APIKey,
		Model:    cfg.Embedding.Model,
		BaseURL:  cfg.Embedding.BaseURL,
		Timeout:  cfg.Embedding.Timeout,
	}
	embeddingProvider, err := embedding.NewProvider(embeddingConfig)
	if err != nil {
		log.Fatalf("❌ Failed to create embedding provider: %v", err)
	}

	// 3. 测试向量化
	ctx := context.Background()
	log.Println("🧪 Testing embedding...")
	testEmbedding, err := embeddingProvider.Embed(ctx, "红烧肉怎么做？")
	if err != nil {
		log.Fatalf("❌ Failed to test embedding: %v", err)
	}

	log.Printf("✅ Embedding test successful!")
	log.Printf("   Dimension: %d", embeddingProvider.Dimension())
	log.Printf("   Sample (first 5): %v", testEmbedding[:5])

	// 4. 测试批量向量化
	log.Println("🧪 Testing batch embedding...")
	testTexts := []string{
		"红烧肉怎么做？",
		"宫保鸡丁需要什么食材？",
		"糖醋排骨的做法",
	}
	batchEmbeddings, err := embeddingProvider.EmbedBatch(ctx, testTexts)
	if err != nil {
		log.Fatalf("❌ Failed to test batch embedding: %v", err)
	}

	log.Printf("✅ Batch embedding test successful!")
	log.Printf("   Processed %d texts", len(batchEmbeddings))
	for i, emb := range batchEmbeddings {
		log.Printf("   [%d] Dimension: %d, Sample: %v", i+1, len(emb), emb[:3])
	}

	// 5. 显示配置信息
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎉 CookRAG-Go Initialization Successful!")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Embedding Provider: %s (Model: %s)\n", cfg.Embedding.Provider, cfg.Embedding.Model)
	fmt.Printf("Vector Dimension:  %d\n", embeddingProvider.Dimension())
	fmt.Printf("Milvus:            %s:%s\n", cfg.Milvus.Host, cfg.Milvus.Port)
	fmt.Printf("Neo4j:             %s\n", cfg.Neo4j.URI)
	fmt.Printf("Redis:             %s:%s\n", cfg.Redis.Host, cfg.Redis.Port)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\n📝 Next steps:")
	fmt.Println("1. Start Milvus: docker-compose up -d milvus etcd minio")
	fmt.Println("2. Start Neo4j: docker-compose up -d neo4j")
	fmt.Println("3. Start Redis: docker-compose up -d redis")
	fmt.Println("4. Run: go run cmd/server/main.go")
	fmt.Println("\n💡 Get your free API key at: https://open.bigmodel.cn/")
	fmt.Println()
}
