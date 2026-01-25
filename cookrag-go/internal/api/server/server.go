package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/charmbracelet/log"
	"cookrag-go/internal/api/handlers"
	"cookrag-go/internal/core/router"
)

// Server HTTP服务器
type Server struct {
	router       *gin.Engine
	httpServer   *http.Server
	port         int
	queryRouter  *router.QueryRouter
	llmProvider  any // LLM provider (can be nil initially)
	queryHandler *handlers.QueryHandler
}

// Config 服务器配置
type Config struct {
	Port           int
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxHeaderBytes int
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Port:           8080,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}
}

// NewServer 创建HTTP服务器
func NewServer(config *Config, queryRouter *router.QueryRouter, llmProvider any) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware())
	router.Use(corsMiddleware())

	// 创建查询处理器（传入路由器）
	queryHandler := handlers.NewQueryHandler(queryRouter, llmProvider)

	return &Server{
		router:       router,
		port:         config.Port,
		queryRouter:  queryRouter,
		llmProvider:  llmProvider,
		queryHandler: queryHandler,
		httpServer: &http.Server{
			Addr:           fmt.Sprintf(":%d", config.Port),
			Handler:        router,
			ReadTimeout:    config.ReadTimeout,
			WriteTimeout:   config.WriteTimeout,
			MaxHeaderBytes: config.MaxHeaderBytes,
		},
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	s.setupRoutes()

	log.Infof("🚀 Starting HTTP server on port %d", s.port)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Shutdown 关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	log.Info("🛑 Shutting down HTTP server...")
	return s.httpServer.Shutdown(ctx)
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	api := s.router.Group("/api/v1")
	{
		// 查询接口
		api.POST("/query", s.queryHandler.HandleQuery)

		// 健康检查
		api.GET("/health", s.queryHandler.HandleHealth)
		api.GET("/ready", s.queryHandler.HandleReady)

		// 指标
		api.GET("/metrics", s.queryHandler.HandleMetrics)
	}

	// 根路径
	s.router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "CookRAG-Go API Server",
			"version": "1.0.0",
		})
	})
}

// loggerMiddleware 日志中间件
func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if query != "" {
			path = path + "?" + query
		}

		log.Infof("📡 %s %s %s %d %dms",
			method,
			clientIP,
			path,
			statusCode,
			latency.Milliseconds(),
		)
	}
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
