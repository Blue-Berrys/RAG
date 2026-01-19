package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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

func main() {
	// 设置日志
	log.SetLevel(log.InfoLevel)
	log.SetReportTimestamp(true)
	log.SetTimeFormat(time.Kitchen)

	log.Info("🚀 Starting CookRAG-Go Enterprise RAG System...")

	// 1. 加载配置
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}
	log.Info("✅ Config loaded")

	// 2. 初始化Embedding提供者
	embeddingConfig := embeddingCfg.Config{
		Provider: cfg.Embedding.Provider,
		APIKey:   cfg.Embedding.APIKey,
		Model:    cfg.Embedding.Model,
		Timeout:  cfg.Embedding.Timeout,
	}
	embeddingProvider, err := embeddingCfg.NewProvider(embeddingConfig)
	if err != nil {
		log.Fatalf("❌ Failed to initialize embedding provider: %v", err)
	}
	log.Infof("✅ Embedding provider initialized: %s (dimension: %d)",
		cfg.Embedding.Provider, embeddingProvider.Dimension())

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
	var vectorRetriever *retrieval.VectorRetriever
	if redisCache != nil {
		vectorRetriever = retrieval.NewVectorRetriever(
			retrieval.DefaultVectorRetrieverConfig(),
			embeddingProvider,
			milvusClient,
			redisCache,
		)
	} else {
		vectorRetriever = retrieval.NewVectorRetriever(
			retrieval.DefaultVectorRetrieverConfig(),
			embeddingProvider,
			milvusClient,
			nil,
		)
	}

	bm25Retriever := retrieval.NewBM25Retriever(retrieval.DefaultBM25Config())

	graphRetriever := retrieval.NewGraphRetriever(
		retrieval.DefaultGraphRetrieverConfig(),
		neo4jClient,
	)

	hybridRetriever := retrieval.NewHybridRetriever(
		retrieval.DefaultHybridRetrieverConfig(),
		vectorRetriever,
		bm25Retriever,
	)

	// 5. 初始化路由器
	queryRouter := router.NewQueryRouter(
		router.DefaultQueryRouterConfig(),
		vectorRetriever,
		bm25Retriever,
		graphRetriever,
		hybridRetriever,
	)

	// 6. 初始化LLM生成器
	llmProvider, err := llm.NewZhipuLLM("glm-4-flash")
	if err != nil {
		log.Warnf("⚠️  Failed to initialize LLM: %v", err)
		llmProvider = nil
	} else {
		log.Info("✅ LLM provider initialized")
	}

	// 7. 启动监控
	metricsCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go observability.Global.StartMetricsReporter(metricsCtx, 30*time.Second)

	// 8. 演示完整的RAG流程（包含LLM生成）
	demonstrateCompleteRAG(metricsCtx, queryRouter, llmProvider, vectorRetriever, embeddingProvider, milvusClient)

	// 9. 启动HTTP服务器
	go func() {
		srv := server.NewServer(server.DefaultConfig())
		if err := srv.Start(); err != nil {
			log.Errorf("❌ HTTP server error: %v", err)
		}
	}()

	// 10. 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("🛑 Shutting down...")

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

// demonstrateCompleteRAG 演示完整的RAG流程（包含LLM生成）
func demonstrateCompleteRAG(ctx context.Context, queryRouter *router.QueryRouter, llmProvider *llm.ZhipuLLM, vectorRetriever *retrieval.VectorRetriever, embeddingProvider embeddingCfg.Provider, milvusClient *milvus.Client) {
	log.Info("📚 Running Complete RAG Demonstration...")

	// 从 docs/dishes 目录加载所有菜谱文档
	documents, err := loadDocumentsFromDir("docs/dishes")
	if err != nil {
		log.Warnf("⚠️  Failed to load documents: %v", err)
		log.Infof("📝 Using sample documents instead...")
		documents = getSampleDocuments()
	}

	log.Infof("📚 Loaded %d documents", len(documents))

	// 索引到BM25
	log.Infof("📝 Indexing %d documents with BM25...", len(documents))
	bm25Retriever := retrieval.NewBM25Retriever(retrieval.DefaultBM25Config())
	if err := bm25Retriever.IndexDocuments(ctx, documents); err != nil {
		log.Warnf("⚠️  Failed to index BM25: %v", err)
	} else {
		log.Infof("✅ BM25 indexing completed: %d docs", len(documents))
	}

	// 如果有向量检索器，索引到Milvus
	if vectorRetriever != nil && embeddingProvider != nil && milvusClient != nil {
		log.Infof("📦 Indexing %d documents to Milvus for vector search...", len(documents))

		// 确保 Milvus 集合存在
		collectionName := "cookrag_documents"
		hasCollection, err := milvusClient.HasCollection(ctx, collectionName)
		if err != nil {
			log.Warnf("⚠️  Failed to check collection: %v", err)
		} else if !hasCollection {
			log.Infof("📦 Creating Milvus collection: %s", collectionName)
			if err := milvusClient.CreateCollection(ctx, collectionName, embeddingProvider.Dimension()); err != nil {
				log.Warnf("⚠️  Failed to create collection: %v", err)
			} else {
				log.Infof("✅ Collection created: %s", collectionName)

				// 创建索引
				if err := milvusClient.CreateIndex(ctx, collectionName, "vector", "IVF_FLAT", map[string]string{}); err != nil {
					log.Warnf("⚠️  Failed to create index: %v", err)
				} else {
					log.Infof("✅ Index created on collection: %s", collectionName)
				}

				// 加载集合
				if err := milvusClient.LoadCollection(ctx, collectionName); err != nil {
					log.Warnf("⚠️  Failed to load collection: %v", err)
				} else {
					log.Infof("✅ Collection loaded: %s", collectionName)
				}
			}
		} else {
			// 集合已存在，确保已加载
			if err := milvusClient.LoadCollection(ctx, collectionName); err != nil {
				log.Warnf("⚠️  Failed to load collection: %v", err)
			}
			log.Infof("✅ Collection already exists: %s", collectionName)
		}

		// 索引文档
		if err := vectorRetriever.IndexDocuments(ctx, documents); err != nil {
			log.Warnf("⚠️  Failed to index to Milvus: %v", err)
		} else {
			log.Infof("✅ Documents indexed to Milvus")
		}
	}

	// 演示查询
	queries := []string{
		"红烧肉怎么做？",
		"川菜有哪些特色？",
		"有什么好吃的素食菜？",
		"西红柿豆腐汤羹怎么做？",
	}

	for _, query := range queries {
		log.Infof("\n" + strings.Repeat("=", 70))
		log.Infof("🔍 Query: %s", query)
		log.Infof(strings.Repeat("=", 70))

		// 1. 检索相关文档
		startTime := time.Now()
		result, err := queryRouter.Route(ctx, query)
		latency := time.Since(startTime).Milliseconds()

		if err != nil {
			log.Errorf("❌ Query failed: %v", err)
			observability.Global.RecordError()
			continue
		}

		log.Infof("✅ Retrieval Result:")
		log.Infof("  Strategy: %s", result.Strategy)
		log.Infof("  Documents Found: %d", len(result.Documents))
		log.Infof("  Retrieval Latency: %dms", latency)

		// 显示检索到的文档
		if len(result.Documents) > 0 {
			log.Infof("\n📄 Retrieved Documents:")
			for i, doc := range result.Documents {
				if i >= 3 { // 显示前3个
					break
				}
				log.Infof("  [%d] Score: %.4f", i+1, doc.Score)
				log.Infof("      Content: %.150s...", doc.Content)
			}
		} else {
			log.Warnf("  ⚠️  No documents found - using general knowledge for LLM")
			result.Documents = []models.Document{
				{
					ID:      "general",
					Content: "（无相关文档检索到，将基于常识回答）",
					Metadata: map[string]interface{}{"source": "general"},
				},
			}
		}

		// 2. 使用LLM生成答案
		if llmProvider != nil {
			log.Infof("\n🤖 Generating AI Answer...")

			// 构建上下文
			context := buildContext(result.Documents)

			// 构建提示词
			prompt := buildPrompt(query, context)

			// 调用LLM生成
			llmStartTime := time.Now()
			answer, err := llmProvider.Generate(ctx, prompt)
			llmLatency := time.Since(llmStartTime).Milliseconds()

			if err != nil {
				log.Errorf("❌ LLM generation failed: %v", err)
			} else {
				log.Infof("✅ AI Answer Generated (LLM Latency: %dms):", llmLatency)
				log.Infof("\n📝 Answer:\n%s\n", answer)
			}
		} else {
			log.Warnf("\n⚠️  LLM not available - skipping answer generation")
		}

		observability.Global.RecordQuery(time.Duration(latency)*time.Millisecond, result.Strategy)
	}

	log.Info("\n✅ Demonstration completed")
}

// loadDocumentsFromDir 从目录加载所有 Markdown 文档
func loadDocumentsFromDir(dir string) ([]models.Document, error) {
	var documents []models.Document

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录和非 markdown 文件
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		// 读取文件内容
		content, err := os.ReadFile(path)
		if err != nil {
			log.Warnf("⚠️  Failed to read file %s: %v", path, err)
			return nil
		}

		// 获取相对路径作为 ID
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			relPath = path
		}

		// 提取类别（从父目录名）
		category := "未分类"
		if parts := strings.Split(relPath, string(filepath.Separator)); len(parts) > 1 {
			category = parts[0]
		}

		// 提取菜名（从文件名）
		dishName := strings.TrimSuffix(filepath.Base(path), ".md")

		// 创建文档
		doc := models.Document{
			ID:      relPath,
			Content: string(content),
			Metadata: map[string]interface{}{
				"file":     relPath,
				"category": category,
				"dish":     dishName,
			},
		}

		documents = append(documents, doc)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	if len(documents) == 0 {
		return nil, fmt.Errorf("no documents found in directory: %s", dir)
	}

	return documents, nil
}

// getSampleDocuments 获取示例文档（作为后备）
func getSampleDocuments() []models.Document {
	return []models.Document{
		{
			ID:      "doc1",
			Content: "红烧肉是一道经典的中国菜，主要食材是五花肉，用酱油、糖、料酒等调料炖煮而成。做法是先将五花肉切块焯水，然后用糖炒糖色，加入酱油、料酒、八角、桂皮等调料小火慢炖1-2小时，直到肉质软烂，肥而不腻。红烧肉富含蛋白质和脂肪，是中式料理的代表之一。",
			Metadata: map[string]interface{}{
				"category": "肉类",
				"cuisine":  "中式",
				"difficulty": "简单",
			},
		},
		{
			ID:      "doc2",
			Content: "宫保鸡丁是四川传统名菜，属于川菜代表。主料是鸡胸肉和花生米，调料包括干辣椒、花椒、葱姜蒜、糖醋汁。制作要点是先将鸡胸肉切丁上浆，然后热油快炒，保持鸡肉嫩滑。特点是酸甜微辣，鸡肉嫩滑，花生酥脆，营养均衡。",
			Metadata: map[string]interface{}{
				"category": "肉类",
				"cuisine":  "川菜",
				"difficulty": "中等",
			},
		},
		{
			ID:      "doc3",
			Content: "麻婆豆腐是川菜中的经典素食菜品，发明于清朝同治年间。主要食材是嫩豆腐和牛肉末，调料有豆瓣酱、花椒、辣椒面。特点是麻、辣、鲜、香、烫，口感丰富。制作关键是豆腐要先焯水去豆腥味，炒制时要小火慢炖让豆腐充分入味。",
			Metadata: map[string]interface{}{
				"category": "素食",
				"cuisine":  "川菜",
				"difficulty": "简单",
			},
		},
	}
}

// buildContext 从文档构建上下文
func buildContext(docs []models.Document) string {
	if len(docs) == 0 {
		return "（无相关文档）"
	}

	context := "基于以下相关信息：\n\n"
	for i, doc := range docs {
		context += fmt.Sprintf("[%d] %s\n", i+1, doc.Content)
		if len(doc.Metadata) > 0 {
			context += fmt.Sprintf("    元数据: %v\n", doc.Metadata)
		}
		context += "\n"
	}
	return context
}

// buildPrompt 构建LLM提示词
func buildPrompt(query, context string) string {
	return fmt.Sprintf(`你是一个专业的烹饪助手。请根据提供的信息回答用户的问题。

%s

问题：%s

请提供详细、准确、有帮助的回答。如果提供的信息不足以完整回答问题，请结合你的知识给出建议，但要说明哪些是来自提供的信息，哪些是基于常识的建议。

回答：`, context, query)
}
