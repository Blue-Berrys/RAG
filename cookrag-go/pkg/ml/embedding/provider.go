package embedding

import (
	"context"
	"fmt"
)

// Provider Embedding服务提供商接口
type Provider interface {
	// Embed 单个文本向量化
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch 批量向量化（推荐，更高效）
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension 返回向量维度
	Dimension() int
}

// Config Embedding配置
type Config struct {
	Provider  string `yaml:"provider"`   // zhipu, qianfan, dashscope, volcengine
	APIKey    string `yaml:"api_key"`
	SecretKey string `yaml:"secret_key"` // 百度需要
	Model     string `yaml:"model"`
	BaseURL   string `yaml:"base_url"`
	Timeout   int    `yaml:"timeout"` // 超时时间（秒）
}

// NewProvider 创建Embedding Provider
func NewProvider(config Config) (Provider, error) {
	if config.Timeout == 0 {
		config.Timeout = 30
	}

	switch config.Provider {
	case "zhipu":
		return NewZhipuEmbedding(config), nil
	case "qianfan":
		return NewQianfanEmbedding(config), nil
	case "dashscope":
		return NewDashscopeEmbedding(config), nil
	case "volcengine":
		return NewVolcengineEmbedding(config), nil
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s, supported: zhipu, qianfan, dashscope, volcengine", config.Provider)
	}
}
