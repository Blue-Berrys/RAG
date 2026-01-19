package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/config"
	"cookrag-go/internal/kg"
	"cookrag-go/pkg/storage/neo4j"
)

func main() {
	log.SetLevel(log.InfoLevel)
	log.SetReportTimestamp(true)
	log.SetTimeFormat(time.Kitchen)

	log.Infof("🕸️  CookRAG Knowledge Graph Builder")

	// 1. 加载配置
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}
	log.Infof("✅ Config loaded")

	// 2. 连接 Neo4j
	neo4jClient, err := neo4j.NewClient(
		cfg.Neo4j.URI,
		cfg.Neo4j.Username,
		cfg.Neo4j.Password,
		cfg.Neo4j.Database,
	)
	if err != nil {
		log.Warnf("⚠️  Failed to connect to Neo4j: %v", err)
		log.Infof("⚠️  Continuing without graph indexing...")
		neo4jClient = nil
	} else {
		log.Infof("✅ Connected to Neo4j")
		defer neo4jClient.Close(context.Background())
	}

	if neo4jClient == nil {
		log.Fatalf("❌ Neo4j connection is required for graph building")
	}

	// 3. 清空现有图谱（可选）
	// neo4jClient.ClearGraph(context.Background())

	// 4. 加载文档
	docsDir := "docs/dishes"
	if len(os.Args) > 1 {
		docsDir = os.Args[1]
	}

	log.Infof("📚 Loading documents from: %s", docsDir)
	documents, err := loadDocumentsFromDir(docsDir)
	if err != nil {
		log.Fatalf("❌ Failed to load documents: %v", err)
	}
	log.Infof("✅ Loaded %d documents", len(documents))

	// 5. 构建知识图谱
	builder := kg.NewGraphBuilder(neo4jClient)

	stats, err := builder.BuildFromDocuments(context.Background(), documents)
	if err != nil {
		log.Fatalf("❌ Failed to build graph: %v", err)
	}

	// 6. 打印统计
	log.Infof("\n📊 Build Summary:")
	log.Infof("   Dishes:      %d", stats.TotalDishes)
	log.Infof("   Ingredients: %d", stats.TotalIngredients)
	log.Infof("   Categories:  %d", stats.TotalCategories)
	log.Infof("   Relations:   %d", stats.TotalRelations)
	log.Infof("   Duration:    %v", stats.BuildDuration)

	log.Infof("\n✅ Knowledge graph built successfully!")
}

func loadDocumentsFromDir(dir string) ([]kg.Document, error) {
	var documents []kg.Document

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 只处理 .md 文件
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		// 读取文件内容
		content, err := os.ReadFile(path)
		if err != nil {
			log.Warnf("⚠️  Failed to read file %s: %v", path, err)
			return nil
		}

		// 提取相对路径
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// 提取分类和菜名
		// 路径格式: category/subdir/dish.md 或 category/dish.md
		parts := strings.Split(relPath, string(filepath.Separator))
		var category, dishName string

		if len(parts) >= 2 {
			// 最后一个部分是文件名
			filename := parts[len(parts)-1]
			dishName = strings.TrimSuffix(filename, ".md")

			// 倒数第二个部分可能是子目录或分类
			if len(parts) >= 3 {
				// 有子目录的情况: vegetable_dish/西红柿豆腐汤羹/西红柿豆腐汤羹.md
				category = parts[0]
			} else {
				// 没有子目录的情况: vegetable_dish/皮蛋豆腐.md
				category = parts[0]
			}
		}

		documents = append(documents, kg.Document{
			Content:  string(content),
			Category: category,
			DishName: dishName,
		})

		return nil
	})

	return documents, err
}
