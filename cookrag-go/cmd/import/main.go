package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/config"
	"cookrag-go/internal/models"
	"cookrag-go/pkg/data"
	"cookrag-go/pkg/ml/embedding"
	"cookrag-go/pkg/storage/milvus"
	"cookrag-go/pkg/storage/neo4j"
)

func main() {
	// 设置日志
	log.SetLevel(log.InfoLevel)
	log.SetReportTimestamp(true)
	log.SetTimeFormat(time.Kitchen)

	log.Info("🚀 Starting CookRAG-Go Data Importer...")

	// 1. 加载配置
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}
	log.Info("✅ Config loaded")

	// 2. 初始化Embedding提供者
	embeddingProvider, err := embedding.NewProvider(cfg.Embedding)
	if err != nil {
		log.Fatalf("❌ Failed to initialize embedding provider: %v", err)
	}
	log.Infof("✅ Embedding provider initialized: %s", cfg.Embedding.Provider)

	// 3. 初始化存储客户端
	milvusClient, err := milvus.NewClient(cfg.Milvus.Host, cfg.Milvus.Port)
	if err != nil {
		log.Warnf("⚠️  Failed to connect to Milvus: %v", err)
		log.Warnf("⚠️  Vector indexing will be disabled")
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
		log.Warnf("⚠️  Graph indexing will be disabled")
		neo4jClient = nil
	} else {
		log.Info("✅ Neo4j client connected")
	}

	// 4. 创建索引器
	indexer := data.NewIndexer(embeddingProvider, milvusClient, neo4jClient)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// 5. 加载数据
	log.Info("\n📚 Loading data...")

	docs, err := loadSampleData(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to load data: %v", err)
	}

	log.Infof("✅ Loaded %d documents", len(docs))

	// 显示数据样例
	if len(docs) > 0 {
		log.Info("\n📄 Sample document:")
		log.Infof("  ID: %s", docs[0].ID)
		log.Infof("  Content: %.100s...", docs[0].Content)
		log.Infof("  Metadata: %v", docs[0].Metadata)
	}

	// 6. 索引数据
	log.Info("\n📊 Starting indexing...")

	indexConfig := &data.IndexConfig{
		CollectionName:   "cookrag_documents",
		VectorIndex:      milvusClient != nil,
		BM25Index:        true,
		GraphIndex:       neo4jClient != nil,
		BatchSize:        10,
		CreateCollection: true,
	}

	if err := indexer.IndexDocuments(ctx, docs, indexConfig); err != nil {
		log.Fatalf("❌ Failed to index documents: %v", err)
	}

	// 7. 验证索引
	log.Info("\n✅ Verifying index...")

	if milvusClient != nil {
		stats, err := milvusClient.GetCollectionStats(ctx, indexConfig.CollectionName)
		if err != nil {
			log.Warnf("⚠️  Failed to get collection stats: %v", err)
		} else {
			log.Infof("📊 Milvus collection stats: %v", stats)
		}
	}

	bm25Retriever := indexer.GetBM25Retriever()
	if bm25Retriever != nil {
		stats := bm25Retriever.GetStats()
		log.Infof("📊 BM25 index stats: %v", stats)
	}

	// 8. 测试检索
	log.Info("\n🔍 Testing retrieval...")

	testQueries := []string{
		"红烧肉怎么做？",
		"有什么川菜推荐？",
		"简单快手菜",
	}

	for _, query := range testQueries {
		log.Infof("\n🔍 Query: %s", query)

		if bm25Retriever != nil {
			results, err := bm25Retriever.Retrieve(ctx, query, 3)
			if err != nil {
				log.Warnf("⚠️  BM25 retrieval failed: %v", err)
			} else {
				log.Infof("✅ BM25 found %d results", len(results))
				for i, doc := range results {
					if i >= 2 {
						break
					}
					log.Infof("  [%d] Score: %.4f, Content: %.60s...",
						i+1, doc.Score, doc.Content)
				}
			}
		}
	}

	log.Info("\n🎉 Data import completed successfully!")
	log.Infof("\n📊 Summary:")
	log.Infof("  Total documents: %d", len(docs))
	log.Infof("  Vector index: %v", indexConfig.VectorIndex)
	log.Infof("  BM25 index: %v", indexConfig.BM25Index)
	log.Infof("  Graph index: %v", indexConfig.GraphIndex)

	// 等待用户中断
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	log.Info("\n⏳ Press Ctrl+C to exit...")
	<-sigChan

	log.Info("👋 Exiting...")
}

// loadSampleData 加载示例数据
func loadSampleData(ctx context.Context) ([]models.Document, error) {
	// 尝试从文件加载
	loader := data.NewRecipeLoader("data/recipes/recipes.json")

	docs, err := loader.Load(ctx)
	if err != nil {
		log.Warnf("⚠️  Failed to load from file: %v", err)
		log.Info("📝 Using built-in sample data...")

		// 使用内置示例数据
		return getBuiltinSampleData(), nil
	}

	return docs, nil
}

// getBuiltinSampleData 获取内置示例数据
func getBuiltinSampleData() []models.Document {
	return []models.Document{
		{
			ID: "doc_1",
			Content: "红烧肉是一道经典的中国菜，主要食材是猪肉，用酱油、糖等调料炖煮而成。做法：五花肉切块，焯水后炒糖色，加调料焖40分钟收汁即可。",
			Metadata: map[string]interface{}{
				"name":     "红烧肉",
				"category": "肉类",
				"cuisine":  "中式",
				"type":     "recipe",
			},
		},
		{
			ID: "doc_2",
			Content: "宫保鸡丁是四川传统名菜，主料是鸡胸肉和花生米，口味酸甜微辣。做法：鸡肉切丁腌制，炸花生米，炒辣椒花椒，下鸡丁炒，加调料最后加花生米。",
			Metadata: map[string]interface{}{
				"name":     "宫保鸡丁",
				"category": "肉类",
				"cuisine":  "川菜",
				"type":     "recipe",
			},
		},
		{
			ID: "doc_3",
			Content: "麻婆豆腐是川菜代表之一，主要食材是豆腐和牛肉末，口感麻辣鲜香。做法：豆腐切块焯水，炒肉末加豆瓣酱，加豆腐煮，勾芡撒花椒粉。",
			Metadata: map[string]interface{}{
				"name":     "麻婆豆腐",
				"category": "素食",
				"cuisine":  "川菜",
				"type":     "recipe",
			},
		},
		{
			ID: "doc_4",
			Content: "清蒸鲈鱼是粤菜经典菜品，清淡营养。做法：鲈鱼处理干净划刀，用盐料酒腌制，放姜丝蒸8分钟，倒掉水，放葱丝，淋热油和蒸鱼豉油。",
			Metadata: map[string]interface{}{
				"name":     "清蒸鲈鱼",
				"category": "海鲜",
				"cuisine":  "粤菜",
				"type":     "recipe",
			},
		},
		{
			ID: "doc_5",
			Content: "西红柿炒鸡蛋是最经典的家常菜之一。做法：鸡蛋打散炒半熟盛出，炒西红柿出汁，加鸡蛋翻炒，加盐糖调味，撒葱花即可。简单快手，营养丰富。",
			Metadata: map[string]interface{}{
				"name":     "西红柿炒鸡蛋",
				"category": "家常菜",
				"cuisine":  "中式",
				"type":     "recipe",
			},
		},
		{
			ID: "doc_6",
			Content: "水煮鱼是川菜中的经典菜品，麻辣鲜香。做法：草鱼片加蛋清淀粉腌制，豆芽焯水铺底，炒豆瓣酱出红油加水煮开放鱼片，倒盆中，撒辣椒花椒淋热油。",
			Metadata: map[string]interface{}{
				"name":     "水煮鱼",
				"category": "肉类",
				"cuisine":  "川菜",
				"type":     "recipe",
			},
		},
		{
			ID: "doc_7",
			Content: "糖醋排骨是经典酸甜口味菜肴。做法：排骨焯水洗净，炒糖色，下排骨翻炒上色，加调料加水焖30分钟，加醋糖大火收汁，撒芝麻即可。",
			Metadata: map[string]interface{}{
				"name":     "糖醋排骨",
				"category": "肉类",
				"cuisine":  "中式",
				"type":     "recipe",
			},
		},
		{
			ID: "doc_8",
			Content: "鱼香肉丝是川菜经典，酸甜辣口味。做法：里脊肉切丝腌制，木耳胡萝卜切丝，调鱼香汁（糖醋生抽淀粉），炒肉丝加调料，加配菜炒，倒汁炒匀撒葱花。",
			Metadata: map[string]interface{}{
				"name":     "鱼香肉丝",
				"category": "肉类",
				"cuisine":  "川菜",
				"type":     "recipe",
			},
		},
		{
			ID: "doc_9",
			Content: "回锅肉是川菜代表，下饭神器。做法：五花肉煮八成熟切薄片，青椒切块，炒肉片出油，加豆瓣酱豆豉炒香，加青椒炒，加糖调味。",
			Metadata: map[string]interface{}{
				"name":     "回锅肉",
				"category": "肉类",
				"cuisine":  "川菜",
				"type":     "recipe",
			},
		},
		{
			ID: "doc_10",
			Content: "扬州炒饭是经典炒饭菜品。做法：鸡蛋炒熟盛出，火腿切丁，虾仁焯水，炒虾仁火腿胡萝卜豌豆，加米饭翻炒，加鸡蛋，加盐调味，撒葱花。",
			Metadata: map[string]interface{}{
				"name":     "扬州炒饭",
				"category": "主食",
				"cuisine":  "苏菜",
				"type":     "recipe",
			},
		},
	}
}
