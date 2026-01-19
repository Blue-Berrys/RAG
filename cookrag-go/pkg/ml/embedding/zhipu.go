package embedding

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"
)

// ZhipuEmbedding 智谱AI Embedding服务（使用 eino 框架）
// 官网: https://open.bigmodel.cn/
// 文档: https://open.bigmodel.cn/dev/api#embedding
// 使用 OpenAI 兼容接口
type ZhipuEmbedding struct {
	embedder embedding.Embedder
	model    string
	dimension int
}

// NewZhipuEmbedding 创建智谱AI Embedding（使用 eino 框架）
func NewZhipuEmbedding(config Config) *ZhipuEmbedding {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/paas/v4"
	}

	model := config.Model
	if model == "" {
		model = "embedding-2" // 默认模型，1024维
	}

	dimension := 1024
	if model == "embedding-3" {
		dimension = 1024
	}

	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// 使用 eino-ext 的 OpenAI embedding 组件
	// 智谱AI 提供 OpenAI 兼容接口
	embedder, err := openai.NewEmbedder(context.Background(), &openai.EmbeddingConfig{
		APIKey:     config.APIKey,
		BaseURL:    baseURL,
		Model:      model,
		Timeout:    timeout,
		HTTPClient: &http.Client{Timeout: timeout},
		ByAzure:    false, // 使用标准 OpenAI API，不是 Azure
	})
	if err != nil {
		// 如果创建失败，返回一个会返回错误的 embedder
		return &ZhipuEmbedding{
			embedder: nil,
			model:    model,
			dimension: dimension,
		}
	}

	return &ZhipuEmbedding{
		embedder: embedder,
		model:    model,
		dimension: dimension,
	}
}

// Embed 单个文本向量化
func (e *ZhipuEmbedding) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.embedder == nil {
		return nil, fmt.Errorf("embedder not initialized")
	}

	embeddings, err := e.embedder.EmbedStrings(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("embed failed: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	// 转换 []float64 到 []float32
	result := make([]float32, len(embeddings[0]))
	for i, v := range embeddings[0] {
		result[i] = float32(v)
	}

	return result, nil
}

// EmbedBatch 批量向量化
func (e *ZhipuEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if e.embedder == nil {
		return nil, fmt.Errorf("embedder not initialized")
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("empty texts")
	}

	// 智谱支持批量，推荐一次最多10个
	const batchSize = 10
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := e.embedder.EmbedStrings(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embed batch failed: %w", err)
		}

		// 转换 [][]float64 到 [][]float32
		for _, emb := range embeddings {
			result := make([]float32, len(emb))
			for j, v := range emb {
				result[j] = float32(v)
			}
			allEmbeddings = append(allEmbeddings, result)
		}
	}

	return allEmbeddings, nil
}

// Dimension 返回向量维度
func (e *ZhipuEmbedding) Dimension() int {
	return e.dimension
}
