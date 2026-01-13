package data

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/models"
)

// DocumentLoader 文档加载器接口
type DocumentLoader interface {
	Load(ctx context.Context) ([]models.Document, error)
}

// JSONLoader JSON文件加载器
type JSONLoader struct {
	filePath string
}

// NewJSONLoader 创建JSON加载器
func NewJSONLoader(filePath string) *JSONLoader {
	return &JSONLoader{
		filePath: filePath,
	}
}

// Load 加载JSON文档
func (l *JSONLoader) Load(ctx context.Context) ([]models.Document, error) {
	log.Infof("📄 Loading JSON documents from: %s", l.filePath)

	file, err := os.Open(l.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var docs []models.Document
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&docs); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	log.Infof("✅ Loaded %d documents from JSON", len(docs))
	return docs, nil
}

// TextLoader 文本文件加载器
type TextLoader struct {
	directory string
	ext       []string
}

// NewTextLoader 创建文本加载器
func NewTextLoader(directory string, ext []string) *TextLoader {
	return &TextLoader{
		directory: directory,
		ext:       ext,
	}
}

// Load 加载文本文件
func (l *TextLoader) Load(ctx context.Context) ([]models.Document, error) {
	log.Infof("📄 Loading text files from: %s", l.directory)

	var docs []models.Document

	err := filepath.Walk(l.directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 检查文件扩展名
		if !l.matchesExt(path) {
			return nil
		}

		// 读取文件
		content, err := os.ReadFile(path)
		if err != nil {
			log.Warnf("⚠️  Failed to read file %s: %v", path, err)
			return nil
		}

		doc := models.Document{
			ID:      generateDocID(path),
			Content: string(content),
			Metadata: map[string]interface{}{
				"source": path,
				"size":   info.Size(),
				"mod_time": info.ModTime(),
			},
		}

		docs = append(docs, doc)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	log.Infof("✅ Loaded %d text files", len(docs))
	return docs, nil
}

// matchesExt 检查文件扩展名
func (l *TextLoader) matchesExt(path string) bool {
	if len(l.ext) == 0 {
		return true
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range l.ext {
		if strings.ToLower(e) == ext {
			return true
		}
	}
	return false
}

// CSLoader CSV文件加载器
type CSVLoader struct {
	filePath   string
	contentCol int    // 内容列索引
	metaCols   []int  // 元数据列索引
	hasHeader  bool   // 是否有表头
}

// NewCSVLoader 创建CSV加载器
func NewCSVLoader(filePath string, contentCol int, metaCols []int, hasHeader bool) *CSVLoader {
	return &CSVLoader{
		filePath:   filePath,
		contentCol: contentCol,
		metaCols:   metaCols,
		hasHeader:  hasHeader,
	}
}

// Load 加载CSV文档
func (l *CSVLoader) Load(ctx context.Context) ([]models.Document, error) {
	log.Infof("📄 Loading CSV documents from: %s", l.filePath)

	file, err := os.Open(l.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','

	var docs []models.Document
	rowNum := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV: %w", err)
		}

		rowNum++

		// 跳过表头
		if l.hasHeader && rowNum == 1 {
			continue
		}

		// 检查内容列索引
		if l.contentCol >= len(record) {
			log.Warnf("⚠️  Row %d: content_col %d out of range", rowNum, l.contentCol)
			continue
		}

		doc := models.Document{
			ID:      fmt.Sprintf("csv_row_%d", rowNum),
			Content: record[l.contentCol],
			Metadata: map[string]interface{}{
				"row": rowNum,
				"source": l.filePath,
			},
		}

		// 添加元数据列
		for i, colIdx := range l.metaCols {
			if colIdx < len(record) {
				key := fmt.Sprintf("meta_%d", i)
				doc.Metadata[key] = record[colIdx]
			}
		}

		docs = append(docs, doc)
	}

	log.Infof("✅ Loaded %d documents from CSV", len(docs))
	return docs, nil
}

// RecipeLoader 菜谱数据加载器（专门用于本项目）
type RecipeLoader struct {
	filePath string
}

// Recipe 菜谱数据结构
type Recipe struct {
	Name        string   `json:"name"`
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
	Category    string   `json:"category"`
	Cuisine     string   `json:"cuisine"`
	Tags        []string `json:"tags"`
}

// NewRecipeLoader 创建菜谱加载器
func NewRecipeLoader(filePath string) *RecipeLoader {
	return &RecipeLoader{
		filePath: filePath,
	}
}

// Load 加载菜谱数据
func (l *RecipeLoader) Load(ctx context.Context) ([]models.Document, error) {
	log.Infof("📖 Loading recipes from: %s", l.filePath)

	file, err := os.Open(l.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var recipes []Recipe
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&recipes); err != nil {
		return nil, fmt.Errorf("failed to decode recipes: %w", err)
	}

	// 转换为文档
	docs := make([]models.Document, 0, len(recipes))
	for i, recipe := range recipes {
		// 构建内容
		content := fmt.Sprintf("菜名：%s\n\n食材：\n%s\n\n步骤：\n%s",
			recipe.Name,
			strings.Join(recipe.Ingredients, "\n"),
			strings.Join(recipe.Steps, "\n"))

		doc := models.Document{
			ID:      fmt.Sprintf("recipe_%d", i),
			Content: content,
			Metadata: map[string]interface{}{
				"name":        recipe.Name,
				"category":    recipe.Category,
				"cuisine":     recipe.Cuisine,
				"tags":        recipe.Tags,
				"ingredients": recipe.Ingredients,
				"type":        "recipe",
			},
		}

		docs = append(docs, doc)
	}

	log.Infof("✅ Loaded %d recipes", len(recipes))
	return docs, nil
}

// generateDocID 生成文档ID
func generateDocID(path string) string {
	// 简单的ID生成：使用文件路径的hash
	return fmt.Sprintf("doc_%x", len(path))
}
