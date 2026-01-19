package retrieval

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"cookrag-go/internal/models"
	"github.com/yanyiwu/gojieba"
)

// BM25Config BM25配置参数
type BM25Config struct {
	K1 float64 // 词频饱和参数 (通常1.2-2.0)
	B  float64 // 长度惩罚参数 (通常0.75)
}

// DefaultBM25Config 默认BM25配置
func DefaultBM25Config() *BM25Config {
	return &BM25Config{
		K1: 1.5,
		B:  0.75,
	}
}

// InvertedIndex 倒排索引
type InvertedIndex struct {
	mu       sync.RWMutex
	// 词项 -> 文档ID列表
	Postings map[string][]int64
	// 词项 -> 文档频率
	DocFreq map[string]int
	// 文档ID -> 文档长度
	DocLengths map[int64]int
	// 平均文档长度
	AvgDocLength float64
	// 总文档数
	TotalDocs int
}

// BM25Retriever BM25检索器
type BM25Retriever struct {
	config    *BM25Config
	index     *InvertedIndex
	tokenizer *gojieba.Jieba
}

// NewBM25Retriever 创建BM25检索器
func NewBM25Retriever(config *BM25Config) *BM25Retriever {
	if config == nil {
		config = DefaultBM25Config()
	}

	// 初始化 jieba 分词器
	tokenizer := gojieba.NewJieba()

	return &BM25Retriever{
		config:    config,
		tokenizer: tokenizer,
		index: &InvertedIndex{
			Postings:     make(map[string][]int64),
			DocFreq:      make(map[string]int),
			DocLengths:   make(map[int64]int),
			AvgDocLength: 0,
			TotalDocs:    0,
		},
	}
}

// Tokenize 使用 jieba 进行中文分词
func (r *BM25Retriever) Tokenize(text string) []string {
	// 使用 jieba 分词，搜索模式 (HMM=true)
	words := r.tokenizer.Cut(text, true)

	// 停用词过滤（简化版）
	stopWords := map[string]bool{
		"的": true, "了": true, "在": true, "是": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"之": true, "与": true, "及": true, "等": true, "或": true,
		"吗": true, "呢": true, "吧": true, "啊": true, "呀": true,
		// 英文停用词
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"of": true, "for": true, "with": true, "by": true, "from": true,
	}

	filtered := make([]string, 0)
	for _, word := range words {
		word = strings.TrimSpace(word)
		// 过滤停用词、单字符、纯数字/标点
		if word != "" && !stopWords[word] && len(word) > 1 && !isPunctuation(word) {
			filtered = append(filtered, word)
		}
	}

	return filtered
}

// isPunctuation 判断是否是标点符号
// 只有整个字符串都是标点符号才返回true，只要有一个有效字符就保留
func isPunctuation(s string) bool {
	if len(s) == 0 {
		return true
	}

	hasValidChar := false
	for _, r := range s {
		isAlpha := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		isChinese := r >= 0x4e00 && r <= 0x9fa5

		if isAlpha || isDigit || isChinese {
			hasValidChar = true
		}
	}

	return !hasValidChar // 没有任何有效字符才算标点
}

// IndexDocuments 索引文档
func (r *BM25Retriever) IndexDocuments(ctx context.Context, documents []models.Document) error {
	log.Infof("📝 Indexing %d documents with BM25", len(documents))

	r.index.mu.Lock()
	defer r.index.mu.Unlock()

	totalLength := 0

	for _, doc := range documents {
		docID := doc.ID
		if docID == "" {
			docID = fmt.Sprintf("doc_%d", r.index.TotalDocs)
		}

		// 分词
		words := r.Tokenize(doc.Content)
		docLength := len(words)
		docIDInt := int64(r.index.TotalDocs)
		r.index.DocLengths[docIDInt] = docLength
		totalLength += docLength

		// 构建倒排索引
		termFreq := make(map[string]int)
		for _, word := range words {
			termFreq[word]++
		}

		// 更新倒排表
		for term := range termFreq {
			if _, exists := r.index.Postings[term]; !exists {
				r.index.Postings[term] = make([]int64, 0)
			}
			r.index.Postings[term] = append(r.index.Postings[term], int64(r.index.TotalDocs))
		}

		r.index.TotalDocs++
	}

	// 计算平均文档长度
	if r.index.TotalDocs > 0 {
		r.index.AvgDocLength = float64(totalLength) / float64(r.index.TotalDocs)
	}

	// 计算文档频率
	for term, postings := range r.index.Postings {
		uniqueDocs := make(map[int64]bool)
		for _, docID := range postings {
			uniqueDocs[docID] = true
		}
		r.index.DocFreq[term] = len(uniqueDocs)
	}

	log.Infof("✅ BM25 indexing completed: %d docs, avg_len: %.2f, %d unique terms",
		r.index.TotalDocs, r.index.AvgDocLength, len(r.index.Postings))

	return nil
}

// Retrieve BM25检索
func (r *BM25Retriever) Retrieve(ctx context.Context, query string, topK int) ([]models.Document, error) {
	startTime := time.Now()

	// 分词
	queryTerms := r.Tokenize(query)
	if len(queryTerms) == 0 {
		return []models.Document{}, nil
	}

	log.Infof("🔍 BM25 retrieval: query='%s', terms=%d, top_k=%d", query, len(queryTerms), topK)

	r.index.mu.RLock()
	defer r.index.mu.RUnlock()

	// 计算每个文档的BM25分数
	scores := make(map[int64]float64)

	for _, term := range queryTerms {
		postings, termExists := r.index.Postings[term]
		if !termExists {
			continue
		}

		docFreq := r.index.DocFreq[term]
		idf := math.Log((float64(r.index.TotalDocs) - float64(docFreq) + 0.5) / (float64(docFreq) + 0.5))

		// 计算每个文档的分数贡献
		for _, docID := range postings {
			docLength := r.index.DocLengths[docID]
			normFactor := 1 - r.config.B + r.config.B*float64(docLength)/r.index.AvgDocLength

			// 简化版：使用词频=1（实际应该统计词频）
			tf := 1.0
			score := idf * (tf * (r.config.K1 + 1)) / (tf + r.config.K1*normFactor)

			scores[docID] += score
		}
	}

	// 排序
	type docScore struct {
		DocID int64
		Score float64
	}

	rankedDocs := make([]docScore, 0, len(scores))
	for docID, score := range scores {
		rankedDocs = append(rankedDocs, docScore{docID, score})
	}

	sort.Slice(rankedDocs, func(i, j int) bool {
		return rankedDocs[i].Score > rankedDocs[j].Score
	})

	// 返回top-k结果
	results := make([]models.Document, 0, min(topK, len(rankedDocs)))
	for i := 0; i < min(topK, len(rankedDocs)); i++ {
		results = append(results, models.Document{
			ID:    fmt.Sprintf("doc_%d", rankedDocs[i].DocID),
			Score: float32(rankedDocs[i].Score),
		})
	}

	latency := time.Since(startTime).Milliseconds()
	log.Infof("✅ BM25 retrieval completed: %d results in %dms", len(results), latency)

	return results, nil
}

// GetStats 获取索引统计信息
func (r *BM25Retriever) GetStats() map[string]interface{} {
	r.index.mu.RLock()
	defer r.index.mu.RUnlock()

	return map[string]interface{}{
		"total_docs":      r.index.TotalDocs,
		"unique_terms":    len(r.index.Postings),
		"avg_doc_length":  r.index.AvgDocLength,
		"k1":              r.config.K1,
		"b":               r.config.B,
	}
}
