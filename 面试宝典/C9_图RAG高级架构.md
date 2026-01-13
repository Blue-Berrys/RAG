# C9: 图 RAG 高级架构

## 一、项目背景与目标

### 1.1 从传统 RAG 到图 RAG 的演进

上一章构建的基于向量检索的传统 RAG 系统，虽然采用了父子文本块的分块策略，能够有效回答简单的菜谱查询。但在处理复杂的关系推理和多跳查询时仍存在明显局限：

**传统 RAG 的局限**：
1. **关系理解缺失**：虽然父子分块保持了文档结构，但无法显式建模食材、菜谱、烹饪方法之间的语义关系
2. **跨文档关联困难**：难以发现不同菜谱之间的相似性、替代关系等隐含联系
3. **推理能力有限**：缺乏基于知识图谱的多跳推理能力，难以回答需要复杂逻辑推理的问题

### 1.2 图 RAG 系统的核心优势

通过引入知识图谱，新系统将具备：
- **结构化知识表达**：以图的形式显式编码实体间的语义关系
- **增强推理能力**：支持多跳推理和复杂关系查询
- **智能查询路由**：根据查询复杂度自动选择最适合的检索策略
- **事实性与可解释性**：基于图结构的推理路径提供可追溯的答案

---

## 二、环境配置

### 2.1 核心组件

**Neo4j 图数据库**：
- 存储菜谱、食材、烹饪步骤及其关系网络
- 支持 Cypher 查询语言
- 可视化图谱结构

**Milvus 向量数据库**：
- 存储菜谱文档的向量嵌入
- 支持语义相似度检索

**LLM API**：
- 用于查询分析、路由决策、答案生成

### 2.2 数据导入

Neo4j 使用 Docker Compose 自动导入数据：

**数据结构**：
- **菜谱节点**：菜名、难度、烹饪时间、菜系等
- **食材节点**：食材名称、分类、营养信息等
- **烹饪步骤节点**：步骤描述、烹饪方法、所需工具等
- **关系网络**：菜谱与食材、步骤之间的复杂关系

**Cypher 导入脚本**：
```cypher
// 创建菜谱节点
CREATE (r:Recipe {
  name: "宫保鸡丁",
  difficulty: "★★",
  time: "30分钟",
  cuisine: "川菜"
})

// 创建食材节点
CREATE (i1:Ingredient {name: "鸡胸肉", category: "肉类"})
CREATE (i2:Ingredient {name: "花生", category: "坚果"})

// 创建关系
MATCH (r:Recipe {name: "宫保鸡丁"})
MATCH (i1:Ingredient {name: "鸡胸肉"})
CREATE (r)-[:CONTAINS_INGREDIENT {amount: "300g"}]->(i1)
```

---

## 三、系统架构设计

### 3.1 整体架构

图 RAG 系统采用模块化设计，包含以下核心组件：

**1. 图数据准备模块 (GraphDataPreparationModule)**
- 连接 Neo4j 数据库
- 加载图数据（菜谱、食材、步骤节点及其关系）
- 构建结构化菜谱文档

**2. 向量索引模块 (MilvusIndexConstructionModule)**
- 构建和管理 Milvus 向量索引
- 使用 BGE-small-zh-v1.5 模型
- 512 维向量空间

**3. 混合检索模块 (HybridRetrievalModule)**
- 传统的混合检索策略
- 结合向量检索和图扩展
- 双层检索（实体级+主题级）
- RRF 轮询融合

**4. 图 RAG 检索模块 (GraphRAGRetrieval)**
- 基于图结构的高级检索
- 支持多跳推理和子图提取
- 图查询理解、多跳遍历、知识子图提取

**5. 智能查询路由 (IntelligentQueryRouter)**
- 分析查询特征（复杂度、关系密集度、推理需求）
- 自动选择检索策略
- LLM 驱动的查询分析

**6. 生成集成模块 (GenerationIntegrationModule)**
- 基于检索结果生成最终答案
- 支持流式输出
- 自适应生成策略、错误处理与重试机制

### 3.2 数据流程

**数据准备阶段**：
1. 从 Neo4j 加载图数据（菜谱、食材、步骤节点及其关系）
2. 构建结构化菜谱文档，保持知识完整性
3. 进行智能文档分块（章节和长度双重分块策略）
4. 构建 Milvus 向量索引，支持语义检索

**查询处理阶段**：
1. 用户输入查询
2. 智能查询路由器分析查询特征
3. 根据分析结果选择检索策略：
   - 简单查询 → 传统混合检索
   - 复杂推理 → 图 RAG 检索
   - 中等复杂 → 组合检索策略
4. 执行相应的检索操作
5. 生成模块基于检索结果生成答案

**错误处理与降级**：
1. 高级策略失败时自动降级到传统混合检索
2. 传统混合检索失败时返回系统异常
3. 支持流式输出中断时的自动重试机制

---

## 四、智能查询路由

### 4.1 四维查询分析

智能查询路由从四个维度分析查询特征：

**1. 复杂度分析 (Complexity Analysis)**
- **0.0-0.3**：简单查找（如"西红柿炒鸡蛋怎么做"）
- **0.4-0.7**：中等复杂（如"川菜有哪些特色菜"）
- **0.8-1.0**：高复杂推理（如"哪些菜适合减肥且下饭"）

**2. 关系密集度分析 (Relation Density)**
- **0.0-0.3**：单一实体查询
- **0.4-0.7**：实体关系查询
- **0.8-1.0**：复杂关系网络查询

**3. 推理需求判断 (Reasoning Analysis)**
- 是否需要多跳推理？
- 是否需要因果分析？
- 是否需要对比分析？

**4. 实体识别统计 (Entity Analysis)**
- 实体数量和类型
- 实体之间的关系类型

### 4.2 LLM 智能分析

使用 LLM 综合评估查询特征：

```python
# LLM 查询分析
def analyze_query(query: str) -> Dict[str, float]:
    prompt = f"""
    分析以下查询的四个维度：
    查询: {query}
    
    请从以下维度打分（0.0-1.0）：
    1. 复杂度 (0.0-0.3: 简单 / 0.4-0.7: 中等 / 0.8-1.0: 高复杂)
    2. 关系密集度 (0.0-0.3: 单一实体 / 0.4-0.7: 实体关系 / 0.8-1.0: 复杂网络)
    3. 推理需求 (是否需要多跳推理、因果分析、对比分析)
    4. 实体数量 (实体数量和类型)
    
    只返回JSON格式的打分结果。
    """
    
    result = llm.complete(prompt)
    return json.loads(result)
```

### 4.3 降级处理

**LLM 分析失败**：
- 降级到规则分析
- 基于关键词匹配
- 启发式规则判断

**规则分析方法**：
```python
def rule_based_analysis(query: str) -> Dict[str, float]:
    # 关键词匹配
    complexity = 0.3
    if "推荐" in query or "适合" in query:
        complexity = 0.5
    if "哪些" in query and "且" in query:
        complexity = 0.8
    
    # 实体识别
    entities = extract_entities(query)
    relation_density = min(0.3 * len(entities), 1.0)
    
    return {
        "complexity": complexity,
        "relation_density": relation_density,
        "reasoning": 1 if "为什么" in query else 0,
        "entity_count": len(entities)
    }
```

### 4.4 路由决策

根据查询分析结果，自动选择检索策略：

**简单查询（复杂度 < 0.4）**：
- 策略：传统混合检索
- 原因：保底策略，快速响应

**复杂推理（关系密集 > 0.7）**：
- 策略：图 RAG 检索
- 原因：高级复杂策略，利用图谱推理

**中等复杂**：
- 策略：组合检索策略
- 原因：融合两种方法

---

## 五、检索策略详解

### 5.1 传统混合检索（保底策略）

**双层检索**：
1. **实体级检索**：基于菜名、食材等实体检索
2. **主题级检索**：基于菜系、难度等主题检索

**RRF 轮询融合**：
```python
def rrf_fusion(entity_results, topic_results, k=60):
    """
    RRF 轮询融合
    """
    scores = {}
    # 轮询交替添加结果
    for i, doc in enumerate(entity_results):
        scores[doc] = scores.get(doc, 0) + 1 / (k + i + 1)
    for i, doc in enumerate(topic_results):
        scores[doc] = scores.get(doc, 0) + 1 / (k + i + 1)
    
    return sorted(scores.items(), key=lambda x: x[1], reverse=True)
```

**内部降级机制**：
- 关键词提取失败 → 简单分词
- 图索引不足 → Neo4j 补充
- Neo4j 失败 → 静默失败

### 5.2 图 RAG 检索（高级策略）

**图查询理解**：
- 识别查询类型：entity_relation（实体关系）、multi_hop（多跳）、subgraph（子图）、path_finding（路径查找）

**多跳图遍历**：
- 最大深度：3 跳
- 发现隐含关联
- 示例：菜谱 → 食材 → 营养成分

**知识子图提取**：
- 完整知识网络
- 最大 100 节点
- 保持图的连通性

**图结构推理**：
- 推理链构建
- 可信度验证
- 路径排序

**Cypher 查询示例**：
```cypher
// 查询包含"鸡肉"的川菜
MATCH (r:Recipe)-[:CONTAINS_INGREDIENT]->(i:Ingredient {name: "鸡肉"})
WHERE r.cuisine = "川菜"
RETURN r.name, r.difficulty, r.time
```

### 5.3 组合检索策略

**配额分配**：
```python
def split_quota(top_k: int) -> Tuple[int, int]:
    """
    分配检索配额
    """
    traditional_k = top_k // 2
    graph_k = top_k - traditional_k
    return traditional_k, graph_k
```

**并行执行**：
- 同时执行传统检索和图 RAG 检索
- 提升检索速度

**Round-robin 合并**：
- 交替添加结果
- 图 RAG 优先（更精确）

**去重和排序**：
- 基于内容哈希去重
- 按相关性排序

---

## 六、降级策略

### 6.1 有限降级原则

**Level 3：图 RAG 检索**（最高级）
- 多跳推理 + 子图提取
- 失败 → 降级到 Level 1

**Level 2：组合检索**（中级）
- 融合两种方法
- 失败 → 降级到 Level 1

**Level 1：传统混合检索**（保底）
- 无更低级降级
- 失败 → 系统异常

### 6.2 降级触发条件

**图 RAG 检索失败**：
- Neo4j 连接失败
- Cypher 查询超时
- 图遍历结果为空

**组合检索失败**：
- 图 RAG 失败
- 自动降级到传统混合检索

**传统混合检索失败**：
- Milvus 连接失败
- 向量检索结果为空
- BM25 检索结果为空
- 返回系统异常

### 6.3 降级实现

```python
def retrieve_with_fallback(query: str, top_k: int) -> List[Document]:
    """
    带降级的检索
    """
    analysis = router.analyze_query(query)
    
    # 路由决策
    if analysis["complexity"] < 0.4:
        strategy = "traditional"
    elif analysis["relation_density"] > 0.7:
        strategy = "graph_rag"
    else:
        strategy = "combined"
    
    # 执行检索
    try:
        if strategy == "graph_rag":
            try:
                return graph_rag_retrieve(query, top_k)
            except Exception as e:
                logger.warning(f"图RAG检索失败，降级到传统混合检索: {e}")
                return hybrid_retrieve(query, top_k)
        
        elif strategy == "combined":
            try:
                return combined_retrieve(query, top_k)
            except Exception as e:
                logger.warning(f"组合检索失败，降级到传统混合检索: {e}")
                return hybrid_retrieve(query, top_k)
        
        else:  # traditional
            return hybrid_retrieve(query, top_k)
    
    except Exception as e:
        logger.error(f"检索失败，无更低级降级策略: {e}")
        raise SystemError("检索系统异常")
```

---

## 七、生成与输出

### 7.1 自适应生成策略

根据检索策略选择不同的生成方式：

**传统混合检索**：
- 基础 Prompt 模板
- Temperature: 0.7

**图 RAG 检索**：
- 强调图证据的 Prompt
- 要求引用推理路径
- Temperature: 0.5（更严格）

**组合检索**：
- 融合图证据和文本证据
- Temperature: 0.6

### 7.2 流式输出

**优势**：
- 逐字符实时显示
- 提升用户体验
- 降低感知延迟

**实现**：
```python
def stream_answer(query: str, context: List[Document]):
    """
    流式输出答案
    """
    prompt = build_prompt(query, context)
    
    for chunk in llm.stream(prompt):
        print(chunk, end="", flush=True)
```

**错误处理**：
- 流式输出中断时自动重试
- 最多重试 3 次
- 失败后返回非流式输出

### 7.3 统计更新

**路由统计**：
- 记录每种策略的使用次数
- 计算成功率
- 分析性能指标

**统计信息**：
```python
{
    "total_queries": 1000,
    "traditional_queries": 600,
    "graph_rag_queries": 300,
    "combined_queries": 100,
    "fallback_rate": 0.1,
    "avg_latency": {
        "traditional": 0.5,
        "graph_rag": 2.0,
        "combined": 1.5
    }
}
```

---

## 八、面试高频问题

### Q1: 什么是智能查询路由？如何实现？

**参考答案**：
智能查询路由是根据查询特征自动选择最适合的检索策略。

**四维查询分析**：
1. **复杂度分析**：0.0-0.3（简单）/ 0.4-0.7（中等）/ 0.8-1.0（高复杂）
2. **关系密集度**：0.0-0.3（单一实体）/ 0.4-0.7（实体关系）/ 0.8-1.0（复杂网络）
3. **推理需求**：多跳推理、因果分析、对比分析
4. **实体识别**：实体数量和类型

**路由决策**：
- **简单查询（复杂度 < 0.4）**：传统混合检索（保底策略）
- **复杂推理（关系密集 > 0.7）**：图 RAG 检索（高级策略）
- **中等复杂**：组合检索策略

**实现方式**：
1. 使用 LLM 分析查询特征
2. 如果 LLM 失败，降级到规则分析（关键词匹配、启发式规则）
3. 根据分析结果选择检索策略

**优势**：
- 提升检索精确性
- 优化系统性能
- 降低成本（简单查询用简单策略）

### Q2: 图 RAG 检索的流程是什么？

**参考答案**：
图 RAG 检索基于图结构进行高级检索，支持多跳推理和子图提取。

**核心流程**：

**1. 图查询理解**
- 识别查询类型：entity_relation（实体关系）、multi_hop（多跳）、subgraph（子图）、path_finding（路径查找）

**2. 多跳图遍历**
- 最大深度：3 跳
- 发现隐含关联
- 示例：菜谱 → 食材 → 营养成分

**3. 知识子图提取**
- 完整知识网络
- 最大 100 节点
- 保持图的连通性

**4. 图结构推理**
- 推理链构建
- 可信度验证
- 路径排序

**Cypher 查询示例**：
```cypher
// 查询包含"鸡肉"的川菜
MATCH (r:Recipe)-[:CONTAINS_INGREDIENT]->(i:Ingredient {name: "鸡肉"})
WHERE r.cuisine = "川菜"
RETURN r.name, r.difficulty, r.time
```

**优势**：
- 支持复杂关系推理
- 可追溯推理路径
- 高精确性和可解释性

### Q3: 什么是降级策略？如何设计？

**参考答案**：
降级策略是当高级检索策略失败时，自动切换到更低级的策略，确保系统可用性。

**三级降级**：

**Level 3：图 RAG 检索**（最高级）
- 多跳推理 + 子图提取
- 失败 → 降级到 Level 1

**Level 2：组合检索**（中级）
- 融合两种方法
- 失败 → 降级到 Level 1

**Level 1：传统混合检索**（保底）
- 无更低级降级
- 失败 → 系统异常

**降级触发条件**：
- Neo4j 连接失败
- Cypher 查询超时
- 图遍历结果为空

**降级实现**：
```python
try:
    # 尝试图RAG检索
    return graph_rag_retrieve(query, top_k)
except Exception as e:
    logger.warning(f"图RAG检索失败，降级到传统混合检索: {e}")
    # 降级到传统混合检索
    return hybrid_retrieve(query, top_k)
```

**设计原则**：
1. **有限降级**：不能无限降级，必须有保底策略
2. **快速失败**：避免长时间等待超时
3. **日志记录**：记录降级原因，便于分析
4. **监控告警**：监控降级率，及时发现问题

### Q4: 如何实现组合检索策略？

**参考答案**：
组合检索策略同时使用传统检索和图 RAG 检索，融合两者优势。

**实现流程**：

**1. 配额分配**
```python
def split_quota(top_k: int) -> Tuple[int, int]:
    traditional_k = top_k // 2
    graph_k = top_k - traditional_k
    return traditional_k, graph_k
```

**2. 并行执行**
- 同时执行传统检索和图 RAG 检索
- 提升检索速度

**3. Round-robin 合并**
- 交替添加结果
- 图 RAG 优先（更精确）

```python
def round_robin_merge(traditional_results, graph_results, top_k):
    merged = []
    i, j = 0, 0
    turn = "graph"  # 图RAG优先
    
    while len(merged) < top_k and (i < len(traditional_results) or j < len(graph_results)):
        if turn == "graph" and j < len(graph_results):
            merged.append(graph_results[j])
            j += 1
        elif i < len(traditional_results):
            merged.append(traditional_results[i])
            i += 1
        turn = "traditional" if turn == "graph" else "graph"
    
    return merged
```

**4. 去重和排序**
- 基于内容哈希去重
- 按相关性排序

**优势**：
- 兼顾语义理解和关系推理
- 提升召回率和精确率
- 对不同查询类型更鲁棒

### Q5: 如何评估图 RAG 系统的性能？

**参考答案**：
评估图 RAG 系统需要多维度指标：

**1. 检索质量**
- **上下文精确率**：检索到的上下文中真正相关的比例
- **上下文召回率**：应支持答案的证据被检回的比例
- **图路径准确性**：图推理路径是否正确
- **子图完整性**：子图是否包含必要信息

**2. 生成质量**
- **忠实度**：答案是否基于检索到的图证据和文本
- **答案相关性**：是否回答了用户问题
- **可解释性**：是否提供推理路径

**3. 系统性能**
- **推理延迟**：总响应时间
- **吞吐量**：QPS（每秒查询数）
- **成本**：API 调用、计算资源、内存消耗

**4. 路由效果**
- **路由准确率**：选择的策略是否合适
- **降级率**：降级到传统检索的比例
- **各策略平均延迟**：传统、图RAG、组合

**5. 用户反馈**
- **thumbs up/down**：用户直接反馈
- **问题解决率**：用户问题是否得到解决
- **重查询率**：用户是否需要重新查询

**基准测试**：
- 构建测试集：简单查询、复杂查询、推理查询
- 对比不同策略的性能
- A/B 测试优化路由算法

### Q6: 如何优化图 RAG 系统的响应速度？

**参考答案**：
优化图 RAG 系统响应速度的方法：

**1. 查询路由优化**
- 快速识别查询类型
- 避免不必要的复杂检索
- 简单查询用简单策略

**2. 图查询优化**
- **索引优化**：为常用查询模式建立索引
- **查询缓存**：缓存热门查询的图路径
- **限制深度**：最大深度 3 跳，避免无限遍历
- **子图大小限制**：最大 100 节点

**3. 并行处理**
- 传统检索和图检索并行执行
- 批量处理多个查询
- 异步 I/O

**4. 缓存策略**
- **图路径缓存**：缓存热门查询的图遍历路径
- **子图缓存**：缓存常用子图结构
- **向量结果缓存**：缓存向量检索结果

**5. 降级策略**
- 设置超时时间（如 5 秒）
- 超时自动降级到传统检索
- 避免长时间等待

**6. 模型优化**
- 使用更快的嵌入模型
- 量化模型减少计算时间
- GPU 加速

**权衡**：
- 速度 vs 精确率：限制深度和子图大小可能降低精确率
- 成本 vs 性能：并行处理和缓存增加成本

### Q7: 图 RAG 系统在生产环境中面临哪些挑战？

**参考答案**：
图 RAG 系统在生产环境中的挑战：

**1. 图谱构建与维护**
- **挑战**：从非结构化文本构建高质量图谱耗时耗力
- **解决**：增量更新、版本管理、人机协同

**2. 查询性能**
- **挑战**：图遍历可能很慢，尤其是深度查询
- **解决**：
  - 索引优化
  - 查询缓存
  - 限制遍历深度
  - 并行处理

**3. 可扩展性**
- **挑战**：图谱规模增长时性能下降
- **解决**：
  - 分布式图数据库（NebulaGraph）
  - 图分区
  - 负载均衡

**4. 复杂度管理**
- **挑战**：图查询复杂，调试困难
- **解决**：
  - 查询可视化
  - 详细的日志记录
  - 性能监控工具

**5. 成本控制**
- **挑战**：图数据库和 LLM API 成本高
- **解决**：
  - 查询路由（简单查询不用图）
  - 缓存热门查询
  - 降级策略

**6. 数据一致性**
- **挑战**：图谱数据可能与源数据不同步
- **解决**：
  - 定期同步
  - 版本控制
  - 时间戳管理

**7. 错误处理**
- **挑战**：图查询失败、超时、结果为空
- **解决**：
  - 多级降级策略
  - 超时控制
  - 详细的错误日志

### Q8: 传统 RAG、图 RAG、混合检索如何选择？

**参考答案**：
根据查询特征选择最合适的检索策略：

**传统 RAG（向量检索 + BM25）**
- **适用场景**：
  - 简单查找（如"宫保鸡丁怎么做"）
  - 语义相似度匹配
  - 快速响应需求
- **优势**：快速、成本低、覆盖广
- **劣势**：无法处理复杂关系推理

**图 RAG（图检索）**
- **适用场景**：
  - 复杂关系查询（如"哪些菜包含鸡肉且是川菜"）
  - 多跳推理（如"菜谱 → 食材 → 营养成分"）
  - 需要可解释性
- **优势**：精确、可解释、支持推理
- **劣势**：慢、成本高、依赖图谱质量

**混合检索（传统 + 图）**
- **适用场景**：
  - 中等复杂查询
  - 需要平衡性能和精确性
  - 生产环境部署
- **优势**：综合表现好、鲁棒性强
- **劣势**：复杂度高

**选择决策树**：
```
if 复杂度 < 0.4:
    传统 RAG
elif 关系密集度 > 0.7:
    图 RAG
else:
    混合检索
```

**实践建议**：
- 从传统 RAG 开始（保底）
- 逐步引入图 RAG（增强）
- 最终使用混合检索（优化）
- 根据实际效果调整路由策略
