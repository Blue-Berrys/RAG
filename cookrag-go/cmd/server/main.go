package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"cookrag-go/internal/config"
	"cookrag-go/pkg/ml/embedding"
)

func main() {
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
	embeddingProvider, err := embedding.NewProvider(cfg.Embedding)
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
	fmt.Println("\n" + "="*60)
	fmt.Println("🎉 CookRAG-Go Initialization Successful!")
	fmt.Println("="*60)
	fmt.Printf("Embedding Provider: %s (Model: %s)\n", cfg.Embedding.Provider, cfg.Embedding.Model)
	fmt.Printf("Vector Dimension:  %d\n", embeddingProvider.Dimension())
	fmt.Printf("Milvus:            %s:%s\n", cfg.Milvus.Host, cfg.Milvus.Port)
	fmt.Printf("Neo4j:             %s\n", cfg.Neo4j.URI)
	fmt.Printf("Redis:             %s:%s\n", cfg.Redis.Host, cfg.Redis.Port)
	fmt.Println("="*60)
	fmt.Println("\n📝 Next steps:")
	fmt.Println("1. Start Milvus: docker-compose up -d milvus etcd minio")
	fmt.Println("2. Start Neo4j: docker-compose up -d neo4j")
	fmt.Println("3. Start Redis: docker-compose up -d redis")
	fmt.Println("4. Run: go run cmd/server/main.go")
	fmt.Println("\n💡 Get your free API key at: https://open.bigmodel.cn/")
	fmt.Println()
}
