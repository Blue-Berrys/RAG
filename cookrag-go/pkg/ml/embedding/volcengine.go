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

type VolcengineEmbedding struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	dimension  int
}

type VolcengineEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

func NewVolcengineEmbedding(config Config) *VolcengineEmbedding {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://ark.cn-beijing.volces.com/api/v3"
	}

	return &VolcengineEmbedding{
		apiKey:     config.APIKey,
		baseURL:    baseURL,
		model:      config.Model,
		httpClient: &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
		dimension:  1024,
	}
}

func (e *VolcengineEmbedding) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model": e.model,
		"input": []string{text},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewBuffer(jsonData))
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

	var result VolcengineEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Data[0].Embedding, nil
}

func (e *VolcengineEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	const batchSize = 100
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		reqBody := map[string]interface{}{"model": e.model, "input": batch}
		jsonData, _ := json.Marshal(reqBody)

		req, _ := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+e.apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var result VolcengineEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		sort.Slice(result.Data, func(i, j int) bool {
			return result.Data[i].Index < result.Data[j].Index
		})

		for _, item := range result.Data {
			allEmbeddings = append(allEmbeddings, item.Embedding)
		}
	}

	return allEmbeddings, nil
}

func (e *VolcengineEmbedding) Dimension() int {
	return e.dimension
}
