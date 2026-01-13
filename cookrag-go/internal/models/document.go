package models

// Document 文档模型
type Document struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
	Score    float32                `json:"score,omitempty"`
}

// RetrievalResult 检索结果
type RetrievalResult struct {
	Documents []Document `json:"documents"`
	Strategy  string      `json:"strategy"`
	Latency   float64     `json:"latency_ms"`
	Query     string      `json:"query"`
}

// QueryAnalysis 查询分析结果
type QueryAnalysis struct {
	Query                 string  `json:"query"`
	Complexity            float64 `json:"complexity"`
	RelationshipIntensity float64 `json:"relationship_intensity"`
	RecommendedStrategy   string  `json:"recommended_strategy"`
	Confidence             float64 `json:"confidence"`
}
