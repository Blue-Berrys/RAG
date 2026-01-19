package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Embedding  EmbeddingConfig  `mapstructure:"embedding"`
	Milvus     MilvusConfig     `mapstructure:"milvus"`
	Neo4j      Neo4jConfig      `mapstructure:"neo4j"`
	Redis      RedisConfig      `mapstructure:"redis"`
	LLM        LLMConfig        `mapstructure:"llm"`
	Observability ObservabilityConfig `mapstructure:"observability"`
}

type ServerConfig struct {
	Port         string `mapstructure:"port"`
	Mode         string `mapstructure:"mode"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

type EmbeddingConfig struct {
	Provider   string `mapstructure:"provider"`
	APIKey     string `mapstructure:"api_key"`
	SecretKey  string `mapstructure:"secret_key"`
	Model      string `mapstructure:"model"`
	BaseURL    string `mapstructure:"base_url"`
	Timeout    int    `mapstructure:"timeout"`
}

type MilvusConfig struct {
	Host           string `mapstructure:"host"`
	Port           string `mapstructure:"port"`
	Username       string `mapstructure:"username"`
	Password       string `mapstructure:"password"`
	Database       string `mapstructure:"database"`
	CollectionName string `mapstructure:"collection_name"`
	Dimension      int    `mapstructure:"dimension"`
	IndexType      string `mapstructure:"index_type"`
	MetricType     string `mapstructure:"metric_type"`
}

type Neo4jConfig struct {
	URI      string `mapstructure:"uri"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type LLMConfig struct {
	Provider    string `mapstructure:"provider"`
	Model       string `mapstructure:"model"`
	APIKey      string `mapstructure:"api_key"`
	Temperature float64 `mapstructure:"temperature"`
	MaxTokens   int    `mapstructure:"max_tokens"`
	Timeout     int    `mapstructure:"timeout"`
}

type ObservabilityConfig struct {
	EnableTracing    bool   `mapstructure:"enable_tracing"`
	EnableMetrics    bool   `mapstructure:"enable_metrics"`
	PrometheusPort   int    `mapstructure:"prometheus_port"`
	LogLevel         string `mapstructure:"log_level"`
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置配置文件
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// 环境变量支持
	v.AutomaticEnv()
	v.SetEnvPrefix("COOKRAG")

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	 // 解析配置
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 从环境变量加载敏感信息
	// 支持 ${VAR} 和 $VAR 两种格式
	getEnvValue := func(s string) string {
		if len(s) == 0 {
			return s
		}
		// 检查 ${VAR} 格式
		if len(s) > 2 && s[0] == '$' && s[1] == '{' && s[len(s)-1] == '}' {
			envName := s[2 : len(s)-1]
			if val := os.Getenv(envName); val != "" {
				return val
			}
		}
		// 检查 $VAR 格式
		if s[0] == '$' {
			envName := s[1:]
			if val := os.Getenv(envName); val != "" {
				return val
			}
		}
		return s
	}

	config.Embedding.APIKey = getEnvValue(config.Embedding.APIKey)
	config.LLM.APIKey = getEnvValue(config.LLM.APIKey)
	config.Neo4j.Username = getEnvValue(config.Neo4j.Username)
	config.Neo4j.Password = getEnvValue(config.Neo4j.Password)
	config.Redis.Password = getEnvValue(config.Redis.Password)

	return &config, nil
}
