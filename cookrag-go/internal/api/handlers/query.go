package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/charmbracelet/log"
	"cookrag-go/internal/core/router"
	"cookrag-go/internal/models"
)

// QueryHandler 查询处理器
type QueryHandler struct {
	router *router.QueryRouter
	llm    any // LLM provider for answer generation (can be nil)
}

// NewQueryHandler 创建查询处理器
func NewQueryHandler(r *router.QueryRouter, llm any) *QueryHandler {
	return &QueryHandler{
		router: r,
		llm:    llm,
	}
}

// QueryRequest 查询请求
type QueryRequest struct {
	Query string `json:"query" binding:"required"`
}

// QueryResponse 查询响应
type QueryResponse struct {
	Answer    string            `json:"answer"`
	Documents []models.Document `json:"documents"`
	Strategy  string            `json:"strategy"`
	Latency   float64           `json:"latency_ms"`
}

// HandleQuery 处理查询请求
func (h *QueryHandler) HandleQuery(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	log.Infof("📥 Received query: %s", req.Query)

	// 调用路由器进行检索
	result, err := h.router.Route(c.Request.Context(), req.Query)
	if err != nil {
		log.Errorf("❌ Query failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Query processing failed",
			"details": err.Error(),
		})
		return
	}

	// 构建响应
	response := QueryResponse{
		Answer:    "", // LLM生成的答案将在后续添加
		Documents: result.Documents,
		Strategy:  result.Strategy,
		Latency:   result.Latency,
	}

	c.JSON(http.StatusOK, response)
}

// HandleHealth 健康检查
func (h *QueryHandler) HandleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"service": "CookRAG-Go",
	})
}

// HandleReady 就绪检查
func (h *QueryHandler) HandleReady(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// HandleMetrics 指标接口
func (h *QueryHandler) HandleMetrics(c *gin.Context) {
	// TODO: 实现Prometheus指标
	c.JSON(http.StatusOK, gin.H{
		"metrics": "prometheus metrics here",
	})
}
