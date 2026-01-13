# 导入 SQLite3 数据库模块
import sqlite3
# 导入操作系统功能模块
import os
# 导入类型提示相关的工具
from typing import Dict, Any, List, Tuple
# 导入自定义的简单知识库模块
from .knowledge_base import SimpleKnowledgeBase
# 导入自定义的简单 SQL 生成器模块
from .sql_generator import SimpleSQLGenerator


class SimpleText2SQLAgent:
    """Text2SQL 代理类：负责协调知识库检索、SQL 生成与执行"""
    
    def __init__(self, milvus_uri: str = "http://localhost:19530", api_key: str = None):
        """
        初始化代理
        Args:
            milvus_uri: Milvus 数据库连接地址，用于知识库检索
            api_key: LLM API 密钥，用于 SQL 生成
        """
        # 初始化知识库组件（基于 Milvus）
        self.knowledge_base = SimpleKnowledgeBase(milvus_uri)
        # 初始化 SQL 生成器组件（基于 LLM）
        self.sql_generator = SimpleSQLGenerator(api_key)
        # 数据库路径和连接初始化
        self.db_path = None
        self.connection = None
        
        # 核心配置参数
        self.max_retry_count = 3       # SQL 执行失败后的最大重试次数
        self.top_k_retrieval = 5       # 知识库检索时返回的相关信息条数
        self.max_result_rows = 100     # 查询结果返回的最大行数，防止内存溢出
    
    def connect_database(self, db_path: str) -> bool:
        """连接指定的 SQLite 数据库"""
        try:
            self.db_path = db_path
            # 建立数据库连接
            self.connection = sqlite3.connect(db_path)
            print(f"成功连接到数据库: {db_path}")
            return True
        except Exception as e:
            print(f"数据库连接失败: {str(e)}")
            return False
    
    def load_knowledge_base(self):
        """加载知识库数据（将 Schema 和 示例数据等加载到向量数据库）"""
        self.knowledge_base.load_data()
    
    def query(self, user_question: str) -> Dict[str, Any]:
        """
        执行完整的 Text2SQL 查询流程
        流程：用户问题 -> 检索相关知识 -> 生成 SQL -> 执行 SQL -> (失败则修复并重试) -> 返回结果
        """
        if not self.connection:
            return {
                "success": False,
                "error": "数据库未连接",
                "sql": None,
                "results": None
            }
        
        print(f"\n=== 处理查询: {user_question} ===")
        
        # 1. 检索阶段：从知识库中检索与问题相关的表结构（Schema）和示例
        print("检索知识库...")
        knowledge_results = self.knowledge_base.search(user_question, self.top_k_retrieval)
        print(f"检索到 {len(knowledge_results)} 条相关信息")
        
        # 2. 生成阶段：利用 LLM 将用户问题和检索到的知识转换为 SQL 语句
        print("生成SQL...")
        sql = self.sql_generator.generate_sql(user_question, knowledge_results)
        print(f"生成的SQL: {sql}")
        
        # 3. 执行阶段：执行 SQL 并带有自动修复重试逻辑
        retry_count = 0
        while retry_count < self.max_retry_count:
            print(f"执行SQL (尝试 {retry_count + 1}/{self.max_retry_count})...")
            
            # 执行底层的 SQL 运行
            success, result = self._execute_sql(sql)
            
            if success:
                # 执行成功，直接返回结果
                print("SQL执行成功!")
                return {
                    "success": True,
                    "error": None,
                    "sql": sql,
                    "results": result,
                    "retry_count": retry_count
                }
            else:
                # 执行失败，进入修复流程
                print(f"SQL执行失败: {result}")
                
                if retry_count < self.max_retry_count - 1:
                    print("尝试修复SQL...")
                    # 调用生成器的 fix_sql 方法，传入错误信息进行针对性修复
                    sql = self.sql_generator.fix_sql(sql, result, knowledge_results)
                    print(f"修复后的SQL: {sql}")
                
                retry_count += 1
        
        # 达到最大重试次数仍失败
        return {
            "success": False,
            "error": f"超过最大重试次数 ({self.max_retry_count})",
            "sql": sql,
            "results": None,
            "retry_count": retry_count
        }
    
    def _execute_sql(self, sql: str) -> Tuple[bool, Any]:
        """执行单条 SQL 语句并格式化返回结果"""
        try:
            cursor = self.connection.cursor()
            
            # 安全防护：对于 SELECT 查询，如果未指定 LIMIT，则自动添加，防止数据量过大
            if sql.strip().upper().startswith('SELECT') and 'LIMIT' not in sql.upper():
                sql = f"{sql.rstrip(';')} LIMIT {self.max_result_rows}"
            
            # 在数据库中执行 SQL
            cursor.execute(sql)
            
            # 判断是否为查询语句（返回数据集）
            if sql.strip().upper().startswith('SELECT'):
                # 获取列名信息
                columns = [desc[0] for desc in cursor.description]
                # 获取所有结果行
                rows = cursor.fetchall()
                
                # 将结果转换为字典列表格式，方便后续处理
                results = []
                for row in rows:
                    result_row = {}
                    for i, value in enumerate(row):
                        result_row[columns[i]] = value
                    results.append(result_row)
                
                cursor.close()
                return True, {
                    "columns": columns,
                    "rows": results,
                    "count": len(results)
                }
            else:
                # 对于非查询语句（如 INSERT, UPDATE），提交更改
                self.connection.commit()
                cursor.close()
                return True, "SQL执行成功"
        
        except Exception as e:
            # 返回错误标志和异常信息（用于 SQL 修复逻辑）
            return False, str(e)
    
    def add_example(self, question: str, sql: str):
        """将高质量的 用户问题->SQL 对应关系保存为示例，用于 Few-shot 学习"""
        # 确定示例文件的保存路径
        data_dir = os.path.join(os.path.dirname(__file__), "data")
        qsql_path = os.path.join(data_dir, "qsql_examples.json")
        
        try:
            import json
            
            # 读取现有的示例数据
            if os.path.exists(qsql_path):
                with open(qsql_path, 'r', encoding='utf-8') as f:
                    data = json.load(f)
            else:
                data = []
            
            # 追加新的示例
            data.append({
                "question": question,
                "sql": sql,
                "database": "sqlite"
            })
            
            # 保存回 JSON 文件
            with open(qsql_path, 'w', encoding='utf-8') as f:
                json.dump(data, f, ensure_ascii=False, indent=2)
            
            print(f"已添加新示例: {question}")
            
        except Exception as e:
            print(f"添加示例失败: {str(e)}")
    
    def get_table_info(self) -> List[Dict[str, Any]]:
        """从数据库元数据中提取所有表的 Schema 信息（表名、列名、类型、约束等）"""
        if not self.connection:
            return []
        
        try:
            cursor = self.connection.cursor()
            
            # 获取 SQLite 中所有的表名
            cursor.execute("SELECT name FROM sqlite_master WHERE type='table'")
            tables = cursor.fetchall()
            
            table_info = []
            for table in tables:
                table_name = table[0]
                
                # 使用 PRAGMA 命令获取每个表的详细结构信息
                cursor.execute(f"PRAGMA table_info({table_name})")
                columns = cursor.fetchall()
                
                table_info.append({
                    "table_name": table_name,
                    "columns": [
                        {
                            "name": col[1],
                            "type": col[2],
                            "nullable": not col[3],
                            "default": col[4],
                            "primary_key": bool(col[5])
                        }
                        for col in columns
                    ]
                })
            
            cursor.close()
            return table_info
            
        except Exception as e:
            print(f"获取表信息失败: {str(e)}")
            return []
    
    def cleanup(self):
        """释放所有资源（数据库连接和知识库连接）"""
        if self.connection:
            self.connection.close()
            self.connection = None
            print("数据库连接已关闭")
        
        # 清理知识库中的向量库连接
        self.knowledge_base.cleanup()
        print("知识库已清理") 
 