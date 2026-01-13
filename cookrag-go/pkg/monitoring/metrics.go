package monitoring

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MetricsCollector 监控指标收集器
type MetricsCollector struct {
	mu sync.Mutex

	// HTTP请求指标
	requestDuration *prometheus.HistogramVec
	requestTotal    *prometheus.CounterVec
	requestActive   *prometheus.GaugeVec

	// RAG检索指标
	retrievalDuration *prometheus.HistogramVec
	retrievalTotal    *prometheus.CounterVec
	retrievalResults  *prometheus.HistogramVec

	// LLM生成指标
	llmDuration    *prometheus.HistogramVec
	llmTokensTotal *prometheus.CounterVec
	llmTotal       *prometheus.CounterVec

	// 缓存指标
	cacheHits   *prometheus.CounterVec
	cacheMisses *prometheus.CounterVec
}

// NewMetricsCollector 创建监控指标收集器
func NewMetricsCollector() *MetricsCollector {
	c := &MetricsCollector{
		// HTTP请求指标
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency distributions",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"method", "endpoint", "status"},
		),
		requestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),
		requestActive: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "http_requests_active",
				Help: "Current number of active HTTP requests",
			},
			[]string{"method", "endpoint"},
		),

		// RAG检索指标
		retrievalDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rag_retrieval_duration_seconds",
				Help:    "RAG retrieval latency distributions",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"strategy"},
		),
		retrievalTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rag_retrievals_total",
				Help: "Total number of RAG retrievals",
			},
			[]string{"strategy"},
		),
		retrievalResults: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rag_retrieval_results_count",
				Help:    "Number of results from RAG retrieval",
				Buckets: []float64{1, 5, 10, 20, 50, 100},
			},
			[]string{"strategy"},
		),

		// LLM生成指标
		llmDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "llm_generation_duration_seconds",
				Help:    "LLM generation latency distributions",
				Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30},
			},
			[]string{"model"},
		),
		llmTokensTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "llm_tokens_total",
				Help: "Total number of LLM tokens generated",
			},
			[]string{"model", "type"},
		),
		llmTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "llm_generations_total",
				Help: "Total number of LLM generations",
			},
			[]string{"model"},
		),

		// 缓存指标
		cacheHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"cache_type"},
		),
		cacheMisses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"cache_type"},
		),
	}

	// 注册所有指标
	prometheus.MustRegister(
		c.requestDuration,
		c.requestTotal,
		c.requestActive,
		c.retrievalDuration,
		c.retrievalTotal,
		c.retrievalResults,
		c.llmDuration,
		c.llmTokensTotal,
		c.llmTotal,
		c.cacheHits,
		c.cacheMisses,
	)

	return c
}

// RecordHTTPRequest 记录HTTP请求
func (m *MetricsCollector) RecordHTTPRequest(method, endpoint, status string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requestTotal.WithLabelValues(method, endpoint, status).Inc()
	m.requestDuration.WithLabelValues(method, endpoint, status).Observe(duration.Seconds())
}

// IncActiveRequests 增加活跃请求计数
func (m *MetricsCollector) IncActiveRequests(method, endpoint string) {
	m.requestActive.WithLabelValues(method, endpoint).Inc()
}

// DecActiveRequests 减少活跃请求计数
func (m *MetricsCollector) DecActiveRequests(method, endpoint string) {
	m.requestActive.WithLabelValues(method, endpoint).Dec()
}

// RecordRetrieval 记录RAG检索
func (m *MetricsCollector) RecordRetrieval(strategy string, resultCount int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.retrievalTotal.WithLabelValues(strategy).Inc()
	m.retrievalDuration.WithLabelValues(strategy).Observe(duration.Seconds())
	m.retrievalResults.WithLabelValues(strategy).Observe(float64(resultCount))
}

// RecordLLMGeneration 记录LLM生成
func (m *MetricsCollector) RecordLLMGeneration(model string, promptTokens, completionTokens int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.llmTotal.WithLabelValues(model).Inc()
	m.llmDuration.WithLabelValues(model).Observe(duration.Seconds())
	m.llmTokensTotal.WithLabelValues(model, "prompt").Add(float64(promptTokens))
	m.llmTokensTotal.WithLabelValues(model, "completion").Add(float64(completionTokens))
}

// RecordCacheHit 记录缓存命中
func (m *MetricsCollector) RecordCacheHit(cacheType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cacheHits.WithLabelValues(cacheType).Inc()
}

// RecordCacheMiss 记录缓存未命中
func (m *MetricsCollector) RecordCacheMiss(cacheType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cacheMisses.WithLabelValues(cacheType).Inc()
}
