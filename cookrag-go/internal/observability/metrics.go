package observability

import (
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// Metrics 监控指标
type Metrics struct {
	mu              sync.RWMutex
	QueryCount      int64         `json:"query_count"`
	TotalLatency    time.Duration `json:"total_latency_ms"`
	ErrorCount      int64         `json:"error_count"`
	CacheHitCount   int64         `json:"cache_hit_count"`
	CacheMissCount  int64         `json:"cache_miss_count"`
	VectorRetrievalCount int64    `json:"vector_retrieval_count"`
	BM25RetrievalCount   int64    `json:"bm25_retrieval_count"`
	GraphRetrievalCount  int64    `json:"graph_retrieval_count"`
	HybridRetrievalCount int64    `json:"hybrid_retrieval_count"`
}

// MetricsCollector 指标收集器
type MetricsCollector struct {
	mu          sync.RWMutex
	metrics     *Metrics
	startTime   time.Time
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: &Metrics{},
		startTime: time.Now(),
	}
}

// RecordQuery 记录查询
func (m *MetricsCollector) RecordQuery(latency time.Duration, strategy string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.QueryCount++
	m.metrics.TotalLatency += latency

	switch strategy {
	case "vector":
		m.metrics.VectorRetrievalCount++
	case "bm25":
		m.metrics.BM25RetrievalCount++
	case "graph":
		m.metrics.GraphRetrievalCount++
	case "hybrid":
		m.metrics.HybridRetrievalCount++
	}
}

// RecordError 记录错误
func (m *MetricsCollector) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.ErrorCount++
}

// RecordCacheHit 记录缓存命中
func (m *MetricsCollector) RecordCacheHit() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.CacheHitCount++
}

// RecordCacheMiss 记录缓存未命中
func (m *MetricsCollector) RecordCacheMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.CacheMissCount++
}

// GetMetrics 获取指标
func (m *MetricsCollector) GetMetrics() *Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.metrics
}

// GetAverageLatency 获取平均延迟
func (m *MetricsCollector) GetAverageLatency() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.metrics.QueryCount == 0 {
		return 0
	}

	return m.metrics.TotalLatency / time.Duration(m.metrics.QueryCount)
}

// GetCacheHitRate 获取缓存命中率
func (m *MetricsCollector) GetCacheHitRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.metrics.CacheHitCount + m.metrics.CacheMissCount
	if total == 0 {
		return 0
	}

	return float64(m.metrics.CacheHitCount) / float64(total)
}

// GetErrorRate 获取错误率
func (m *MetricsCollector) GetErrorRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.metrics.QueryCount == 0 {
		return 0
	}

	return float64(m.metrics.ErrorCount) / float64(m.metrics.QueryCount)
}

// GetUptime 获取运行时间
func (m *MetricsCollector) GetUptime() time.Duration {
	return time.Since(m.startTime)
}

// LogMetrics 日志记录指标
func (m *MetricsCollector) LogMetrics() {
	metrics := m.GetMetrics()

	log.Infof("📊 Metrics Summary:")
	log.Infof("  Uptime: %s", m.GetUptime().Round(time.Second))
	log.Infof("  Total Queries: %d", metrics.QueryCount)
	log.Infof("  Average Latency: %dms", m.GetAverageLatency().Milliseconds())
	log.Infof("  Error Rate: %.2f%%", m.GetErrorRate()*100)
	log.Infof("  Cache Hit Rate: %.2f%%", m.GetCacheHitRate()*100)
	log.Infof("  Strategy Distribution:")
	log.Infof("    Vector: %d", metrics.VectorRetrievalCount)
	log.Infof("    BM25: %d", metrics.BM25RetrievalCount)
	log.Infof("    Graph: %d", metrics.GraphRetrievalCount)
	log.Infof("    Hybrid: %d", metrics.HybridRetrievalCount)
}

// StartMetricsReporter 启动指标报告
func (m *MetricsCollector) StartMetricsReporter(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("🛑 Metrics reporter stopped")
			return
		case <-ticker.C:
			m.LogMetrics()
		}
	}
}

// Global global metrics collector
var Global *MetricsCollector

func init() {
	Global = NewMetricsCollector()
}
