package main

import (
	"fmt"
	"os"
	"cookrag-go/internal/config"
)

func main() {
	// 加载配置
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("📋 配置信息:")
	fmt.Printf("Neo4j URI: %s\n", cfg.Neo4j.URI)
	fmt.Printf("Neo4j Username: %s\n", cfg.Neo4j.Username)
	fmt.Printf("Neo4j Password: %s\n", cfg.Neo4j.Password)
	fmt.Printf("Neo4j Database: %s\n", cfg.Neo4j.Database)
	fmt.Println("")
	fmt.Printf("Zhipu API Key: %s\n", cfg.Embedding.APIKey[:20]+"...")
}
