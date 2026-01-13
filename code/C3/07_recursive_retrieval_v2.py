# 导入操作系统模块
import os
# 导入 pandas 用于处理 Excel 数据
import pandas as pd
# 导入 dotenv 用于加载环境变量
from dotenv import load_dotenv
# 导入 LlamaIndex 核心组件：索引、文档对象和全局设置
from llama_index.core import VectorStoreIndex, Document, Settings
# 导入向量索引检索器
from llama_index.core.retrievers import VectorIndexRetriever
# 导入基于检索器的查询引擎
from llama_index.core.query_engine import RetrieverQueryEngine
# 导入元数据过滤器，用于精确匹配特定工作表
from llama_index.core.vector_stores import MetadataFilters, ExactMatchFilter
# 导入 DeepSeek LLM 接口
from llama_index.llms.deepseek import DeepSeek
# 导入 HuggingFace 嵌入模型接口
from llama_index.embeddings.huggingface import HuggingFaceEmbedding

# 加载环境变量（如 API Key）
load_dotenv()

# ==================== 配置模型 ====================
# 配置全局 LLM：使用 DeepSeek 聊天模型
Settings.llm = DeepSeek(model="deepseek-chat", api_key="sk-5de384802e84440c8d332ab1f9bbd860")
# 配置全局嵌入模型：使用 BGE 中文小模型，适合中文语义匹配
Settings.embed_model = HuggingFaceEmbedding(model_name="BAAI/bge-small-zh-v1.5")

# ==================== 1. 加载和预处理数据 ====================
# Excel 文件路径
excel_file = '../../data/C3/excel/movie.xlsx'
# 使用 pandas 读取 Excel
xls = pd.ExcelFile(excel_file)

# 存储摘要文档（用于“路由”：决定去哪个表查）
summary_docs = []
# 存储内容文档（用于“检索”：获取表内具体信息）
content_docs = []

print("开始加载和处理Excel文件...")
# 遍历 Excel 中的每一个工作表
for sheet_name in xls.sheet_names:
    df = pd.read_excel(xls, sheet_name=sheet_name)
    
    # --- 数据清洗 ---
    # 处理“评分人数”列，将“123人评价”这种格式转换为纯数字
    if '评分人数' in df.columns:
        df['评分人数'] = df['评分人数'].astype(str).str.replace('人评价', '').str.strip()
        # 转换为数值类型，无法转换的变为 0
        df['评分人数'] = pd.to_numeric(df['评分人数'], errors='coerce').fillna(0).astype(int)

    # --- 创建摘要文档 (用于路由) ---
    # 提取年份
    year = sheet_name.replace('年份_', '')
    # 构建描述性文本，告诉 AI 这个表里有什么
    summary_text = f"这个表格包含了年份为 {year} 的电影信息，包括电影名称、导演、评分、评分人数等。"
    summary_doc = Document(
        text=summary_text,
        metadata={"sheet_name": sheet_name} # 在元数据中保存工作表名称，方便后续关联
    )
    summary_docs.append(summary_doc)
    
    # --- 创建内容文档 (用于最终问答) ---
    # 将整个 DataFrame 转换为字符串作为文档内容
    content_text = df.to_string(index=False)
    content_doc = Document(
        text=content_text,
        metadata={"sheet_name": sheet_name} # 同样保存工作表名称，用于元数据过滤
    )
    content_docs.append(content_doc)

print("数据加载和处理完成。\n")

# ==================== 2. 构建向量索引 ====================
# 2.1 为摘要创建索引：这相当于“目录索引”
summary_index = VectorStoreIndex(summary_docs)

# 2.2 为内容创建索引：这存储了所有表的具体数据
content_index = VectorStoreIndex(content_docs)

print("摘要索引和内容索引构建完成。\n")

# ==================== 3. 定义两步式查询逻辑 ====================
def query_safe_recursive(query_str):
    """
    执行两步式检索逻辑：
    1. 在摘要索引中通过语义匹配找到最相关的年份工作表。
    2. 在内容索引中使用元数据过滤，只检索该工作表的内容并由 LLM 回答。
    """
    print(f"--- 开始执行查询 ---")
    print(f"查询: {query_str}")
    
    # --- 第一步：路由 (Routing) ---
    print("\n第一步：在摘要索引中进行路由...")
    # 创建只检索 1 个最相似节点的检索器
    summary_retriever = VectorIndexRetriever(index=summary_index, similarity_top_k=1)
    retrieved_nodes = summary_retriever.retrieve(query_str)
    
    if not retrieved_nodes:
        return "抱歉，未能找到相关的电影年份信息。"
    
    # 从匹配到的节点中提取工作表名称（如“年份_1994”）
    matched_sheet_name = retrieved_nodes[0].node.metadata['sheet_name']
    print(f"路由结果：匹配到工作表 -> {matched_sheet_name}")
    
    # --- 第二步：检索 (Retrieval) ---
    print("\n第二步：在内容索引中检索具体信息...")
    # 创建带过滤器的检索器：只搜索 sheet_name 等于匹配到的名称的文档
    content_retriever = VectorIndexRetriever(
        index=content_index,
        similarity_top_k=1, # 匹配整个表格内容
        filters=MetadataFilters(
            filters=[ExactMatchFilter(key="sheet_name", value=matched_sheet_name)]
        )
    )
    
    # 将带过滤器的检索器包装成查询引擎
    query_engine = RetrieverQueryEngine.from_args(content_retriever)
    # 执行查询
    response = query_engine.query(query_str)
    
    print("--- 查询执行结束 ---\n")
    return response

# ==================== 4. 执行查询并展示结果 ====================
query = "1994年评分人数最少的电影是哪一部？"
response = query_safe_recursive(query)

print(f"最终回答: {response}")
