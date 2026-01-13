package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

// ZhipuEmbedding 智谱AI Embedding服务
// 官网: https://open.bigmodel.cn/
// 文档: https://open.bigmodel.cn/dev/api#embedding
// 目前完全免费，推荐使用！
type ZhipuEmbedding struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	dimension  int
}

type ZhipuEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

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

	return &ZhipuEmbedding{
		apiKey:     config.APIKey,
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
		dimension:  dimension,
	}
}

func (e *ZhipuEmbedding) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model": e.model,
		"input": []string{text},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		e.baseURL+"/embeddings",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result ZhipuEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Data[0].Embedding, nil
}

func (e *ZhipuEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
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

		reqBody := map[string]interface{}{
			"model": e.model,
			"input": batch,
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("marshal request failed: %w", err)
		}

		req, err := http.NewRequestWithContext(
			ctx,
			"POST",
			e.baseURL+"/embeddings",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			return nil, fmt.Errorf("create request failed: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+e.apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}

		var result ZhipuEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode response failed: %w", err)
		}

		// 按index排序
		sort.Slice(result.Data, func(i, j int) bool {
			return result.Data[i].Index < result.Data[j].Index
		})

		for _, item := range result.Data {
			allEmbeddings = append(allEmbeddings, item.Embedding)
		}
	}

	return allEmbeddings, nil
}

func (e *ZhipuEmbedding) Dimension() int {
	return e.dimension
}
