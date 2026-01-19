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
	Labels     []string               `json:"labels"`
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties"`
}

// GraphRelation 图关系
type GraphRelation struct {
	StartNodeID   string                 `json:"start_node_id"`
	EndNodeID     string                 `json:"end_node_id"`
	RelationType  string                 `json:"relation_type"`
	Properties    map[string]interface{} `json:"properties"`
}

// Subgraph 子图
type Subgraph struct {
	Nodes      []*GraphNode      `json:"nodes"`
	Relations  []*GraphRelation  `json:"relations"`
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
	session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead, DatabaseName: database})
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

// ExecuteQuery 执行Cypher查询
func (c *Client) ExecuteQuery(ctx context.Context, cypher string, params map[string]interface{}) ([][]interface{}, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead, DatabaseName: c.database})
	defer session.Close(ctx)

	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var results [][]interface{}
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
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: c.database})
	defer session.Close(ctx)

	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute write query: %w", err)
	}

	var results [][]interface{}
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

// MultiHopSearch 多跳搜索
func (c *Client) MultiHopSearch(ctx context.Context, entities []string, maxDepth int) (*Subgraph, error) {
	log.Printf("🕸️  Performing multi-hop search (entities: %v, max_depth: %d)", entities, maxDepth)

	cypher := `
	MATCH path = (start)-[*1..2]-(related)
	WHERE start.name IN $entities
	RETURN
		elementId(start) AS start_id,
		start.name AS start_name,
		labels(start) AS start_labels,
		elementId(related) AS related_id,
		related.name AS related_name,
		labels(related) AS related_labels,
		type(last(relationships(path))) AS relation_type,
		length(path) AS hops
	LIMIT 100
	`

	results, err := c.ExecuteQuery(ctx, cypher, map[string]interface{}{
		"entities": entities,
	})
	if err != nil {
		return nil, err
	}

	subgraph := &Subgraph{
		Nodes:     make([]*GraphNode, 0),
		Relations: make([]*GraphRelation, 0),
	}

	// 解析结果
	for _, row := range results {
		if len(row) >= 7 {
			startID := fmt.Sprintf("%v", row[0])
			startName := fmt.Sprintf("%v", row[1])
			startLabels := toStringSlice(row[2])
			relatedID := fmt.Sprintf("%v", row[3])
			relatedName := fmt.Sprintf("%v", row[4])
			relatedLabels := toStringSlice(row[5])
			relationType := fmt.Sprintf("%v", row[6])
			_ = row[7] // hops field (unused)

			// 添加起始节点
			subgraph.Nodes = append(subgraph.Nodes, &GraphNode{
				NodeID: startID,
				Labels: startLabels,
				Name:   startName,
			})

			// 添加相关节点
			subgraph.Nodes = append(subgraph.Nodes, &GraphNode{
				NodeID: relatedID,
				Labels: relatedLabels,
				Name:   relatedName,
			})

			// 添加关系
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
		"queryParts": queryParts,
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

// GetNodeNeighbors 获取节点邻居
func (c *Client) GetNodeNeighbors(ctx context.Context, nodeId string, depth int) ([]*GraphNode, error) {
	cypher := `
	MATCH (n)-[r*1..2]-(neighbor)
	WHERE n.nodeId = $nodeId
	RETURN DISTINCT neighbor.nodeId AS node_id, neighbor.name AS name, labels(neighbor) AS labels
	LIMIT 50
	`

	results, err := c.ExecuteQuery(ctx, cypher, map[string]interface{}{
		"nodeId": nodeId,
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

// CommunityDetection 社区检测（简化版）
func (c *Client) CommunityDetection(ctx context.Context, nodes []*GraphNode) (map[string][]*GraphNode, error) {
	log.Printf("🔍 Performing community detection on %d nodes", len(nodes))

	// 简单实现：按标签分组
	communities := make(map[string][]*GraphNode)

	for _, node := range nodes {
		for _, label := range node.Labels {
			if _, exists := communities[label]; !exists {
				communities[label] = make([]*GraphNode, 0)
			}
			communities[label] = append(communities[label], node)
		}
	}

	log.Printf("✅ Community detection completed: %d communities", len(communities))
	return communities, nil
}

// CreateNode 创建节点
func (c *Client) CreateNode(ctx context.Context, label, name string, properties map[string]interface{}) (string, error) {
	cypher := fmt.Sprintf(`
		MERGE (n:%s {name: $name})
		SET n += $props
		RETURN elementId(n) as id
	`, label)

	// 合并 name 到 properties
	if properties == nil {
		properties = make(map[string]interface{})
	}
	properties["name"] = name

	results, err := c.ExecuteWrite(ctx, cypher, map[string]interface{}{
		"name":  name,
		"props": properties,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create node: %w", err)
	}

	if len(results) > 0 && len(results[0]) > 0 {
		return fmt.Sprintf("%v", results[0][0]), nil
	}

	return "", fmt.Errorf("no node created")
}

// CreateRelation 创建关系
func (c *Client) CreateRelation(ctx context.Context, fromID, toID, relType string, properties map[string]interface{}) error {
	cypher := fmt.Sprintf(`
		MATCH (from), (to)
		WHERE elementId(from) = $fromId AND elementId(to) = $toId
		MERGE (from)-[r:%s]->(to)
		SET r += $props
		RETURN r
	`, relType)

	params := map[string]interface{}{
		"fromId": fromID,
		"toId":   toID,
		"props":  properties,
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
