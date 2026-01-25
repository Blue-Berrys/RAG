package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/api/server"
	"cookrag-go/internal/config"
	"cookrag-go/internal/core/retrieval"
	"cookrag-go/internal/core/router"
	"cookrag-go/internal/models"
	"cookrag-go/internal/observability"
	embeddingCfg "cookrag-go/pkg/ml/embedding"
	"cookrag-go/pkg/ml/llm"
	"cookrag-go/pkg/storage/cache"
	"cookrag-go/pkg/storage/milvus"
	"cookrag-go/pkg/storage/neo4j"
)

// initLoggingWithFile 初始化日志配置（同时输出到终端和文件）
func initLoggingWithFile() (*os.File, error) {
	// 设置日志级别
	log.SetLevel(log.InfoLevel)
	log.SetReportTimestamp(true)
	log.SetTimeFormat(time.Kitchen)

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
	// charmbracelet/log 的 logger 是一个全局变量
	log.SetOutput(logFile)

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

	log.Info("🚀 Starting CookRAG-Go Server...")

	// 1. 加载配置
	configPath := "config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	log.Infof("✅ Config loaded from %s", configPath)
	log.Infof("📊 Server mode: %s, Port: %s", cfg.Server.Mode, cfg.Server.Port)

	// 2. 初始化Embedding Provider
	log.Infof("🔤 Initializing embedding provider: %s", cfg.Embedding.Provider)
	embeddingConfig := embeddingCfg.Config{
		Provider: cfg.Embedding.Provider,
		APIKey:   cfg.Embedding.APIKey,
		Model:    cfg.Embedding.Model,
		BaseURL:  cfg.Embedding.BaseURL,
		Timeout:  cfg.Embedding.Timeout,
	}
	embeddingProvider, err := embeddingCfg.NewProvider(embeddingConfig)
	if err != nil {
		log.Fatalf("❌ Failed to create embedding provider: %v", err)
	}
	log.Infof("✅ Embedding provider initialized: %s (dimension: %d)", cfg.Embedding.Provider, embeddingProvider.Dimension())

	// 3. 初始化存储客户端
	milvusClient, err := milvus.NewClient(cfg.Milvus.Host, cfg.Milvus.Port)
	if err != nil {
		log.Warnf("⚠️  Failed to connect to Milvus: %v", err)
		milvusClient = nil
	} else {
		log.Info("✅ Milvus client connected")
	}

	neo4jClient, err := neo4j.NewClient(
		cfg.Neo4j.URI,
		cfg.Neo4j.Username,
		cfg.Neo4j.Password,
		cfg.Neo4j.Database,
	)
	if err != nil {
		log.Warnf("⚠️  Failed to connect to Neo4j: %v", err)
		neo4jClient = nil
	} else {
		log.Info("✅ Neo4j client connected")
	}

	var redisCache cache.Cache
	redisClient, err := cache.NewRedisClient(
		cfg.Redis.Host,
		cfg.Redis.Port,
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	if err != nil {
		log.Warnf("⚠️  Failed to connect to Redis: %v", err)
		redisCache = nil
		log.Info("⚠️  Running without cache")
	} else {
		redisCache = redisClient
		log.Info("✅ Redis client connected")
	}

	// 4. 初始化检索器
	ctx := context.Background()

	var vectorRetriever *retrieval.VectorRetriever
	vectorRetriever = retrieval.NewVectorRetriever(
		retrieval.DefaultVectorRetrieverConfig(),
		embeddingProvider,
		milvusClient,
		redisCache,
	)
	log.Info("✅ Vector retriever initialized")

	bm25Retriever := retrieval.NewBM25Retriever(retrieval.DefaultBM25Config())
	log.Info("✅ BM25 retriever initialized")

	graphRetriever := retrieval.NewGraphRetriever(
		retrieval.DefaultGraphRetrieverConfig(),
		neo4jClient,
	)
	log.Info("✅ Graph retriever initialized")

	hybridRetriever := retrieval.NewHybridRetriever(
		retrieval.DefaultHybridRetrieverConfig(),
		vectorRetriever,
		bm25Retriever,
	)
	log.Info("✅ Hybrid retriever initialized")

	// 5. 初始化路由器
	queryRouter := router.NewQueryRouter(
		router.DefaultQueryRouterConfig(),
		vectorRetriever,
		bm25Retriever,
		graphRetriever,
		hybridRetriever,
	)
	log.Info("✅ Query router initialized")

	// 6. 初始化LLM (可选，用于生成答案)
	var llmProvider *llm.ZhipuLLM
	llmProvider, err = llm.NewZhipuLLM("glm-4-flash")
	if err != nil {
		log.Warnf("⚠️  Failed to initialize LLM: %v", err)
		llmProvider = nil
	} else {
		log.Info("✅ LLM provider initialized")
	}

	// 7. 初始化文档（如果Milvus为空）
	initializeDocuments(ctx, vectorRetriever, bm25Retriever, embeddingProvider, milvusClient)

	// 8. 启动监控
	metricsCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go observability.Global.StartMetricsReporter(metricsCtx, 30*time.Second)

	// 9. 启动HTTP服务器
	serverConfig := &server.Config{
		Port:           8080,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}
	srv := server.NewServer(serverConfig, queryRouter, llmProvider)

	// 10. 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 在goroutine中启动服务器
	go func() {
		log.Infof("🌐 HTTP server starting on port %d", 8080)
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Errorf("❌ HTTP server error: %v", err)
		}
	}()

	// 等待信号
	<-sigChan
	log.Info("🛑 Shutting down...")

	// 优雅关闭服务器
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorf("❌ Server shutdown error: %v", err)
	}

	// 清理资源
	if milvusClient != nil {
		milvusClient.Close(context.Background())
	}
	if neo4jClient != nil {
		neo4jClient.Close(context.Background())
	}
	if redisClient != nil {
		redisClient.Close()
	}

	observability.Global.LogMetrics()
	log.Info("✅ Shutdown completed")
}

// initializeDocuments 初始化文档（如果需要）
func initializeDocuments(ctx context.Context, vectorRetriever *retrieval.VectorRetriever, bm25Retriever *retrieval.BM25Retriever, embeddingProvider embeddingCfg.Provider, milvusClient *milvus.Client) {
	log.Info("📚 Initializing documents...")

	// 加载示例文档
	documents := getSampleDocuments()
	log.Infof("📚 Loaded %d sample documents", len(documents))

	// 索引到BM25
	bm25Retriever.IndexDocuments(ctx, documents)
	log.Info("✅ Documents indexed to BM25")

	// 索引到Milvus（如果需要）
	if vectorRetriever != nil && embeddingProvider != nil && milvusClient != nil {
		collectionName := "cookrag_documents"
		hasCollection, err := milvusClient.HasCollection(ctx, collectionName)
		if err != nil {
			log.Warnf("⚠️  Failed to check collection: %v", err)
			return
		}

		if !hasCollection {
			log.Infof("📦 Creating Milvus collection: %s", collectionName)
			if err := milvusClient.CreateCollection(ctx, collectionName, embeddingProvider.Dimension()); err != nil {
				log.Warnf("⚠️  Failed to create collection: %v", err)
				return
			}

			// 创建索引
			if err := milvusClient.CreateIndex(ctx, collectionName, "vector", "IVF_FLAT", map[string]string{}); err != nil {
				log.Warnf("⚠️  Failed to create index: %v", err)
			}

			// 加载集合
			if err := milvusClient.LoadCollection(ctx, collectionName); err != nil {
				log.Warnf("⚠️  Failed to load collection: %v", err)
				return
			}

			// 插入数据
			if err := vectorRetriever.IndexDocuments(ctx, documents); err != nil {
				log.Warnf("⚠️  Failed to index to Milvus: %v", err)
			} else {
				log.Info("✅ Documents indexed to Milvus")
			}
		} else {
			// 检查是否已有数据
			stats, err := milvusClient.GetCollectionStats(ctx, collectionName)
			if err != nil {
				log.Warnf("⚠️  Failed to get collection stats: %v", err)
				return
			}

			rowCount := int64(0)
			if count, ok := stats["row_count"]; ok {
				switch v := count.(type) {
				case int64:
					rowCount = v
				case string:
					fmt.Sscanf(v, "%d", &rowCount)
				case float64:
					rowCount = int64(v)
				}
			}

			if rowCount == 0 {
				log.Infof("📝 Collection is empty, inserting %d documents", len(documents))
				if err := vectorRetriever.IndexDocuments(ctx, documents); err != nil {
					log.Warnf("⚠️  Failed to index to Milvus: %v", err)
				} else {
					log.Info("✅ Documents indexed to Milvus")
				}
			} else {
				log.Infof("⏭️  Collection already has %d documents", rowCount)
			}

			// 确保集合已加载
			milvusClient.LoadCollection(ctx, collectionName)
		}
	}
}

// getSampleDocuments 获取示例文档
func getSampleDocuments() []models.Document {
	return []models.Document{
		{
			ID:      "doc1",
			Content: "红烧肉是一道经典的中国菜，主要食材是五花肉，用酱油、糖、料酒等调料炖煮而成。做法是先将五花肉切块焯水，然后用糖炒糖色，加入酱油、料酒、八角、桂皮等调料小火慢炖1-2小时，直到肉质软烂，肥而不腻。红烧肉富含蛋白质和脂肪，是中式料理的代表之一。",
			Metadata: map[string]interface{}{
				"category": "肉类",
				"cuisine":  "中式",
			},
		},
		{
			ID:      "doc2",
			Content: "宫保鸡丁是四川传统名菜，属于川菜代表。主料是鸡胸肉和花生米，调料包括干辣椒、花椒、葱姜蒜、糖醋汁。制作要点是先将鸡胸肉切丁上浆，然后热油快炒，保持鸡肉嫩滑。特点是酸甜微辣，鸡肉嫩滑，花生酥脆，营养均衡。",
			Metadata: map[string]interface{}{
				"category": "肉类",
				"cuisine":  "川菜",
			},
		},
		{
			ID:      "doc3",
			Content: "麻婆豆腐是川菜中的经典素食菜品，发明于清朝同治年间。主要食材是嫩豆腐和牛肉末，调料有豆瓣酱、花椒、辣椒面。特点是麻、辣、鲜、香、烫，口感丰富。制作关键是豆腐要先焯水去豆腥味，炒制时要小火慢炖让豆腐充分入味。",
			Metadata: map[string]interface{}{
				"category": "素食",
				"cuisine":  "川菜",
			},
		},
	}
}
