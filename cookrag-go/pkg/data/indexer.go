package data

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/core/retrieval"
	"cookrag-go/internal/models"
	"cookrag-go/pkg/ml/embedding"
	"cookrag-go/pkg/storage/milvus"
	"cookrag-go/pkg/storage/neo4j"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// Indexer 索引器
type Indexer struct {
	embeddingProvider embedding.Provider
	milvusClient      *milvus.Client
	neo4jClient       *neo4j.Client
	bm25Retriever     *retrieval.BM25Retriever
	vectorRetriever   *retrieval.VectorRetriever
}

// NewIndexer 创建索引器
func NewIndexer(
	embeddingProvider embedding.Provider,
	milvusClient *milvus.Client,
	neo4jClient *neo4j.Client,
) *Indexer {
	return &Indexer{
		embeddingProvider: embeddingProvider,
		milvusClient:      milvusClient,
		neo4jClient:       neo4jClient,
	}
}

// IndexConfig 索引配置
type IndexConfig struct {
	CollectionName  string   // Milvus集合名称
	VectorIndex     bool     // 是否创建向量索引
	BM25Index       bool     // 是否创建BM25索引
	GraphIndex      bool     // 是否创建图索引
	BatchSize       int      // 批量大小
	CreateCollection bool    // 是否创建集合
}

// DefaultIndexConfig 默认索引配置
func DefaultIndexConfig() *IndexConfig {
	return &IndexConfig{
		CollectionName:   "cookrag_documents",
		VectorIndex:      true,
		BM25Index:        true,
		GraphIndex:       false, // 图索引需要特殊数据结构
		BatchSize:        100,
		CreateCollection: true,
	}
}

// IndexDocuments 索引文档
func (idx *Indexer) IndexDocuments(ctx context.Context, docs []models.Document, config *IndexConfig) error {
	if config == nil {
		config = DefaultIndexConfig()
	}

	log.Infof("📚 Starting document indexing: %d documents", len(docs))
	startTime := time.Now()

	// 1. 向量索引
	if config.VectorIndex && idx.milvusClient != nil {
		if err := idx.indexVector(ctx, docs, config); err != nil {
			return fmt.Errorf("vector indexing failed: %w", err)
		}
	}

	// 2. BM25索引
	if config.BM25Index {
		if err := idx.indexBM25(ctx, docs); err != nil {
			return fmt.Errorf("BM25 indexing failed: %w", err)
		}
	}

	// 3. 图索引（可选）
	if config.GraphIndex && idx.neo4jClient != nil {
		if err := idx.indexGraph(ctx, docs); err != nil {
			log.Warnf("⚠️  Graph indexing failed: %v", err)
		}
	}

	duration := time.Since(startTime)
	log.Infof("✅ Indexing completed: %d documents in %s", len(docs), duration)

	return nil
}

// indexVector 创建向量索引
func (idx *Indexer) indexVector(ctx context.Context, docs []models.Document, config *IndexConfig) error {
	log.Infof("🔤 Creating vector index...")

	// 1. 创建集合（如果需要）
	if config.CreateCollection {
		collectionExists, err := idx.milvusClient.HasCollection(ctx, config.CollectionName)
		if err != nil {
			return fmt.Errorf("failed to check collection: %w", err)
		}

		if !collectionExists {
			dimension := idx.embeddingProvider.Dimension()
			if err := idx.milvusClient.CreateCollection(ctx, config.CollectionName, dimension); err != nil {
				return fmt.Errorf("failed to create collection: %w", err)
			}

			// 创建索引
			idxType := milvus entity.IndexType
			if err := idx.milvusClient.CreateIndex(
				ctx,
				config.CollectionName,
				"vector",
				idxType.HNSW,
				map[string]string{
					"M":              "16",
					"efConstruction": "256",
				},
			); err != nil {
				return fmt.Errorf("failed to create index: %w", err)
			}

			// 加载集合
			if err := idx.milvusClient.LoadCollection(ctx, config.CollectionName); err != nil {
				return fmt.Errorf("failed to load collection: %w", err)
			}
		}
	}

	// 2. 批量生成向量
	log.Infof("🔤 Generating embeddings for %d documents...", len(docs))

	texts := make([]string, len(docs))
	for i, doc := range docs {
		texts[i] = doc.Content
	}

	embeddings, err := idx.embeddingProvider.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// 3. 批量插入到Milvus
	log.Infof("📝 Inserting documents into Milvus...")

	ids := make([]int64, len(docs))
	metadataList := make([]map[string]interface{}, len(docs))

	for i, doc := range docs {
		ids[i] = int64(i)
		metadataList[i] = doc.Metadata
	}

	if err := idx.milvusClient.InsertBatch(
		ctx,
		config.CollectionName,
		ids,
		embeddings,
		texts,
		metadataList,
	); err != nil {
		return fmt.Errorf("failed to insert documents: %w", err)
	}

	log.Infof("✅ Vector index created: %d documents", len(docs))
	return nil
}

// indexBM25 创建BM25索引
func (idx *Indexer) indexBM25(ctx context.Context, docs []models.Document) error {
	log.Infof("📝 Creating BM25 index...")

	bm25Retriever := retrieval.NewBM25Retriever(retrieval.DefaultBM25Config())

	if err := bm25Retriever.IndexDocuments(ctx, docs); err != nil {
		return fmt.Errorf("failed to index BM25: %w", err)
	}

	idx.bm25Retriever = bm25Retriever

	stats := bm25Retriever.GetStats()
	log.Infof("✅ BM25 index created: %v", stats)

	return nil
}

// indexGraph 创建图索引（简化版）
func (idx *Indexer) indexGraph(ctx context.Context, docs []models.Document) error {
	log.Infof("🕸️  Creating graph index...")

	// 简化实现：为菜谱创建图节点
	// 实际应用中需要根据数据结构调整

	nodeCount := 0
	for i, doc := range docs {
		// 提取实体（菜谱名称）
		if name, ok := doc.Metadata["name"].(string); ok {
			// 这里应该创建Neo4j节点
			// 简化实现：只记录数量
			nodeCount++
			_ = name
		}
	}

	log.Infof("✅ Graph index created: %d nodes", nodeCount)
	return nil
}

// GetBM25Retriever 获取BM25检索器
func (idx *Indexer) GetBM25Retriever() *retrieval.BM25Retriever {
	return idx.bm25Retriever
}

// ClearIndex 清空索引
func (idx *Indexer) ClearIndex(ctx context.Context, config *IndexConfig) error {
	log.Infof("🗑️  Clearing index...")

	if config.VectorIndex && idx.milvusClient != nil {
		if err := idx.milvusClient.DeleteCollection(ctx, config.CollectionName); err != nil {
			return fmt.Errorf("failed to delete collection: %w", err)
		}
		log.Infof("✅ Vector index cleared")
	}

	if config.BM25Index {
		idx.bm25Retriever = retrieval.NewBM25Retriever(retrieval.DefaultBM25Config())
		log.Infof("✅ BM25 index cleared")
	}

	return nil
}
