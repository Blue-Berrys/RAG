package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/models"
	"cookrag-go/internal/observability"
)

// Provider LLM提供者接口
type Provider interface {
	Generate(ctx context.Context, prompt string) (string, error)
	GenerateWithStream(ctx context.Context, prompt string) (<-chan string, error)
}

// Generator LLM生成器
type Generator struct {
	provider Provider
}

// NewGenerator 创建生成器
func NewGenerator(provider Provider) *Generator {
	return &Generator{
		provider: provider,
	}
}

// GenerateAnswer 生成答案
func (g *Generator) GenerateAnswer(ctx context.Context, query string, documents []models.Document) (string, error) {
	// 创建链路追踪 span
	span := observability.GlobalTracer.StartSpan(ctx, "llm_generate_answer", map[string]interface{}{
		"query":          query,
		"doc_count":      len(documents),
		"provider":       "llm",
	})
	defer span.End()

	startTime := time.Now()

	log.Infof("🤖 Generating answer for query: %s", query)
	log.Infof("📚 Using %d context documents", len(documents))

	// 构建上下文
	context := g.buildContext(documents)

	// 构建提示词
	prompt := g.buildPrompt(query, context)

	// 调用LLM生成
	answer, err := g.provider.Generate(ctx, prompt)
	if err != nil {
		span.SetError(err)
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	latency := float64(time.Since(startTime).Milliseconds())
	span.AddMetadata("latency_ms", latency)
	span.AddMetadata("answer_length", len(answer))
	span.AddMetadata("prompt_length", len(prompt))

	log.Infof("✅ Answer generated successfully")
	return answer, nil
}

// GenerateAnswerWithStream 流式生成答案
func (g *Generator) GenerateAnswerWithStream(ctx context.Context, query string, documents []models.Document) (<-chan string, error) {
	log.Infof("🤖 Generating streaming answer for query: %s", query)

	// 构建上下文
	context := g.buildContext(documents)

	// 构建提示词
	prompt := g.buildPrompt(query, context)

	// 调用LLM流式生成
	stream, err := g.provider.GenerateWithStream(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM stream generation failed: %w", err)
	}

	return stream, nil
}

// buildContext 构建上下文
func (g *Generator) buildContext(documents []models.Document) string {
	if len(documents) == 0 {
		return "没有找到相关文档。"
	}

	context := "参考文档：\n\n"
	for i, doc := range documents {
		context += fmt.Sprintf("[文档%d] %s\n", i+1, doc.Content)
		if doc.Metadata != nil {
			if source, ok := doc.Metadata["source"].(string); ok {
				context += fmt.Sprintf("来源: %s\n", source)
			}
		}
		context += "\n"
	}

	return context
}

// buildPrompt 构建提示词
func (g *Generator) buildPrompt(query string, context string) string {
	prompt := fmt.Sprintf(`你是一个专业的问答助手。请根据以下参考文档回答用户的问题。

参考文档：
%s

问题：%s

要求：
1. 基于参考文档回答问题
2. 如果参考文档中没有相关信息，请明确说明
3. 回答要准确、简洁、易懂
4. 必要时可以引用参考文档中的具体内容

回答：`, context, query)

	return prompt
}
