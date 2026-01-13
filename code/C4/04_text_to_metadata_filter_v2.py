# 导入操作系统相关功能的模块
import os
# 导入 DeepSeek 聊天模型接口（虽然代码后续使用了 OpenAI SDK 调用）
from langchain_deepseek import ChatDeepSeek 
# 导入 Bilibili 视频加载器，用于抓取视频信息
from langchain_community.document_loaders import BiliBiliLoader
# 导入属性信息类，用于描述元数据字段
from langchain.chains.query_constructor.base import AttributeInfo
# 导入 OpenAI SDK，用于调用 DeepSeek API
from openai import OpenAI
# 导入 Chroma 向量数据库
from langchain_community.vectorstores import Chroma
# 导入 HuggingFace 嵌入模型接口
from langchain_huggingface import HuggingFaceEmbeddings
# 导入日志模块
import logging

# 配置日志级别为 INFO
logging.basicConfig(level=logging.INFO)

# ==================== 1. 初始化并加载视频数据 ====================
# 定义要加载的 Bilibili 视频链接列表
video_urls = [
    "https://www.bilibili.com/video/BV1Bo4y1A7FU", 
    "https://www.bilibili.com/video/BV1ug4y157xA",
    "https://www.bilibili.com/video/BV1yh411V7ge",
]

bili = []
try:
    # 使用 BiliBiliLoader 加载视频元数据
    loader = BiliBiliLoader(video_urls=video_urls)
    docs = loader.load()
    
    # 遍历加载到的文档，对元数据进行清洗和标准化
    for doc in docs:
        original = doc.metadata
        
        # 提取关键字段，方便后续根据这些字段进行排序
        metadata = {
            'title': original.get('title', '未知标题'),
            'author': original.get('owner', {}).get('name', '未知作者'),
            'source': original.get('bvid', '未知ID'),
            'view_count': original.get('stat', {}).get('view', 0), # 观看次数
            'length': original.get('duration', 0),                # 视频时长（秒）
        }
        
        # 将清洗后的元数据重新赋值给文档对象
        doc.metadata = metadata
        bili.append(doc)
        
except Exception as e:
    print(f"加载BiliBili视频失败: {str(e)}")

# 如果没有数据加载成功，则退出程序
if not bili:
    print("没有成功加载任何视频，程序退出")
    exit()

# ==================== 2. 创建向量存储 ====================
# 初始化嵌入模型：使用 BGE 中文小模型
embed_model = HuggingFaceEmbeddings(model_name="BAAI/bge-small-zh-v1.5")
# 将文档存入 Chroma 向量数据库（内存模式）
vectorstore = Chroma.from_documents(bili, embed_model)

# ==================== 3. 配置元数据字段信息 ====================
# 定义元数据字段的描述信息，这有助于 LLM 理解哪些字段可以用来排序/过滤
metadata_field_info = [
    AttributeInfo(name="title", description="视频标题（字符串）", type="string"),
    AttributeInfo(name="author", description="视频作者（字符串）", type="string"),
    AttributeInfo(name="view_count", description="视频观看次数（整数）", type="integer"),
    AttributeInfo(name="length", description="视频长度（整数）", type="integer")
]

# ==================== 4. 初始化 LLM 客户端 ====================
# 使用 OpenAI SDK 连接 DeepSeek API 终端
client = OpenAI(
    base_url="https://api.deepseek.com",
    api_key=os.getenv("DEEPSEEK_API_KEY")
)

# ==================== 5. 获取所有文档用于后续排序 ====================
# 执行一个空的相似度搜索，获取库中所有的文档对象
all_documents = vectorstore.similarity_search("", k=len(bili)) 

# ==================== 6. 执行查询示例与动态排序 ====================
# 定义自然语言查询，包含排序意图
queries = [
    "时间最短的视频",
    "播放量最高的视频"
]

for query in queries:
    print(f"\n--- 原始查询: '{query}' ---")

    # 构建提示词：要求 LLM 将模糊的查询转为具体的 JSON 排序指令
    prompt = f"""你是一个智能助手，请将用户的问题转换成一个用于排序视频的JSON指令。

你需要识别用户想要排序的字段和排序方向。
- 排序字段必须是 'view_count' (观看次数) 或 'length' (时长) 之一。
- 排序方向必须是 'asc' (升序) 或 'desc' (降序) 之一。

例如:
- '时间最短的视频' 应转换为 {{"sort_by": "length", "order": "asc"}}
- '播放量最高的视频' 应转换为 {{"sort_by": "view_count", "order": "desc"}}

请根据以下问题生成JSON指令:
原始问题: "{query}"

JSON指令:"""
    
    # 调用 LLM 生成指令
    response = client.chat.completions.create(
        model="deepseek-chat",
        messages=[{"role": "user", "content": prompt}],
        temperature=0, # 设置温度为 0，确保输出结果的确定性
        response_format={"type": "json_object"} # 强制要求模型返回 JSON 对象
    )
    
    try:
        import json
        # 解析模型返回的 JSON 字符串
        instruction_str = response.choices[0].message.content
        instruction = json.loads(instruction_str)
        print(f"--- 生成的排序指令: {instruction} ---")

        sort_by = instruction.get('sort_by')
        order = instruction.get('order')

        # 检查指令合法性并执行排序
        if sort_by in ['length', 'view_count'] and order in ['asc', 'desc']:
            # 计算是否需要逆序（降序为 True）
            reverse_order = (order == 'desc')
            # 使用 Python 的内置 sorted 函数根据提取的元数据字段对文档进行排序
            sorted_docs = sorted(all_documents, key=lambda doc: doc.metadata.get(sort_by, 0), reverse=reverse_order)
            
            # 展示排序后的第一名（即满足条件的视频）
            if sorted_docs:
                doc = sorted_docs[0]
                title = doc.metadata.get('title', '未知标题')
                author = doc.metadata.get('author', '未知作者')
                view_count = doc.metadata.get('view_count', '未知')
                length = doc.metadata.get('length', '未知')
                print(f"结果 -> 标题: {title}")
                print(f"        作者: {author}")
                print(f"        观看次数: {view_count}")
                print(f"        时长: {length}秒")
                print("="*50)
            else:
                print("没有找到任何视频")
        else:
            print("生成的指令无效，无法执行排序")

    except (json.JSONDecodeError, KeyError) as e:
        print(f"解析或执行指令失败: {e}")

