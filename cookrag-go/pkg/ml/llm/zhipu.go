package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

// ZhipuLLM 智谱AI LLM实现
type ZhipuLLM struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// ZhipuRequest 智谱AI请求
type ZhipuRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

// ZhipuResponse 智谱AI响应
type ZhipuResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice 消息选择
type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Message 消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage 使用情况
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewZhipuLLM 创建智谱AI LLM
func NewZhipuLLM(model string) (*ZhipuLLM, error) {
	apiKey := os.Getenv("ZHIPU_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ZHIPU_API_KEY environment variable not set")
	}

	return &ZhipuLLM{
		apiKey:  apiKey,
		baseURL: "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		model:   model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// Generate 生成文本
func (z *ZhipuLLM) Generate(ctx context.Context, prompt string) (string, error) {
	log.Infof("🤖 Zhipu LLM generation: model=%s", z.model)

	req := ZhipuRequest{
		Model: z.model,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", z.baseURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+z.apiKey)

	resp, err := z.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var zResp ZhipuResponse
	if err := json.NewDecoder(resp.Body).Decode(&zResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(zResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	log.Infof("✅ Zhipu LLM generation completed: tokens=%d", zResp.Usage.TotalTokens)
	return zResp.Choices[0].Message.Content, nil
}

// GenerateWithStream 流式生成
func (z *ZhipuLLM) GenerateWithStream(ctx context.Context, prompt string) (<-chan string, error) {
	stream := make(chan string)

	req := ZhipuRequest{
		Model: z.model,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: true,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		close(stream)
		return stream, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", z.baseURL, bytes.NewBuffer(reqBody))
	if err != nil {
		close(stream)
		return stream, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+z.apiKey)

	resp, err := z.client.Do(httpReq)
	if err != nil {
		close(stream)
		return stream, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		close(stream)
		return stream, fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	// 启动goroutine处理流式响应
	go func() {
		defer close(stream)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Warnf("Error reading stream: %v", err)
				}
				break
			}

			// 跳过空行和注释
			lineBytes := []byte(line)
			lineBytes = bytes.TrimSpace(lineBytes)
			if len(lineBytes) == 0 || string(lineBytes[0]) == ":" {
				continue
			}

			// 解析SSE格式
			if bytes.HasPrefix(lineBytes, []byte("data:")) {
				data := bytes.TrimPrefix(lineBytes, []byte("data:"))
				data = bytes.TrimSpace(data)

				if string(data) == "[DONE]" {
					break
				}

				var chunk map[string]interface{}
				if err := json.Unmarshal(data, &chunk); err != nil {
					log.Warnf("Error parsing chunk: %v", err)
					continue
				}

				// 提取内容
				if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]interface{}); ok {
						if delta, ok := choice["delta"].(map[string]interface{}); ok {
							if content, ok := delta["content"].(string); ok {
								stream <- content
							}
						}
					}
				}
			}
		}

		log.Infof("✅ Stream generation completed")
	}()

	return stream, nil
}
