package observability

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
)

// TraceContext 追踪上下文
type TraceContext struct {
	TraceID   string
	SpanID    string
	StartTime time.Time
	ParentID  string
	Metadata  map[string]interface{}
}

// Span 追踪span
type Span struct {
	Name      string
	Context   *TraceContext
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Success   bool
	Error     error
	Metadata  map[string]interface{}
}

// Tracer 追踪器
type Tracer struct {
	traces map[string]*TraceContext
}

// NewTracer 创建追踪器
func NewTracer() *Tracer {
	return &Tracer{
		traces: make(map[string]*TraceContext),
	}
}

// StartSpan 开始span
func (t *Tracer) StartSpan(ctx context.Context, name string, metadata map[string]interface{}) *Span {
	span := &Span{
		Name:      name,
		StartTime: time.Now(),
		Success:   true,
		Metadata:  metadata,
	}

	// 生成trace ID和span ID（简化版）
	traceID := generateID()
	spanID := generateID()

	span.Context = &TraceContext{
		TraceID:   traceID,
		SpanID:    spanID,
		StartTime: span.StartTime,
		Metadata:  metadata,
	}

	// 保存trace
	t.traces[traceID] = span.Context

	log.Infof("🔍 Span started: %s [trace_id=%s, span_id=%s]", name, traceID, spanID)

	return span
}

// End 结束span
func (s *Span) End() {
	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)

	if s.Error != nil {
		s.Success = false
	}

	status := "✅"
	if !s.Success {
		status = "❌"
	}

	log.Infof("%s Span ended: %s [duration=%dms, success=%v]",
		status, s.Name, s.Duration.Milliseconds(), s.Success)
}

// SetError 设置错误
func (s *Span) SetError(err error) {
	s.Error = err
	s.Success = false
}

// AddMetadata 添加元数据
func (s *Span) AddMetadata(key string, value interface{}) {
	if s.Metadata == nil {
		s.Metadata = make(map[string]interface{})
	}
	s.Metadata[key] = value
}

// TraceContextKey context key
type TraceContextKey struct{}

// WithTraceContext 添加追踪上下文
func WithTraceContext(ctx context.Context, tc *TraceContext) context.Context {
	return context.WithValue(ctx, TraceContextKey{}, tc)
}

// GetTraceContext 获取追踪上下文
func GetTraceContext(ctx context.Context) *TraceContext {
	if tc, ok := ctx.Value(TraceContextKey{}).(*TraceContext); ok {
		return tc
	}
	return nil
}

// generateID 生成ID（简化版）
func generateID() string {
	return time.Now().Format("20060102150405.000000")
}

// Global tracer
var GlobalTracer *Tracer

func init() {
	GlobalTracer = NewTracer()
}
