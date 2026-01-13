package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// QianfanEmbedding 百度千帆Embedding服务
type QianfanEmbedding struct {
	apiKey      string
	secretKey   string
	accessToken string
	tokenExpiry time.Time
	baseURL     string
	httpClient  *http.Client
	mu          sync.RWMutex
	dimension   int
}

type QianfanTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type QianfanEmbeddingResponse struct {
	Data struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func NewQianfanEmbedding(config Config) *QianfanEmbedding {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop"
	}

	return &QianfanEmbedding{
		apiKey:     config.APIKey,
		secretKey:  config.SecretKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
		dimension:  384, // 百度默认384维
	}
}

func (e *QianfanEmbedding) getAccessToken(ctx context.Context) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.accessToken != "" && time.Now().Before(e.tokenExpiry) {
		return e.accessToken, nil
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://aip.baidubce.com/oauth/2.0/token?grant_type=client_credentials",
		nil,
	)
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(e.apiKey, e.secretKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result QianfanTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	e.accessToken = result.AccessToken
	e.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	return e.accessToken, nil
}

func (e *QianfanEmbedding) Embed(ctx context.Context, text string) ([]float32, error) {
	token, err := e.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token failed: %w", err)
	}

	reqBody := map[string]string{"input": text}
	jsonData, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("%s/embedding?access_token=%s", e.baseURL, token)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result QianfanEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data.Embedding, nil
}

func (e *QianfanEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, err := e.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed failed at index %d: %w", i, err)
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

func (e *QianfanEmbedding) Dimension() int {
	return e.dimension
}
