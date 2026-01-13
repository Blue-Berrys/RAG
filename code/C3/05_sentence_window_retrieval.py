# 导入操作系统相关功能的模块
import os
# 导入 LlamaIndex 节点解析器：SentenceWindowNodeParser 用于创建句子窗口，SentenceSplitter 用于常规分块
from llama_index.core.node_parser import SentenceWindowNodeParser, SentenceSplitter
# 导入核心组件：向量存储索引、目录读取器、全局设置
from llama_index.core import VectorStoreIndex, SimpleDirectoryReader, Settings
# 导入 DeepSeek LLM 接口
from llama_index.llms.deepseek import DeepSeek
# 导入 HuggingFace 嵌入模型接口
from llama_index.embeddings.huggingface import HuggingFaceEmbedding
# 导入元数据替换后处理器，这是句子窗口检索的关键
from llama_index.core.postprocessor import MetadataReplacementPostProcessor

# ==================== 1. 配置模型 ====================
# 配置全局 LLM：使用 DeepSeek 模型，设置温度为 0.1 以获得更稳定的输出
Settings.llm = DeepSeek(model="deepseek-chat", temperature=0.1, api_key="sk-5de384802e84440c8d332ab1f9bbd860")
# 配置全局嵌入模型：使用 BGE 基础英文小模型
Settings.embed_model = HuggingFaceEmbedding(model_name="BAAI/bge-small-en")

# ==================== 2. 加载文档 ====================
# 使用 SimpleDirectoryReader 加载指定的 PDF 文档
documents = SimpleDirectoryReader(
    input_files=["../../data/C3/pdf/IPCC_AR6_WGII_Chapter03.pdf"]
).load_data()

# ==================== 3. 创建节点与构建索引 ====================
# --- 3.1 句子窗口索引 (高级技术) ---
# 初始化句子窗口节点解析器
node_parser = SentenceWindowNodeParser.from_defaults(
    window_size=3,                # 窗口大小为 3：即检索到一个句子时，会自动包含其前后各 3 个句子
    window_metadata_key="window", # 存储上下文窗口文本的元数据键名
    original_text_metadata_key="original_text", # 存储原始单句文本的元数据键名
)
# 从文档中提取句子窗口节点
sentence_nodes = node_parser.get_nodes_from_documents(documents)
# 构建基于句子窗口的向量索引
sentence_index = VectorStoreIndex(sentence_nodes)

# --- 3.2 常规分块索引 (作为基准对比) ---
# 使用标准的长文本分块解析器
base_parser = SentenceSplitter(chunk_size=512)
# 从文档中提取常规节点
base_nodes = base_parser.get_nodes_from_documents(documents)
# 构建基于常规分块的向量索引
base_index = VectorStoreIndex(base_nodes)

# ==================== 4. 构建查询引擎 ====================
# 构建句子窗口查询引擎
sentence_query_engine = sentence_index.as_query_engine(
    similarity_top_k=2, # 检索最相似的 2 个节点
    node_postprocessors=[
        # 核心步骤：在检索后，将单句文本替换为元数据中保存的“窗口”文本（包含上下文的更长文本）
        MetadataReplacementPostProcessor(target_metadata_key="window")
    ],
)
# 构建常规查询引擎
base_query_engine = base_index.as_query_engine(similarity_top_k=2)

# ==================== 5. 执行查询并对比结果 ====================
# 定义查询问题：关于 AMOC (大西洋经向翻转环流) 的担忧
query = "What are the concerns surrounding the AMOC?"
print(f"查询: {query}\n")

# 测试句子窗口检索
print("--- 句子窗口检索结果 ---")
window_response = sentence_query_engine.query(query)
print(f"回答: {window_response}\n")

# 测试常规检索
print("--- 常规检索结果 ---")
base_response = base_query_engine.query(query)
print(f"回答: {base_response}\n")
