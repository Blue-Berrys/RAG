# Milvus 向量数据库快速上手指南

如果你觉得 Milvus 的使用流程比较复杂，最简单的理解方式是将其类比为**传统关系型数据库（如 MySQL）**，但它是专门为“向量”这种特殊数据类型设计的。

### 核心概念类比

| Milvus 概念 | 传统数据库类比 | 说明 |
| :--- | :--- | :--- |
| **Collection** | **表 (Table)** | 存储数据的主要单位。 |
| **Entity** | **行 (Row)** | 每一条具体的数据记录。 |
| **Field** | **列 (Column)** | 存储特定类型数据的字段（如向量、ID、路径等）。 |
| **Schema** | **表结构定义** | 规定了表里有哪些字段、什么类型。 |
| **Index** | **索引** | 传统数据库索引加速文本查找；Milvus 索引加速向量相似度计算。 |

---

## Milvus 标准操作五步走

在 `04_multi_milvus.py` 中，我们可以将整个流程总结为以下五个关键步骤：

### 1. 初始化连接 (Connection)
在使用任何功能之前，必须先连接到 Milvus 服务器。

```python
from pymilvus import MilvusClient

# 初始化客户端，uri 是 Milvus 服务器地址（通常 Docker 部署为 19530 端口）
milvus_client = MilvusClient(uri="http://localhost:19530")
```

### 2. 定义 Schema 并创建集合 (Schema & Collection)
Milvus 是**强类型**的，你必须先告诉它你的“表”长什么样。

*   **定义字段 (Fields)**：必须包含一个主键（Primary Key）和至少一个向量字段（Vector Field）。
*   **创建集合**：应用这个定义。

```python
from pymilvus import FieldSchema, DataType, CollectionSchema

# 1. 定义字段
fields = [
    # 主键：INT64 类型，设置为自动增长
    FieldSchema(name="id", dtype=DataType.INT64, is_primary=True, auto_id=True),
    # 向量字段：FLOAT_VECTOR 类型，必须指定维度（dim），比如 768 或 1024
    FieldSchema(name="vector", dtype=DataType.FLOAT_VECTOR, dim=768),
    # 标量字段：存储普通数据（如图片路径、文本内容等）
    FieldSchema(name="image_path", dtype=DataType.VARCHAR, max_length=512),
]

# 2. 组合成 Schema
schema = CollectionSchema(fields, description="多模态搜索集合")

# 3. 创建集合
milvus_client.create_collection(collection_name="my_collection", schema=schema)
```

### 3. 数据编码与插入 (Insert)
Milvus 不负责生成向量，你需要用模型（如 BGE、CLIP）将图片或文字转成向量后，再交给 Milvus。

```python
# 数据格式是一个字典列表，Key 要对应 Schema 中的字段名
data = [
    {"vector": [0.12, 0.05, ...], "image_path": "data/img1.png"},
    {"vector": [0.31, 0.22, ...], "image_path": "data/img2.png"},
]

milvus_client.insert(collection_name="my_collection", data=data)
```

### 4. 创建索引并加载 (Index & Load) —— **这是最容易忘的一步**
*   **创建索引**：如果不建索引，Milvus 只能进行全表扫描（暴力搜索），速度极慢。
*   **加载集合**：创建索引后，必须将数据“激活”到内存中，才能搜索。

```python
# 配置索引参数
index_params = milvus_client.prepare_index_params()
index_params.add_index(
    field_name="vector",
    index_type="HNSW",      # 常用索引类型，速度快、召回率高
    metric_type="COSINE",   # 相似度度量，COSINE 表示余弦相似度
    params={"M": 16, "efConstruction": 256}
)

# 1. 创建索引
milvus_client.create_index(collection_name="my_collection", index_params=index_params)

# 2. 加载到内存（必须步骤！）
milvus_client.load_collection(collection_name="my_collection")
```

### 5. 执行检索 (Search)
一切准备就绪后，就可以传入一个查询向量，找出最相似的结果。

```python
search_results = milvus_client.search(
    collection_name="my_collection",
    data=[query_vector],           # 传入你的查询向量
    limit=5,                       # 返回前 5 个最像的结果
    output_fields=["image_path"],  # 指定返回哪些原始字段
    search_params={"metric_type": "COSINE", "params": {"ef": 128}}
)

# 解析结果
for hit in search_results[0]:
    print(f"ID: {hit['id']}, 相似度: {hit['distance']}, 路径: {hit['entity']['image_path']}")
```

---

## 为什么你觉得 Milvus 复杂？（常见痛点解释）

1.  **为什么一定要建索引？**
    向量数据的搜索不是简单的“等于”判断，而是复杂的数学计算。如果不通过 `HNSW` 这种算法建立“导航图”，搜索海量向量会拖垮服务器。

2.  **为什么搜索前要 `load`？**
    Milvus 为了节省资源，默认数据是存在硬盘上的。搜索需要极高的性能，因此需要手动调用 `load_collection` 将数据索引加载到内存中。

3.  **距离度量 (Metric Type) 怎么选？**
    *   `COSINE` (余弦相似度)：关注向量的方向，最适合文本语义搜索和图像搜索。
    *   `L2` (欧氏距离)：关注数值的绝对距离，数值越小越相似。
    *   `IP` (内积)：通常用于需要考虑向量长度的场景。

4.  **HNSW 的参数是什么意思？**
    *   `M`：图中每个点连多少条线。越大越准，但费内存。
    *   `efConstruction`：建图时搜索多深。越大索引质量越高，但建索引慢。
    *   `ef`：搜索时看多少个点。越大越准，但搜索变慢。

