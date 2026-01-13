package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type DashscopeEmbedding struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	dimension  int
}

type DashscopeEmbeddingResponse struct {
	Output struct {
		Embeddings []struct {
			TextIndex int       `json:"text_index"`
			Embedding []float32 `json:"embedding"`
		} `json:"embeddings"`
	} `json:"output"`
}

func NewDashscopeEmbedding(config Config) *DashscopeEmbedding {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding"
	}

	model := config.Model
	if model == "" {
		model = "text-embedding-v2"
	}

	return &DashscopeEmbedding{
		apiKey:     config.APIKey,
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
		dimension:  1536,
	}
}

func (e *DashscopeEmbedding) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model": e.model,
		"input": map[string]string{"texts": text},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/text-embedding-sync", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var result DashscopeEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Output.Embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Output.Embeddings[0].Embedding, nil
}

func (e *DashscopeEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	const batchSize = 25
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		reqBody := map[string]interface{}{
			"model": e.model,
			"input": map[string]interface{}{"texts": batch},
		}

		jsonData, _ := json.Marshal(reqBody)
		req, _ := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/text-embedding-sync", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+e.apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var result DashscopeEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		for _, item := range result.Output.Embeddings {
			allEmbeddings = append(allEmbeddings, item.Embedding)
		}
	}

	return allEmbeddings, nil
}

func (e *DashscopeEmbedding) Dimension() int {
	return e.dimension
}
