package retrieval

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/models"
	"cookrag-go/pkg/storage/neo4j"
)

// GraphRetrieverConfig 图RAG检索配置
type GraphRetrieverConfig struct {
	MaxDepth      int    // 最大跳数
	MaxNodes      int    // 最大节点数
	UseCommunity  bool   // 是否使用社区检测
	TopK          int    // 返回结果数量
}

// DefaultGraphRetrieverConfig 默认配置
func DefaultGraphRetrieverConfig() *GraphRetrieverConfig {
	return &GraphRetrieverConfig{
		MaxDepth:     2,
		MaxNodes:     50,
		UseCommunity: true,
		TopK:         10,
	}
}

// GraphRetriever 图RAG检索器
type GraphRetriever struct {
	config     *GraphRetrieverConfig
	neo4jClient *neo4j.Client
}

// NewGraphRetriever 创建图RAG检索器
func NewGraphRetriever(
	config *GraphRetrieverConfig,
	neo4jClient *neo4j.Client,
) *GraphRetriever {
	if config == nil {
		config = DefaultGraphRetrieverConfig()
	}

	return &GraphRetriever{
		config:     config,
		neo4jClient: neo4jClient,
	}
}

// Retrieve 图RAG检索
func (r *GraphRetriever) Retrieve(ctx context.Context, query string) (*models.RetrievalResult, error) {
	startTime := time.Now()

	log.Infof("🕸️  Graph RAG retrieval: query='%s', max_depth=%d", query, r.config.MaxDepth)

	// 1. 提取查询中的实体
	entities, err := r.neo4jClient.ExtractEntities(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to extract entities: %w", err)
	}

	if len(entities) == 0 {
		log.Warnf("⚠️  No entities found in query: %s", query)
		return &models.RetrievalResult{
			Documents: []models.Document{},
			Strategy:  "graph",
			Query:     query,
			Latency:   float64(time.Since(startTime).Milliseconds()),
		}, nil
	}

	log.Infof("🔤 Extracted entities: %v", entities)

	// 2. 多跳搜索获取子图
	subgraph, err := r.neo4jClient.MultiHopSearch(ctx, entities, r.config.MaxDepth)
	if err != nil {
		return nil, fmt.Errorf("multi-hop search failed: %w", err)
	}

	log.Infof("✅ Subgraph retrieved: %d nodes, %d relations",
		len(subgraph.Nodes), len(subgraph.Relations))

	// 3. 社区检测（可选）
	var communities map[string][]*neo4j.GraphNode
	if r.config.UseCommunity && len(subgraph.Nodes) > 0 {
		communities, err = r.neo4jClient.CommunityDetection(ctx, subgraph.Nodes)
		if err != nil {
			log.Warnf("⚠️  Community detection failed: %v", err)
		} else {
			log.Infof("🔍 Detected %d communities", len(communities))
		}
	}

	// 4. 构建文档结果
	documents := r.buildDocumentsFromSubgraph(subgraph, communities)

	// 5. 截取top-k
	if len(documents) > r.config.TopK {
		documents = documents[:r.config.TopK]
	}

	result := &models.RetrievalResult{
		Documents: documents,
		Strategy:  "graph",
		Query:     query,
		Latency:   float64(time.Since(startTime).Milliseconds()),
	}

	log.Infof("✅ Graph RAG retrieval completed: %d results in %.2fms",
		len(documents), result.Latency)

	return result, nil
}

// buildDocumentsFromSubgraph 从子图构建文档
func (r *GraphRetriever) buildDocumentsFromSubgraph(
	subgraph *neo4j.Subgraph,
	communities map[string][]*neo4j.GraphNode,
) []models.Document {
	documents := make([]models.Document, 0)

	// 为每个节点创建文档
	for _, node := range subgraph.Nodes {
		doc := models.Document{
			ID:    node.NodeID,
			Score: 1.0, // 默认分数
			Content: fmt.Sprintf("节点: %s\n标签: %v",
				node.Name, node.Labels),
			Metadata: map[string]interface{}{
				"node_id": node.NodeID,
				"name":    node.Name,
				"labels":  node.Labels,
				"type":    "graph_node",
			},
		}

		// 添加属性
		for key, value := range node.Properties {
			doc.Metadata[key] = value
		}

		// 添加社区信息
		if communities != nil {
			for communityLabel, communityNodes := range communities {
				for _, communityNode := range communityNodes {
					if communityNode.NodeID == node.NodeID {
						doc.Metadata["community"] = communityLabel
						break
					}
				}
			}
		}

		documents = append(documents, doc)
	}

	// 为每个关系创建文档
	for _, relation := range subgraph.Relations {
		doc := models.Document{
			ID:    fmt.Sprintf("rel_%s_%s", relation.StartNodeID, relation.EndNodeID),
			Score: 0.8,
			Content: fmt.Sprintf("关系: %s -> %s\n类型: %s",
				relation.StartNodeID, relation.EndNodeID, relation.RelationType),
			Metadata: map[string]interface{}{
				"start_node_id":  relation.StartNodeID,
				"end_node_id":    relation.EndNodeID,
				"relation_type":  relation.RelationType,
				"type":           "graph_relation",
			},
		}

		// 添加关系属性
		for key, value := range relation.Properties {
			doc.Metadata[key] = value
		}

		documents = append(documents, doc)
	}

	// 计算分数：基于节点度数（连接数）
	nodeDegrees := r.calculateNodeDegrees(subgraph)
	for i := range documents {
		if nodeID, ok := documents[i].Metadata["node_id"].(string); ok {
			if degree, exists := nodeDegrees[nodeID]; exists {
				// 归一化分数
				documents[i].Score = float32(degree) / float32(len(subgraph.Nodes))
			}
		}
	}

	return documents
}

// calculateNodeDegrees 计算节点度数
func (r *GraphRetriever) calculateNodeDegrees(subgraph *neo4j.Subgraph) map[string]int {
	degrees := make(map[string]int)

	// 初始化度数
	for _, node := range subgraph.Nodes {
		degrees[node.NodeID] = 0
	}

	// 统计度数
	for _, relation := range subgraph.Relations {
		if _, exists := degrees[relation.StartNodeID]; exists {
			degrees[relation.StartNodeID]++
		}
		if _, exists := degrees[relation.EndNodeID]; exists {
			degrees[relation.EndNodeID]++
		}
	}

	return degrees
}

// NeighborExpands 邻居扩展（用于增强检索）
func (r *GraphRetriever) NeighborExpand(ctx context.Context, nodeID string, depth int) (*models.RetrievalResult, error) {
	startTime := time.Now()

	log.Infof("🔍 Neighbor expansion: node_id=%s, depth=%d", nodeID, depth)

	neighbors, err := r.neo4jClient.GetNodeNeighbors(ctx, nodeID, depth)
	if err != nil {
		return nil, fmt.Errorf("failed to get neighbors: %w", err)
	}

	documents := make([]models.Document, 0, len(neighbors))
	for _, neighbor := range neighbors {
		doc := models.Document{
			ID:    neighbor.NodeID,
			Score: 0.9,
			Content: fmt.Sprintf("邻居节点: %s\n标签: %v",
				neighbor.Name, neighbor.Labels),
			Metadata: map[string]interface{}{
				"node_id": neighbor.NodeID,
				"name":    neighbor.Name,
				"labels":  neighbor.Labels,
				"type":    "neighbor",
			},
		}
		documents = append(documents, doc)
	}

	result := &models.RetrievalResult{
		Documents: documents,
		Strategy:  "graph_neighbor",
		Query:     nodeID,
		Latency:   float64(time.Since(startTime).Milliseconds()),
	}

	log.Infof("✅ Neighbor expansion completed: %d neighbors in %.2fms",
		len(documents), result.Latency)

	return result, nil
}

// GetStats 获取检索器统计信息
func (r *GraphRetriever) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"max_depth":     r.config.MaxDepth,
		"max_nodes":     r.config.MaxNodes,
		"use_community": r.config.UseCommunity,
		"top_k":         r.config.TopK,
		"strategy":      "graph_rag",
	}
}
