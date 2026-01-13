# 导入操作系统模块
import os
# 导入 pandas 用于处理 Excel 和数据帧
import pandas as pd
# 导入 dotenv 用于加载环境变量
from dotenv import load_dotenv
# 导入向量存储索引
from llama_index.core import VectorStoreIndex
# 导入索引节点，用于在节点中链接其他对象（如查询引擎）
from llama_index.core.schema import IndexNode
# 导入专门用于处理 Pandas 数据帧的查询引擎
from llama_index.experimental.query_engine import PandasQueryEngine
# 导入递归检索器，支持在检索过程中跳转到其他检索器或查询引擎
from llama_index.core.retrievers import RecursiveRetriever
# 导入基于检索器的查询引擎包装器
from llama_index.core.query_engine import RetrieverQueryEngine
# 导入 DeepSeek LLM 接口
from llama_index.llms.deepseek import DeepSeek
# 导入 HuggingFace 嵌入模型接口
from llama_index.embeddings.huggingface import HuggingFaceEmbedding
# 导入全局设置
from llama_index.core import Settings

# 加载 .env 文件中的环境变量
load_dotenv()

# ==================== 配置模型 ====================
# 配置全局大模型：使用 DeepSeek 聊天模型
Settings.llm = DeepSeek(model="deepseek-chat", api_key="sk-5de384802e84440c8d332ab1f9bbd860")
# 配置全局嵌入模型：使用 BGE 中文小模型，支持更好的中文语义理解
Settings.embed_model = HuggingFaceEmbedding(model_name="BAAI/bge-small-zh-v1.5")

# ==================== 1. 加载数据并为每个工作表创建查询引擎和摘要节点 ====================
# Excel 文件路径，包含多个年份的电影数据
excel_file = '../../data/C3/excel/movie.xlsx'
# 使用 pandas 加载 Excel 文件对象
xls = pd.ExcelFile(excel_file)

# 用于存储工作表名称到对应查询引擎的映射
df_query_engines = {}
# 用于存储所有的索引节点（摘要节点）
all_nodes = []

# 遍历 Excel 中的每一个工作表
for sheet_name in xls.sheet_names:
    # 读取当前工作表的数据
    df = pd.read_excel(xls, sheet_name=sheet_name)
    
    # 为当前数据帧创建一个 PandasQueryEngine，允许通过自然语言查询表格数据
    # verbose=True 会打印出生成的 Python 代码，便于调试
    query_engine = PandasQueryEngine(df=df, llm=Settings.llm, verbose=True)
    
    # 提取年份信息并创建一个描述性的摘要
    year = sheet_name.replace('年份_', '')
    summary = f"这个表格包含了年份为 {year} 的电影信息，可以用来回答关于这一年电影的具体问题。"
    
    # 创建一个 IndexNode：
    # - text: 存储该表的摘要，供顶层索引进行向量检索
    # - index_id: 关键标识符，用于递归检索时找到对应的底层查询引擎
    node = IndexNode(text=summary, index_id=sheet_name)
    all_nodes.append(node)
    
    # 建立 ID 到查询引擎的映射关系
    df_query_engines[sheet_name] = query_engine

# ==================== 2. 创建顶层索引 ====================
# 将所有摘要节点构建成向量索引。此时索引中只包含“摘要”，不包含具体的表格行数据
vector_index = VectorStoreIndex(all_nodes)

# ==================== 3. 创建递归检索器 ====================
# 3.1 创建顶层检索器：先在摘要节点中寻找最匹配的一张表
vector_retriever = vector_index.as_retriever(similarity_top_k=1)

# 3.2 创建递归检索器
# 逻辑：当 vector_retriever 检索到一个 IndexNode 时，
# RecursiveRetriever 会根据 node.index_id 在 query_engine_dict 中查找并执行对应的查询引擎
recursive_retriever = RecursiveRetriever(
    "vector", # 默认检索器的标识符
    retriever_dict={"vector": vector_retriever}, # 包含顶层检索器的字典
    query_engine_dict=df_query_engines,           # 包含底层工作表查询引擎的字典
    verbose=True,                                 # 打印跳转过程
)

# ==================== 4. 创建最终查询引擎 ====================
# 将递归检索逻辑包装成一个完整的查询引擎
query_engine = RetrieverQueryEngine.from_args(recursive_retriever)

# ==================== 5. 执行查询 ====================
# 提出一个跨表/针对具体年份的问题
query = "1994年评分人数最少的电影是哪一部？"
print(f"查询: {query}")
# 执行查询：系统会先识别出需要查 1994 年的表，然后调用该表的 Pandas 查询引擎
response = query_engine.query(query)
print(f"回答: {response}")
