package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"cookrag-go/internal/observability"
)

// ZhipuLLM 智谱AI LLM实现（使用 eino 框架）
type ZhipuLLM struct {
	chatModel model.ChatModel
	model     string
}

// NewZhipuLLM 创建智谱AI LLM（使用 eino 框架）
func NewZhipuLLM(model string) (*ZhipuLLM, error) {
	if model == "" {
		model = "glm-4-flash"
	}

	// 从环境变量获取 API Key
	apiKey := os.Getenv("ZHIPU_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ZHIPU_API_KEY environment variable not set")
	}

	// 使用 eino-ext 的 OpenAI ChatModel 组件
	// 智谱AI 提供 OpenAI 兼容接口
	chatModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		APIKey:     apiKey,
		BaseURL:    "https://open.bigmodel.cn/api/paas/v4",
		Model:      model,
		Timeout:    60 * time.Second,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
		ByAzure:    false,
	})
	if err != nil {
		return nil, fmt.Errorf("create chat model failed: %w", err)
	}

	return &ZhipuLLM{
		chatModel: chatModel,
		model:     model,
	}, nil
}

// Generate 生成文本
func (z *ZhipuLLM) Generate(ctx context.Context, prompt string) (string, error) {
	// 创建链路追踪 span
	span := observability.GlobalTracer.StartSpan(ctx, "zhipu_llm_generate", map[string]interface{}{
		"model":         z.model,
		"prompt_length": len(prompt),
	})
	defer span.End()

	startTime := time.Now()

	log.Infof("🤖 Zhipu LLM generation: model=%s", z.model)

	// 将 prompt 转换为 eino 的 Message 格式
	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	// 调用 eino 生成
	response, err := z.chatModel.Generate(ctx, messages)
	if err != nil {
		span.SetError(err)
		return "", fmt.Errorf("generate failed: %w", err)
	}

	if response == nil {
		err := fmt.Errorf("no response returned")
		span.SetError(err)
		return "", err
	}

	latency := float64(time.Since(startTime).Milliseconds())
	span.AddMetadata("latency_ms", latency)
	if response != nil && response.Content != "" {
		span.AddMetadata("response_length", len(response.Content))
	}

	log.Infof("✅ Zhipu LLM generation completed")
	return response.Content, nil
}

// GenerateWithStream 流式生成
func (z *ZhipuLLM) GenerateWithStream(ctx context.Context, prompt string) (<-chan string, error) {
	// 创建链路追踪 span
	span := observability.GlobalTracer.StartSpan(ctx, "zhipu_llm_stream", map[string]interface{}{
		"model":         z.model,
		"prompt_length": len(prompt),
	})
	defer span.End()

	stream := make(chan string, 10)

	// 将 prompt 转换为 eino 的 Message 格式
	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	// 调用 eino 流式生成
	streamReader, err := z.chatModel.Stream(ctx, messages)
	if err != nil {
		span.SetError(err)
		close(stream)
		return stream, fmt.Errorf("stream generation failed: %w", err)
	}

	// 启动 goroutine 处理流式响应
	go func() {
		defer close(stream)
		defer streamReader.Close()

		chunkCount := 0
		totalLength := 0

		for {
			chunk, err := streamReader.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Warnf("Error reading stream: %v", err)
				break
			}

			if chunk != nil && chunk.Content != "" {
				stream <- chunk.Content
				chunkCount++
				totalLength += len(chunk.Content)
			}
		}

		span.AddMetadata("chunk_count", chunkCount)
		span.AddMetadata("total_length", totalLength)

		log.Infof("✅ Stream generation completed")
	}()

	return stream, nil
}
