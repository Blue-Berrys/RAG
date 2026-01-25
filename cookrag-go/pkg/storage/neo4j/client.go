package neo4j

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/yanyiwu/gojieba"
)

// Client Neo4j客户端封装
type Client struct {
	driver   neo4j.DriverWithContext
	database string
}

// GraphNode 图节点
type GraphNode struct {
	NodeID     string                 `json:"node_id"`
	Labels     []string               `json:"labels"`     // 节点标签（分类标记），如 ["Dish", "川菜"] 表示这是菜品且属于川菜
	Name       string                 `json:"name"`       // 节点名称
	Properties map[string]interface{} `json:"properties"` // 节点属性（键值对），如 {"difficulty": "★★★", "time": "60分钟"}
}

// GraphRelation 图关系
type GraphRelation struct {
	StartNodeID  string                 `json:"start_node_id"`
	EndNodeID    string                 `json:"end_node_id"`
	RelationType string                 `json:"relation_type"` // 关系类型，如 "包含"、"属于"、"相似" 等
	Properties   map[string]interface{} `json:"properties"`    // 关系属性（键值对），如 {"amount": "500g", "required": true}
}

// Subgraph 子图
type Subgraph struct {
	Nodes     []*GraphNode     `json:"nodes"`
	Relations []*GraphRelation `json:"relations"`
}

// NewClient 创建Neo4j客户端
func NewClient(uri, username, password, database string) (*Client, error) {
	driver, err := neo4j.NewDriverWithContext(
		uri,
		neo4j.BasicAuth(username, password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	// 测试连接
	ctx := context.Background()
	// 创建一个新的数据库会话（Session），类似于连接池中获取一个连接。
	session := driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeRead, // 只读模式（测试连接）
		DatabaseName: database,             // 从 NewClient 参数传入
	})
	defer session.Close(ctx)

	result, err := session.Run(ctx, "RETURN 1 as test", nil)
	if err != nil {
		driver.Close(ctx)
		return nil, fmt.Errorf("failed to test Neo4j connection: %w", err)
	}

	if result.Next(ctx) {
		log.Printf("✅ Connected to Neo4j: %s (database: %s)", uri, database)
	}

	return &Client{
		driver:   driver,
		database: database,
	}, nil
}

// Close 关闭连接
func (c *Client) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}

// ExecuteQuery 执行Cypher查询（只读）
func (c *Client) ExecuteQuery(ctx context.Context, cypher string, params map[string]interface{}) ([][]interface{}, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeRead,
		DatabaseName: c.database,
	})
	defer session.Close(ctx)

	// params 是查询参数（防止注入，类似 SQL 预处理），在 Cypher 中用 $key 引用
	// 例：cypher = "MATCH (n) WHERE n.name = $name RETURN n", params = {"name": "红烧肉"}
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var results [][]interface{}  // 返回值：二维数组 results[行][列]，如 {{"红烧肉", "★★★"}, {"宫保鸡丁", "★★"}}
	for result.Next(ctx) {
		record := result.Record()
		row := make([]interface{}, 0, len(record.Keys))
		for _, key := range record.Keys {
			row = append(row, record.AsMap()[key])
		}
		results = append(results, row)
	}

	return results, nil
}

// ExecuteWrite 执行写入Cypher查询
func (c *Client) ExecuteWrite(ctx context.Context, cypher string, params map[string]interface{}) ([][]interface{}, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeWrite,
		DatabaseName: c.database,
	})
	defer session.Close(ctx)

	// params 是查询参数，在 Cypher 中用 $key 引用，如 params = {"name": "xxx"}
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute write query: %w", err)
	}

	var results [][]interface{}  // 返回值：二维数组 results[行][列]
	for result.Next(ctx) {
		record := result.Record()
		row := make([]interface{}, 0, len(record.Keys))
		for _, key := range record.Keys {
			row = append(row, record.AsMap()[key])
		}
		results = append(results, row)
	}

	return results, nil
}

// MultiHopSearch 多跳搜索（从起始节点出发，查找1-2跳内的所有相关节点）
func (c *Client) MultiHopSearch(ctx context.Context, entities []string, maxDepth int) (*Subgraph, error) {
	log.Printf("🕸️  Performing multi-hop search (entities: %v, max_depth: %d)", entities, maxDepth)

	// Cypher 多跳查询说明：
	// [*1..2] 表示关系深度：1跳（直接关系）到2跳（间接关系）
	// 例如：红烧肉 -[包含]-> 五花肉 -[属于]-> 猪肉
	cypher := `
	MATCH path = (start)-[*1..2]-(related)
	WHERE start.name IN $entities
	RETURN
		elementId(start) AS start_id,              // 起始节点ID → row[0]
		start.name AS start_name,                  // 起始节点名称 → row[1]
		labels(start) AS start_labels,             // 起始节点标签 → row[2]
		elementId(related) AS related_id,          // 相关节点ID → row[3]
		related.name AS related_name,              // 相关节点名称 → row[4]
		labels(related) AS related_labels,         // 相关节点标签 → row[5]
		type(last(relationships(path))) AS relation_type,  // 关系类型 → row[6]
		length(path) AS hops                       // 跳数 → row[7]
	LIMIT 100
	`

	results, err := c.ExecuteQuery(ctx, cypher, map[string]interface{}{
		"entities": entities,  // 查询参数：要搜索的实体名列表，如 ["红烧肉", "五花肉"]
	})
	if err != nil {
		return nil, err
	}

	// 初始化空子图（结果容器）
	subgraph := &Subgraph{
		Nodes:     make([]*GraphNode, 0),
		Relations: make([]*GraphRelation, 0),
	}

	// 解析查询结果，构建子图
	for _, row := range results {
		if len(row) >= 7 {
			// 提取起始节点信息
			startID := fmt.Sprintf("%v", row[0])   // 起始节点ID
			startName := fmt.Sprintf("%v", row[1]) // 起始节点名称
			startLabels := toStringSlice(row[2])   // 起始节点标签

			// 提取相关节点信息
			relatedID := fmt.Sprintf("%v", row[3])   // 相关节点ID
			relatedName := fmt.Sprintf("%v", row[4]) // 相关节点名称
			relatedLabels := toStringSlice(row[5])   // 相关节点标签

			// 提取关系信息
			relationType := fmt.Sprintf("%v", row[6]) // 关系类型（如 "包含"、"属于"）
			_ = row[7] // hops 字段（未使用）

			// 添加起始节点到子图
			subgraph.Nodes = append(subgraph.Nodes, &GraphNode{
				NodeID: startID,
				Labels: startLabels,
				Name:   startName,
			})

			// 添加相关节点到子图
			subgraph.Nodes = append(subgraph.Nodes, &GraphNode{
				NodeID: relatedID,
				Labels: relatedLabels,
				Name:   relatedName,
			})

			// 添加关系到子图
			subgraph.Relations = append(subgraph.Relations, &GraphRelation{
				StartNodeID:  startID,
				EndNodeID:    relatedID,
				RelationType: relationType,
			})
		}
	}

	log.Printf("✅ Multi-hop search completed: %d nodes, %d relations", len(subgraph.Nodes), len(subgraph.Relations))
	return subgraph, nil
}

// ExtractEntities 提取实体（从查询中提取食材或菜品）
func (c *Client) ExtractEntities(ctx context.Context, query string) ([]string, error) {
	log.Printf("🔤 Extracting entities from query: %s", query)

	// 使用jieba分词
	jieba := gojieba.NewJieba()
	defer jieba.Free()
	words := jieba.CutForSearch(query, true)

	// 停用词
	stopWords := map[string]bool{
		"的": true, "了": true, "是": true, "在": true, "我": true,
		"能": true, "做": true, "哪些": true, "有": true, "和": true,
		"怎么": true, "什么": true, "可以": true,
	}

	// 过滤出可能是实体名的词（2-4个字符的中文词）
	queryParts := make([]string, 0)
	for _, word := range words {
		word = strings.TrimSpace(word)
		// 过滤停用词和短词
		if len([]rune(word)) >= 2 && len([]rune(word)) <= 4 && !stopWords[word] {
			queryParts = append(queryParts, word)
		}
	}

	if len(queryParts) == 0 {
		queryParts = []string{query}
	}

	log.Printf("   Tokenized query parts: %v", queryParts)

	// 查找食材和菜品节点
	// Cypher 查询说明（使用 UNION 合并三个查询）：
	// 1. 精确匹配食材名：entity.name IN $queryParts（如 "土豆" 完全匹配）
	// 2. 精确匹配菜名：entity.name IN $queryParts（如 "红烧肉" 完全匹配）
	// 3. 模糊匹配食材：entity.name CONTAINS part（如 "土豆丝" 包含 "土豆"）
	cypher := `
	MATCH (entity:Ingredient)
	WHERE entity.name IN $queryParts
	RETURN DISTINCT entity.name AS name, 'Ingredient' AS type
	UNION
	MATCH (entity:Dish)
	WHERE entity.name IN $queryParts
	RETURN DISTINCT entity.name AS name, 'Dish' AS type
	UNION
	MATCH (entity:Ingredient)
	WHERE any(part IN $queryParts WHERE entity.name CONTAINS part)
	RETURN DISTINCT entity.name AS name, 'Ingredient' AS type
	LIMIT 20
	`

	results, err := c.ExecuteQuery(ctx, cypher, map[string]interface{}{
		"queryParts": queryParts,  // jieba 分词后的词列表，如 ["红烧", "肉", "怎么做"]
	})
	if err != nil {
		return nil, err
	}

	entities := make([]string, 0)
	for _, row := range results {
		if len(row) > 0 {
			name := fmt.Sprintf("%v", row[0])
			entityType := fmt.Sprintf("%v", row[1])
			log.Printf("   Found: %s (%s)", name, entityType)
			entities = append(entities, name)
		}
	}

	log.Printf("✅ Extracted entities: %v", entities)
	return entities, nil
}

// GetNodeNeighbors 获取节点邻居（1-2跳范围内的所有相关节点）
func (c *Client) GetNodeNeighbors(ctx context.Context, nodeId string, depth int) ([]*GraphNode, error) {
	// Cypher 查询说明：
	// [r*1..2] 表示 1-2 跳的关系路径（如：A→B 是1跳，A→B→C 是2跳）
	// 查找指定节点的所有邻居节点
	cypher := `
	MATCH (n)-[r*1..2]-(neighbor)
	WHERE n.nodeId = $nodeId
	RETURN DISTINCT neighbor.nodeId AS node_id, neighbor.name AS name, labels(neighbor) AS labels
	LIMIT 50
	`

	results, err := c.ExecuteQuery(ctx, cypher, map[string]interface{}{
		"nodeId": nodeId,  // 起始节点ID
	})
	if err != nil {
		return nil, err
	}

	neighbors := make([]*GraphNode, 0)
	for _, row := range results {
		if len(row) >= 3 {
			neighbors = append(neighbors, &GraphNode{
				NodeID: fmt.Sprintf("%v", row[0]),
				Name:   fmt.Sprintf("%v", row[1]),
				Labels: row[2].([]string),
			})
		}
	}

	return neighbors, nil
}

// CommunityDetection 社区检测（简化版：按标签分组节点）
// 社区检测的作用：将图中的节点分成多个"社区"或"群组"，同一社区内的节点联系紧密
// 简化实现：直接按节点标签分组，而不是使用复杂算法（如 Louvain、LPA）
func (c *Client) CommunityDetection(ctx context.Context, nodes []*GraphNode) (map[string][]*GraphNode, error) {
	log.Printf("🔍 Performing community detection on %d nodes", len(nodes))

	// 按标签分组：同一标签的节点归为一个社区
	// 例如：所有标签为 "Dish" 的节点归为一组，标签为 "Ingredient" 的归为一组
	communities := make(map[string][]*GraphNode)

	for _, node := range nodes {
		// 遍历节点的所有标签
		for _, label := range node.Labels {
			// 初始化该标签的节点列表
			if _, exists := communities[label]; !exists {
				communities[label] = make([]*GraphNode, 0)
			}
			// 将节点加入对应标签的社区
			communities[label] = append(communities[label], node)
		}
	}

	// 返回值示例：
	// {
	//   "Dish":      [{NodeID: "1", Name: "红烧肉"}, {NodeID: "2", Name: "宫保鸡丁"}],
	//   "Ingredient": [{NodeID: "3", Name: "五花肉"}, {NodeID: "4", Name: "花生"}]
	// }
	log.Printf("✅ Community detection completed: %d communities", len(communities))
	return communities, nil
}

// CreateNode 创建节点（如果已存在则返回现有节点）
func (c *Client) CreateNode(ctx context.Context, label, name string, properties map[string]interface{}) (string, error) {
	// Cypher 查询说明：
	// MERGE: 如果节点存在则匹配，不存在则创建（类似 "INSERT IF NOT EXISTS"）
	// (n:Dish {name: $name}): 节点标签为 label，属性 name 为参数 $name
	// SET n += $props: 将 $props 中的属性合并到节点（覆盖已有属性）
	// elementId(n): 返回节点的唯一ID
	cypher := fmt.Sprintf(`
		MERGE (n:%s {name: $name})
		SET n += $props
		RETURN elementId(n) as id
	`, label)

	// 合并 name 到 properties（确保 name 作为节点属性存在）
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["name"] = name

	results, err := c.ExecuteWrite(ctx, cypher, map[string]interface{}{
		"name":  name,    // 节点名称（用于 MERGE 匹配）
		"props": properties, // 节点属性（会合并到节点）
	})
	if err != nil {
		return "", fmt.Errorf("failed to create node: %w", err)
	}

	// 提取返回的节点ID
	if len(results) > 0 && len(results[0]) > 0 {
		return fmt.Sprintf("%v", results[0][0]), nil
	}

	return "", fmt.Errorf("no node created")
}

// CreateRelation 创建关系（如果已存在则更新属性）
func (c *Client) CreateRelation(ctx context.Context, fromID, toID, relType string, properties map[string]interface{}) error {
	// Cypher 查询说明：
	// MATCH (from), (to): 匹配两个节点（通过ID）
	// WHERE elementId(from) = $fromId: 根据节点ID查找起始节点
	// MERGE (from)-[r:包含]->(to): 如果关系存在则匹配，不存在则创建
	// SET r += $props: 将属性合并到关系（覆盖已有属性）
	cypher := fmt.Sprintf(`
		MATCH (from), (to)
		WHERE elementId(from) = $fromId AND elementId(to) = $toId
		MERGE (from)-[r:%s]->(to)
		SET r += $props
		RETURN r
	`, relType)

	params := map[string]interface{}{
		"fromId": fromID,  // 起始节点ID（如 "4:xxxxxx"）
		"toId":   toID,    // 目标节点ID
		"props":  properties, // 关系属性（如 {"amount": "500g"}）
	}

	_, err := c.ExecuteWrite(ctx, cypher, params)
	if err != nil {
		return fmt.Errorf("failed to create relation: %w", err)
	}

	return nil
}

// ClearGraph 清空图谱
func (c *Client) ClearGraph(ctx context.Context) error {
	cypher := "MATCH (n) DETACH DELETE n"
	_, err := c.ExecuteWrite(ctx, cypher, nil)
	if err != nil {
		return fmt.Errorf("failed to clear graph: %w", err)
	}
	log.Printf("✅ Graph cleared")
	return nil
}

// toStringSlice 将interface{}转换为[]string
func toStringSlice(v interface{}) []string {
	if v == nil {
		return []string{}
	}

	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if str, ok := item.(string); ok {
				result = append(result, str)
			} else {
				result = append(result, fmt.Sprintf("%v", item))
			}
		}
		return result
	default:
		return []string{fmt.Sprintf("%v", v)}
	}
}
