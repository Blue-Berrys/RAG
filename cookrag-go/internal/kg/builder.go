package kg

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/pkg/storage/neo4j"
)

// GraphBuilder 图谱构建器
type GraphBuilder struct {
	neo4jClient *neo4j.Client
	extractor   *RecipeExtractor
	stats       *BuildStats
}

// BuildStats 构建统计
type BuildStats struct {
	TotalDishes       int
	TotalIngredients  int
	TotalCategories   int
	TotalRelations    int
	BuildDuration     time.Duration
}

// NewGraphBuilder 创建图谱构建器
func NewGraphBuilder(neo4jClient *neo4j.Client) *GraphBuilder {
	return &GraphBuilder{
		neo4jClient: neo4jClient,
		extractor:   NewRecipeExtractor(),
		stats:       &BuildStats{},
	}
}

// BuildFromDocuments 从文档构建知识图谱
func (b *GraphBuilder) BuildFromDocuments(ctx context.Context, documents []Document) (*BuildStats, error) {
	startTime := time.Now()
	log.Infof("🕸️  Starting knowledge graph construction from %d documents", len(documents))

	// 清空现有图谱（可选）
	// b.clearGraph(ctx)

	// 批量创建索引
	b.createIndexes(ctx)

	totalEntities := make(map[string]*Entity)
	totalRelations := make([]Relation, 0)

	// 1. 提取所有文档的实体和关系
	for i, doc := range documents {
		if (i+1)%50 == 0 {
			log.Infof("📊 Processing %d/%d documents...", i+1, len(documents))
		}

		extracted := b.extractor.ExtractFromRecipe(
			doc.Content,
			doc.Category,
			doc.DishName,
		)

		// 合并实体（去重）
		for _, entity := range extracted.Entities {
			key := fmt.Sprintf("%s_%s", entity.Type, entity.Name)
			if existing, ok := totalEntities[key]; !ok {
				totalEntities[key] = &entity
			} else {
				// 更新现有实体的属性
				for k, v := range entity.Properties {
					existing.Properties[k] = v
				}
			}
		}

		// 收集关系
		totalRelations = append(totalRelations, extracted.Relations...)
	}

	// 2. 创建所有实体
	log.Infof("🔨 Creating %d unique entities...", len(totalEntities))
	entityIDs := make(map[string]string)  // entity.ID -> Neo4j node ID
	for _, entity := range totalEntities {
		nodeID, err := b.neo4jClient.CreateNode(ctx, string(entity.Type), entity.Name, entity.Properties)
		if err != nil {
			log.Warnf("⚠️  Failed to create node %s: %v", entity.Name, err)
			continue
		}
		// 使用实体的原始ID作为key（如dish_xxx, ing_xxx）
		entityIDs[entity.ID] = nodeID

		// 统计
		switch entity.Type {
		case EntityDish:
			b.stats.TotalDishes++
		case EntityIngredient:
			b.stats.TotalIngredients++
		case EntityCategory:
			b.stats.TotalCategories++
		}
	}

	// 3. 创建所有关系
	log.Infof("🔗 Creating %d relations...", len(totalRelations))
	for _, relation := range totalRelations {
		// 关系的From/To已经是完整的ID（dish_xxx, ing_xxx等）
		// 直接使用
		fromID, fromOK := entityIDs[relation.From]
		toID, toOK := entityIDs[relation.To]

		if !fromOK || !toOK {
			continue
		}

		err := b.neo4jClient.CreateRelation(ctx, fromID, toID, string(relation.Type), relation.Properties)
		if err != nil {
			log.Warnf("⚠️  Failed to create relation %s->%s: %v", relation.From, relation.To, err)
			continue
		}
		b.stats.TotalRelations++
	}

	b.stats.BuildDuration = time.Since(startTime)

	log.Infof("✅ Knowledge graph built successfully!")
	log.Infof("   📊 Stats:")
	log.Infof("      - Dishes: %d", b.stats.TotalDishes)
	log.Infof("      - Ingredients: %d", b.stats.TotalIngredients)
	log.Infof("      - Categories: %d", b.stats.TotalCategories)
	log.Infof("      - Relations: %d", b.stats.TotalRelations)
	log.Infof("      - Duration: %v", b.stats.BuildDuration)

	return b.stats, nil
}

// createIndexes 创建索引
func (b *GraphBuilder) createIndexes(ctx context.Context) {
	log.Infof("🔧 Creating indexes...")

	indexes := []struct {
		label    string
		property string
	}{
		{"Dish", "name"},
		{"Ingredient", "name"},
		{"Category", "name"},
		{"Cuisine", "name"},
		{"Difficulty", "name"},
	}

	for _, idx := range indexes {
		// Neo4j 索引创建（需要驱动支持）
		// 这里是示意，实际实现取决于 neo4j.Client 的接口
		log.Infof("   Creating index on :%s(%s)", idx.label, idx.property)
	}
}

// Document 简化的文档结构
type Document struct {
	Content  string
	Category string
	DishName string
}
