# 导入 JSON 处理模块
import json
# 导入操作系统功能模块
import os
# 导入类型提示相关的工具
from typing import List, Dict, Any
# 导入 Milvus 客户端及相关数据结构
from pymilvus import MilvusClient, FieldSchema, CollectionSchema, DataType
# 导入 BGE-M3 嵌入模型函数，用于生成文本向量
from pymilvus.model.hybrid import BGEM3EmbeddingFunction


class SimpleKnowledgeBase:
    """Text2SQL 知识库类：负责管理向量数据的存储与检索"""
    
    def __init__(self, milvus_uri: str = "http://localhost:19530"):
        """
        初始化知识库
        Args:
            milvus_uri: Milvus 服务器连接地址
        """
        self.milvus_uri = milvus_uri
        # 创建 Milvus 客户端连接
        self.client = MilvusClient(uri=milvus_uri)
        # 初始化嵌入模型，使用 CPU 进行推理
        self.embedding_function = BGEM3EmbeddingFunction(use_fp16=False, device="cpu")
        # 定义集合名称
        self.collection_name = "text2sql_kb"
        # 执行集合的初始化设置
        self._setup_collection()
    
    def _setup_collection(self):
        """设置并创建 Milvus 集合"""
        # 如果集合已存在，则先删除以便重新初始化
        if self.client.has_collection(self.collection_name):
            self.client.drop_collection(self.collection_name)
        
        # 定义字段模式 (Schema)
        fields = [
            # 主键字段：字符串类型，自动生成 ID
            FieldSchema(name="pk", dtype=DataType.VARCHAR, is_primary=True, auto_id=True, max_length=100),
            # 内容字段：存储原始文本信息
            FieldSchema(name="content", dtype=DataType.VARCHAR, max_length=4096),
            # 类型字段：标识内容类型（ddl-表定义, qsql-问答对, description-表描述）
            FieldSchema(name="type", dtype=DataType.VARCHAR, max_length=32),
            # 向量字段：存储由嵌入模型生成的密集向量
            FieldSchema(name="dense_vector", dtype=DataType.FLOAT_VECTOR, dim=self.embedding_function.dim["dense"])
        ]
        
        # 创建集合 Schema 对象
        schema = CollectionSchema(fields, description="Text2SQL知识库")
        
        # 在 Milvus 中创建集合，设置一致性级别为 Strong（强一致性）
        self.client.create_collection(
            collection_name=self.collection_name,
            schema=schema,
            consistency_level="Strong"
        )
        
        # 配置索引参数
        index_params = self.client.prepare_index_params()
        index_params.add_index(
            field_name="dense_vector",
            index_type="AUTOINDEX",      # 自动选择最优索引类型
            metric_type="IP"             # 使用内积 (Inner Product) 计算相似度
        )
        
        # 为向量字段创建索引以加速检索
        self.client.create_index(
            collection_name=self.collection_name,
            index_params=index_params
        )
    
    def load_data(self):
        """从本地 JSON 文件加载所有知识库原始数据"""
        # 获取当前文件所在目录下的 data 文件夹路径
        data_dir = os.path.join(os.path.dirname(__file__), "data")
        
        # 1. 加载 DDL 数据（表结构定义）
        ddl_path = os.path.join(data_dir, "ddl_examples.json")
        if os.path.exists(ddl_path):
            with open(ddl_path, 'r', encoding='utf-8') as f:
                ddl_data = json.load(f)
            self._add_ddl_data(ddl_data)
        
        # 2. 加载 Q->SQL 数据（自然语言问题与 SQL 语句的对应示例）
        qsql_path = os.path.join(data_dir, "qsql_examples.json")
        if os.path.exists(qsql_path):
            with open(qsql_path, 'r', encoding='utf-8') as f:
                qsql_data = json.load(f)
            self._add_qsql_data(qsql_data)
        
        # 3. 加载表和列的描述数据（业务语义信息）
        desc_path = os.path.join(data_dir, "db_descriptions.json")
        if os.path.exists(desc_path):
            with open(desc_path, 'r', encoding='utf-8') as f:
                desc_data = json.load(f)
            self._add_description_data(desc_data)
        
        # 数据插入后，必须将集合加载到内存中才能执行检索
        self.client.load_collection(collection_name=self.collection_name)
        print("知识库数据加载完成")
    
    def _add_ddl_data(self, data: List[Dict]):
        """处理并添加 DDL 类型的数据"""
        contents = []
        types = []
        
        for item in data:
            # 拼接格式化的 DDL 描述文本
            content = f"表名: {item.get('table_name', '')}\n"
            content += f"DDL: {item.get('ddl_statement', '')}\n"
            content += f"描述: {item.get('description', '')}"
            
            contents.append(content)
            types.append("ddl")
        
        self._insert_data(contents, types)
    
    def _add_qsql_data(self, data: List[Dict]):
        """处理并添加 问答对 (Q->SQL) 类型的数据"""
        contents = []
        types = []
        
        for item in data:
            # 拼接问题和对应的正确 SQL 语句
            content = f"问题: {item.get('question', '')}\n"
            content += f"SQL: {item.get('sql', '')}"
            
            contents.append(content)
            types.append("qsql")
        
        self._insert_data(contents, types)
    
    def _add_description_data(self, data: List[Dict]):
        """处理并添加数据库语义描述信息"""
        contents = []
        types = []
        
        for item in data:
            # 拼接表级和列级的描述信息，帮助 LLM 理解业务含义
            content = f"表名: {item.get('table_name', '')}\n"
            content += f"表描述: {item.get('table_description', '')}\n"
            
            columns = item.get('columns', [])
            if columns:
                content += "字段信息:\n"
                for col in columns:
                    content += f"  - {col.get('name', '')}: {col.get('description', '')} ({col.get('type', '')})\n"
            
            contents.append(content)
            types.append("description")
        
        self._insert_data(contents, types)
    
    def _insert_data(self, contents: List[str], types: List[str]):
        """通用的数据插入方法，包含向量化过程"""
        if not contents:
            return
        
        # 调用嵌入模型，将批量文本转换为高维向量
        embeddings = self.embedding_function(contents)
        
        # 组装待插入 Milvus 的字典列表
        data_to_insert = []
        for i in range(len(contents)):
            data_to_insert.append({
                "content": contents[i],
                "type": types[i],
                "dense_vector": embeddings["dense"][i]
            })
        
        # 批量插入数据到 Milvus
        result = self.client.insert(
            collection_name=self.collection_name,
            data=data_to_insert
        )
    
    def search(self, query: str, top_k: int = 5) -> List[Dict[str, Any]]:
        """
        基于向量相似度搜索与问题最相关的内容
        Args:
            query: 用户输入的自然语言问题
            top_k: 返回最相关的条数
        """
        # 搜索前确保集合已加载到内存
        self.client.load_collection(collection_name=self.collection_name)
            
        # 将用户查询文本转换为向量
        query_embeddings = self.embedding_function([query])
        
        # 在向量字段执行相似度检索
        search_results = self.client.search(
            collection_name=self.collection_name,
            data=query_embeddings["dense"],
            anns_field="dense_vector",
            search_params={"metric_type": "IP"}, # 使用内积度量
            limit=top_k,
            output_fields=["content", "type"]     # 指定返回结果中包含的字段
        )
        
        # 解析并格式化搜索结果
        results = []
        for hit in search_results[0]:
            results.append({
                "content": hit["entity"]["content"],
                "type": hit["entity"]["type"],
                "score": hit["distance"] # 相似度得分
            })
        
        return results
    
    def cleanup(self):
        """清理资源：删除创建的向量集合"""
        try:
            self.client.drop_collection(self.collection_name)
        except:
            pass 
 