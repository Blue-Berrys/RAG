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
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// 智谱AI API Key格式验证正则
// 格式: id.secret (例如: 1234567890.abcdefGHIJKL1234567890)
var zhipuAPIKeyPattern = regexp.MustCompile(`^[a-z0-9]+\.[a-zA-Z0-9]{40,}$`)

// validateAPIKey 验证API Key格式
func validateAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// 去除前后空格
	apiKey = strings.TrimSpace(apiKey)

	// 检查基本长度（智谱AI的API Key通常至少50字符）
	if len(apiKey) < 50 {
		return fmt.Errorf("API key format invalid: too short (expected at least 50 characters, got %d)", len(apiKey))
	}

	// 验证格式是否符合 id.secret 模式
	if !zhipuAPIKeyPattern.MatchString(apiKey) {
		return fmt.Errorf("API key format invalid: expected format 'id.secret' with dot separator")
	}

	return nil
}

// maskAPIKey 掩码API Key用于日志输出
// 示例: 1234567890.abcdefghijkl...WXYZ1234
func maskAPIKey(apiKey string) string {
	if apiKey == "" {
		return "<empty>"
	}

	parts := strings.Split(apiKey, ".")
	if len(parts) != 2 {
		// 如果格式不对，只显示前4位和后4位
		if len(apiKey) <= 8 {
			return "***"
		}
		return apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
	}

	// 显示ID部分的前4位，secret部分只显示前4位和后4位
	id := parts[0]
	secret := parts[1]

	if len(id) > 4 {
		id = id[:4] + "***"
	}
	if len(secret) > 8 {
		secret = secret[:4] + "..." + secret[len(secret)-4:]
	}

	return id + "." + secret
}

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
// 从环境变量 ZHIPU_API_KEY 安全加载API密钥并进行格式验证
func NewZhipuLLM(model string) (*ZhipuLLM, error) {
	apiKey := os.Getenv("ZHIPU_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ZHIPU_API_KEY environment variable not set")
	}

	// 去除前后空格
	apiKey = strings.TrimSpace(apiKey)

	// 验证API Key格式
	if err := validateAPIKey(apiKey); err != nil {
		return nil, fmt.Errorf("invalid ZHIPU_API_KEY: %w", err)
	}

	log.Infof("🔐 Zhipu AI API Key loaded: %s (validated)", maskAPIKey(apiKey))

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
			// 检查context是否已取消，防止goroutine泄漏
			select {
			case <-ctx.Done():
				log.Infof("⚠️  Stream generation cancelled by context")
				return
			default:
				// 继续处理
			}

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
								select {
								case stream <- content:
									// 成功发送
								case <-ctx.Done():
									// context已取消，退出
									return
								}
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
