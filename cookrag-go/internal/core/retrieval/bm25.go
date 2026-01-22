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
	"cookrag-go/internal/observability"
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

	totalLength := 0 // 累计所有文档的总词数（用于计算平均文档长度）

	for _, doc := range documents {
		docID := doc.ID
		if docID == "" {
			docID = fmt.Sprintf("doc_%d", r.index.TotalDocs)
		}

		// 分词
		words := r.Tokenize(doc.Content)
		docLength := len(words) // 当前文档的词数
		docIDInt := int64(r.index.TotalDocs) // TotalDocs 当前值就是当前文档的ID（0, 1, 2...）
		r.index.DocLengths[docIDInt] = docLength
		totalLength += docLength // 累加总词数：例：文档0有50词，文档1有30词 → totalLength=80

		// 构建倒排索引（词 → 文档列表 的映射）
		// 例：{"红烧": [0, 5, 12], "肉": [0, 5, 12, 23]} 表示这些词出现在哪些文档中
		termFreq := make(map[string]int) // 统计当前文档中每个词的出现次数
		for _, word := range words {
			termFreq[word]++ // 例：{"红烧": 2, "肉": 3, "怎么": 1, "做": 1}
		}

		// 更新倒排表（记录每个词出现在哪些文档中）
		for term := range termFreq { // 遍历当前文档中的每个唯一词
			if _, exists := r.index.Postings[term]; !exists {
				r.index.Postings[term] = make([]int64, 0) // 初始化该词的文档列表
			}
			// 将当前文档ID添加到该词的倒排列表
			// TotalDocs 作为计数器：处理文档0时是0，处理完后++变成1（下一个文档的ID）
			r.index.Postings[term] = append(r.index.Postings[term], int64(r.index.TotalDocs))
		}

		r.index.TotalDocs++ // 处理完当前文档后递增，为下一个文档准备ID
	}

	// 计算平均文档长度（BM25算法需要）
	// totalLength: 所有文档的总词数（例：3000词）
	// r.index.TotalDocs: 文档总数（例：10个文档）
	// r.index.AvgDocLength: 平均每个文档的词数（例：3000/10=300词）
	if r.index.TotalDocs > 0 {
		r.index.AvgDocLength = float64(totalLength) / float64(r.index.TotalDocs)
	}

	// 计算文档频率（DF: Document Frequency，即一个词出现在多少个文档中）
	// 用于计算IDF（逆文档频率）：DF越小（词越稀有），IDF越大，权重越高
	for term, postings := range r.index.Postings {
		uniqueDocs := make(map[int64]bool) // 用map去重（确保同一文档只计数一次）
		for _, docID := range postings {
			uniqueDocs[docID] = true
		}
		r.index.DocFreq[term] = len(uniqueDocs) // 例：Postings["红烧"]=[0,1,2] → DF=3
	}

	log.Infof("✅ BM25 indexing completed: %d docs, avg_len: %.2f, %d unique terms",
		r.index.TotalDocs, r.index.AvgDocLength, len(r.index.Postings))

	return nil
}

// Retrieve BM25检索
func (r *BM25Retriever) Retrieve(ctx context.Context, query string, topK int) ([]models.Document, error) {
	// 创建链路追踪 span
	span := observability.GlobalTracer.StartSpan(ctx, "bm25_retrieve", map[string]interface{}{
		"query": query,
		"top_k": topK,
	})
	defer span.End()

	startTime := time.Now()

	// 分词
	queryTerms := r.Tokenize(query)
	if len(queryTerms) == 0 {
		return []models.Document{}, nil
	}

	log.Infof("🔍 BM25 retrieval: query='%s', terms=%d, top_k=%d", query, len(queryTerms), topK)
	span.AddMetadata("term_count", len(queryTerms))

	r.index.mu.RLock()
	defer r.index.mu.RUnlock()

	// 计算每个文档的BM25分数
	scores := make(map[int64]float64)

	for _, term := range queryTerms { // 遍历查询中的每个词（如：["红烧", "肉"]）
		postings, termExists := r.index.Postings[term] // 获取包含该词的文档列表
		if !termExists {
			continue // 词不在索引中，跳过
		}

		docFreq := r.index.DocFreq[term] // 该词的文档频率（出现在多少个文档中）
		// 计算IDF（逆文档频率）：词越稀有，IDF越大
		// 公式：log((总文档数 - 文档频率 + 0.5) / (文档频率 + 0.5))
		idf := math.Log((float64(r.index.TotalDocs) - float64(docFreq) + 0.5) / (float64(docFreq) + 0.5))

		// 计算每个文档的分数贡献
		for _, docID := range postings { // 遍历包含该词的所有文档
			docLength := r.index.DocLengths[docID] // 该文档的词数
			// 归一化因子：长文档会"惩罚"分数（避免长文档占优势）
			// B=0.75: 如果文档长度是平均长度的2倍，因子越大，分数越低
			normFactor := 1 - r.config.B + r.config.B*float64(docLength)/r.index.AvgDocLength

			// 简化版：使用词频=1（实际应该统计词在该文档中出现的次数）
			tf := 1.0
			// BM25核心公式：IDF × (TF × (K1 + 1)) / (TF + K1 × 归一化因子)
			// K1=1.5: 控制词频饱和度（TF再大，分数也不会无限增长）
			score := idf * (tf * (r.config.K1 + 1)) / (tf + r.config.K1*normFactor)

			scores[docID] += score // 累加该词对文档的分数贡献
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
	span.AddMetadata("result_count", len(results))
	span.AddMetadata("latency_ms", float64(latency))
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
