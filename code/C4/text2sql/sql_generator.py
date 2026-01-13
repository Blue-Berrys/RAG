# 导入操作系统模块
import os
# 导入类型提示工具
from typing import List, Dict, Any
# 导入 DeepSeek 聊天模型接口（基于 LangChain）
from langchain_deepseek import ChatDeepSeek
# 导入 LangChain 的消息架构：HumanMessage（用户消息）
from langchain.schema import HumanMessage, SystemMessage


class SimpleSQLGenerator:
    """简化的SQL生成器类：负责与大模型交互，生成和修复 SQL 语句"""
    
    def __init__(self, api_key: str = None):
        """
        初始化生成器
        Args:
            api_key: DeepSeek API 密钥，若不提供则从环境变量获取
        """
        self.llm = ChatDeepSeek(
            model="deepseek-chat",   # 使用 DeepSeek Chat 模型
            temperature=0,           # 设置温度为 0，确保生成的 SQL 具有高确定性和一致性
            api_key=api_key or os.getenv("DEEPSEEK_API_KEY")
        )
    
    def generate_sql(self, user_query: str, knowledge_results: List[Dict[str, Any]]) -> str:
        """
        根据用户问题和检索到的知识生成 SQL
        Args:
            user_query: 用户提出的自然语言问题
            knowledge_results: 从知识库检索到的相关 DDL、描述和示例
        """
        # 构建上下文信息：将检索到的 DDL 和示例格式化为字符串
        context = self._build_context(knowledge_results)
        
        # 构建提示词：指导大模型扮演 SQL 专家角色
        prompt = f"""你是一个SQL专家。请根据以下信息将用户问题转换为SQL查询语句。

数据库信息：
{context}

用户问题：{user_query}

要求：
1. 只返回SQL语句，不要包含任何解释
2. 确保SQL语法正确
3. 使用上下文中提供的表名和字段名
4. 如果需要JOIN，请根据表结构进行合理关联

SQL语句："""

        # 调用大模型进行推理
        messages = [HumanMessage(content=prompt)]
        response = self.llm.invoke(messages)
        
        # 清理返回结果中的代码块标记
        sql = response.content.strip()
        if sql.startswith("```sql"):
            sql = sql[6:]
        if sql.startswith("```"):
            sql = sql[3:]
        if sql.endswith("```"):
            sql = sql[:-3]
        
        return sql.strip()
    
    def fix_sql(self, original_sql: str, error_message: str, knowledge_results: List[Dict[str, Any]]) -> str:
        """
        当生成的 SQL 执行失败时，利用错误信息进行修复
        Args:
            original_sql: 之前生成的错误的 SQL
            error_message: 数据库返回的错误详情
            knowledge_results: 相关的知识库信息
        """
        context = self._build_context(knowledge_results)
        
        # 构建修复提示词
        prompt = f"""请修复以下SQL语句的错误。

数据库信息：
{context}

原始SQL：
{original_sql}

错误信息：
{error_message}

请返回修复后的SQL语句（只返回SQL，不要解释）："""

        # 调用大模型执行修复
        messages = [HumanMessage(content=prompt)]
        response = self.llm.invoke(messages)
        
        # 清理返回结果
        fixed_sql = response.content.strip()
        if fixed_sql.startswith("```sql"):
            fixed_sql = fixed_sql[6:]
        if fixed_sql.startswith("```"):
            fixed_sql = fixed_sql[3:]
        if fixed_sql.endswith("```"):
            fixed_sql = fixed_sql[:-3]
        
        return fixed_sql.strip()
    
    def _build_context(self, knowledge_results: List[Dict[str, Any]]) -> str:
        """构建上下文信息：按类型分组并拼接文本"""
        context = ""
        
        # 按类型分组
        ddl_info = []
        qsql_examples = []
        descriptions = []
        
        for result in knowledge_results:
            if result["type"] == "ddl":
                ddl_info.append(result["content"])
            elif result["type"] == "qsql":
                qsql_examples.append(result["content"])
            elif result["type"] == "description":
                descriptions.append(result["content"])
        
        # 分板块构建最终文本
        if ddl_info:
            context += "=== 表结构信息 ===\n"
            context += "\n".join(ddl_info) + "\n\n"
        
        if descriptions:
            context += "=== 表和字段描述 ===\n"
            context += "\n".join(descriptions) + "\n\n"
        
        if qsql_examples:
            context += "=== 查询示例 ===\n"
            context += "\n".join(qsql_examples) + "\n\n"
        
        return context
