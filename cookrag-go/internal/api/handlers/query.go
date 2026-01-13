package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/charmbracelet/log"
	"cookrag-go/internal/models"
)

// QueryHandler 查询处理器
type QueryHandler struct {
	// 这里应该注入路由器等核心组件
	// router *router.QueryRouter
}

// NewQueryHandler 创建查询处理器
func NewQueryHandler() *QueryHandler {
	return &QueryHandler{}
}

// QueryRequest 查询请求
type QueryRequest struct {
	Query string `json:"query" binding:"required"`
}

// QueryResponse 查询响应
type QueryResponse struct {
	Answer    string                `json:"answer"`
	Documents []models.Document     `json:"documents"`
	Strategy  string                `json:"strategy"`
	Latency   float64               `json:"latency_ms"`
	Analysis  *models.QueryAnalysis `json:"analysis,omitempty"`
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

	// TODO: 实际实现应该调用路由器
	// result, err := h.router.Route(c.Request.Context(), req.Query)

	// 临时响应
	response := QueryResponse{
		Answer: "这是查询结果：" + req.Query,
		Documents: []models.Document{},
		Strategy: "hybrid",
		Latency:  100.0,
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
