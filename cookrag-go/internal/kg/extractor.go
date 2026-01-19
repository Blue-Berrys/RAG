package kg

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// EntityType 实体类型
type EntityType string

const (
	EntityDish        EntityType = "Dish"
	EntityIngredient  EntityType = "Ingredient"
	EntityCategory    EntityType = "Category"
	EntityCuisine     EntityType = "Cuisine"
	EntityDifficulty  EntityType = "Difficulty"
	EntityTool        EntityType = "Tool"
)

// RelationType 关系类型
type RelationType string

const (
	RelationContains    RelationType = "包含"     // Dish -> Ingredient
	RelationBelongsTo   RelationType = "属于"     // Dish -> Category
	RelationCuisine     RelationType = "菜系"     // Dish -> Cuisine
	RelationDifficulty  RelationType = "难度"     // Dish -> Difficulty
	RelationUsesTool    RelationType = "使用"     // Dish -> Tool
	RelationSubstitute  RelationType = "替代"     // Ingredient -> Ingredient
	RelationSubclass    RelationType = "子类"     // Category -> Category
)

// Entity 实体
type Entity struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       EntityType             `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

// Relation 关系
type Relation struct {
	ID          string                 `json:"id"`
	From        string                 `json:"from"`
	To          string                 `json:"to"`
	Type        RelationType           `json:"type"`
	Properties  map[string]interface{} `json:"properties"`
}

// ExtractedData 提取的数据
type ExtractedData struct {
	Entities   []Entity               `json:"entities"`
	Relations  []Relation             `json:"relations"`
}

// RecipeExtractor 菜谱实体提取器
type RecipeExtractor struct {
	// 食材词典（可扩展）
	ingredientDict map[string]bool
}

// NewRecipeExtractor 创建提取器
func NewRecipeExtractor() *RecipeExtractor {
	// 常见食材列表
	ingredients := []string{
		// 蔬菜
		"西红柿", "番茄", "黄瓜", "茄子", "土豆", "萝卜", "白菜", "菠菜", "芹菜", "韭菜",
		"辣椒", "青椒", "红椒", "胡萝卜", "洋葱", "蒜", "姜", "葱",
		// 肉类
		"猪肉", "五花肉", "牛肉", "羊肉", "鸡肉", "鸭肉", "鱼", "虾", "蟹", "海参",
		// 豆制品
		"豆腐", "豆皮", "腐竹",
		// 蛋奶
		"鸡蛋", "鸭蛋", "皮蛋", "牛奶",
		// 调料
		"盐", "糖", "醋", "酱油", "生抽", "老抽", "料酒", "豆瓣酱", "花椒", "八角",
		// 主食
		"米饭", "面条", "面粉",
	}

	dict := make(map[string]bool)
	for _, ing := range ingredients {
		dict[ing] = true
	}

	return &RecipeExtractor{
		ingredientDict: dict,
	}
}

// ExtractFromRecipe 从菜谱提取实体和关系
func (e *RecipeExtractor) ExtractFromRecipe(content, category, dishName string) *ExtractedData {
	data := &ExtractedData{
		Entities:  make([]Entity, 0),
		Relations: make([]Relation, 0),
	}

	// 1. 提取菜品实体
	dishID := fmt.Sprintf("dish_%s", dishName)
	dishEntity := Entity{
		ID:   dishID,
		Name: dishName,
		Type: EntityDish,
		Properties: map[string]interface{}{
			"content":  content,
			"category": category,
		},
	}
	data.Entities = append(data.Entities, dishEntity)

	// 2. 提取食材（从"必备原料"部分）
	ingredients := e.extractIngredients(content)
	for _, ing := range ingredients {
		ingID := fmt.Sprintf("ing_%s", ing)

		// 食材实体
		ingEntity := Entity{
			ID:   ingID,
			Name: ing,
			Type: EntityIngredient,
		}
		data.Entities = append(data.Entities, ingEntity)

		// 菜品-食材关系
		data.Relations = append(data.Relations, Relation{
			ID:   fmt.Sprintf("%s_contains_%s", dishID, ingID),
			From: dishID,
			To:   ingID,
			Type: RelationContains,
		})
	}

	// 3. 提取分类
	if category != "" {
		categoryID := fmt.Sprintf("cat_%s", category)
		categoryEntity := Entity{
			ID:   categoryID,
			Name: category,
			Type: EntityCategory,
		}
		data.Entities = append(data.Entities, categoryEntity)

		// 菜品-分类关系
		data.Relations = append(data.Relations, Relation{
			ID:   fmt.Sprintf("%s_belongsto_%s", dishID, categoryID),
			From: dishID,
			To:   categoryID,
			Type: RelationBelongsTo,
		})
	}

	// 4. 提取难度
	difficulty := e.extractDifficulty(content)
	if difficulty != "" {
		diffID := fmt.Sprintf("diff_%s", difficulty)
		diffEntity := Entity{
			ID:   diffID,
			Name: difficulty,
			Type: EntityDifficulty,
		}
		data.Entities = append(data.Entities, diffEntity)

		// 菜品-难度关系
		data.Relations = append(data.Relations, Relation{
			ID:   fmt.Sprintf("%s_difficulty_%s", dishID, diffID),
			From: dishID,
			To:   diffID,
			Type: RelationDifficulty,
		})
	}

	// 5. 提取菜系（从分类或内容推断）
	cuisine := e.inferCuisine(category, content)
	if cuisine != "" {
		cuisineID := fmt.Sprintf("cuisine_%s", cuisine)
		cuisineEntity := Entity{
			ID:   cuisineID,
			Name: cuisine,
			Type: EntityCuisine,
		}
		data.Entities = append(data.Entities, cuisineEntity)

		// 菜品-菜系关系
		data.Relations = append(data.Relations, Relation{
			ID:   fmt.Sprintf("%s_cuisine_%s", dishID, cuisineID),
			From: dishID,
			To:   cuisineID,
			Type: RelationCuisine,
		})
	}

	// 6. 提取工具
	tools := e.extractTools(content)
	for _, tool := range tools {
		toolID := fmt.Sprintf("tool_%s", tool)
		toolEntity := Entity{
			ID:   toolID,
			Name: tool,
			Type: EntityTool,
		}
		data.Entities = append(data.Entities, toolEntity)

		// 菜品-工具关系
		data.Relations = append(data.Relations, Relation{
			ID:   fmt.Sprintf("%s_uses_%s", dishID, toolID),
			From: dishID,
			To:   toolID,
			Type: RelationUsesTool,
		})
	}

	return data
}

// extractIngredients 提取食材
func (e *RecipeExtractor) extractIngredients(content string) []string {
	ingredients := make([]string, 0)

	// 查找"必备原料"部分
	lines := strings.Split(content, "\n")
	inIngredientSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 进入原料部分
		if strings.Contains(line, "必备原料") || strings.Contains(line, "原料") {
			inIngredientSection = true
			continue
		}

		// 离开原料部分（遇到其他标题）
		if strings.HasPrefix(line, "##") && !strings.Contains(line, "原料") {
			if inIngredientSection {
				break
			}
		}

		if !inIngredientSection {
			continue
		}

		// 提取食材（去除*、-等标记）
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimSpace(line)

		// 过滤掉非食材行（如单位、数字等）
		if e.isIngredient(line) {
			// 提取中文名称（去除空格及后续内容）
			if idx := strings.IndexAny(line, " 0123456789gml克毫升"); idx > 0 {
				line = line[:idx]
			}
			line = strings.TrimSpace(line)
			if len([]rune(line)) >= 2 && len([]rune(line)) <= 4 {
				ingredients = append(ingredients, line)
			}
		}
	}

	return uniqueStrings(ingredients)
}

// isIngredient 判断是否是食材
func (e *RecipeExtractor) isIngredient(text string) bool {
	text = strings.TrimSpace(text)

	// 空行或太短
	if len(text) < 2 {
		return false
	}

	// 包含数字和单位（可能是用量行）
	if regexp.MustCompile(`\d+\s*(g|ml|克|毫升|个|根|片)`).MatchString(text) {
		// 但也可能以食材名开头
		for ing := range e.ingredientDict {
			if strings.HasPrefix(text, ing) {
				return true
			}
		}
	}

	// 在词典中
	if e.ingredientDict[text] {
		return true
	}

	// 中文字符为主的行
	chineseCount := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			chineseCount++
		}
	}
	ratio := float64(chineseCount) / float64(len([]rune(text)))
	return ratio > 0.5 && len([]rune(text)) <= 4
}

// extractDifficulty 提取难度
func (e *RecipeExtractor) extractDifficulty(content string) string {
	re := regexp.MustCompile(`难度[：:]*([★☆]+|[一二三四]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return "未知"
}

// inferCuisine 推断菜系
func (e *RecipeExtractor) inferCuisine(category, content string) string {
	// 从分类推断
	if strings.Contains(category, "川") || strings.Contains(content, "川菜") {
		return "川菜"
	}
	if strings.Contains(category, "湘") || strings.Contains(content, "湘菜") {
		return "湘菜"
	}
	if strings.Contains(category, "粤") || strings.Contains(content, "粤菜") {
		return "粤菜"
	}
	if strings.Contains(category, "鲁") || strings.Contains(content, "鲁菜") {
		return "鲁菜"
	}
	if strings.Contains(category, "苏") || strings.Contains(content, "苏菜") {
		return "苏菜"
	}
	if strings.Contains(category, "浙") || strings.Contains(content, "浙菜") {
		return "浙菜"
	}
	if strings.Contains(category, "闽") || strings.Contains(content, "闽菜") {
		return "闽菜"
	}
	if strings.Contains(category, "徽") || strings.Contains(content, "徽菜") {
		return "徽菜"
	}

	return "家常菜"
}

// extractTools 提取工具
func (e *RecipeExtractor) extractTools(content string) []string {
	tools := []string{
		"锅", "砂锅", "炒锅", "平底锅", "高压锅", "汤锅",
		"刀", "砧板", "碗", "盘子", "筷子",
	}

	found := make([]string, 0)
	for _, tool := range tools {
		if strings.Contains(content, tool) {
			found = append(found, tool)
		}
	}

	return found
}

// uniqueStrings 去重
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
