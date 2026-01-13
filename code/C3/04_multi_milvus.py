# 导入操作系统相关功能的模块
import os
# 导入进度条显示库，用于显示处理进度
from tqdm import tqdm
# 导入文件路径匹配模块，用于查找符合特定模式的文件
from glob import glob
# 导入 PyTorch 深度学习框架
import torch
# 导入视觉化 BGE 模型，用于多模态嵌入
from visual_bge.visual_bge.modeling import Visualized_BGE
# 导入 Milvus 客户端及相关数据结构，用于向量数据库操作
from pymilvus import MilvusClient, FieldSchema, CollectionSchema, DataType
# 导入 NumPy 数组处理库
import numpy as np
# 导入 OpenCV 图像处理库
import cv2
# 导入 PIL 图像处理库
from PIL import Image

# ==================== 1. 初始化设置 ====================
# 预训练模型名称，使用 BGE 基础英文模型
MODEL_NAME = "BAAI/bge-base-en-v1.5"
# 视觉化 BGE 模型权重文件路径
MODEL_PATH = "../../models/bge/Visualized_base_en_v1.5.pth"
# 数据目录路径
DATA_DIR = "../../data/C3"
# Milvus 集合名称，用于存储多模态向量
COLLECTION_NAME = "multimodal_demo"
# Milvus 服务器地址
MILVUS_URI = "http://localhost:19530"

# ==================== 2. 定义工具（编码器和可视化函数）====================
class Encoder:
    """编码器类，用于将图像和文本编码为向量。"""
    
    def __init__(self, model_name: str, model_path: str):
        """
        初始化编码器
        Args:
            model_name: BGE 模型名称
            model_path: 视觉化模型权重文件路径
        """
        # 加载视觉化 BGE 模型，该模型支持图像和文本的多模态编码
        self.model = Visualized_BGE(model_name_bge=model_name, model_weight=model_path)
        # 将模型设置为评估模式，禁用 dropout 等训练特性
        self.model.eval()

    def encode_query(self, image_path: str, text: str) -> list[float]:
        """
        将图像和文本组合编码为向量（多模态查询）
        Args:
            image_path: 查询图像的文件路径
            text: 查询文本内容
        Returns:
            编码后的向量（list 格式）
        """
        # 使用 torch.no_grad() 禁用梯度计算，节省内存并加速推理
        with torch.no_grad():
            # 将图像和文本一起编码为一个统一的向量表示
            query_emb = self.model.encode(image=image_path, text=text)
        # 将 tensor 转换为 Python list，并取第一个元素（因为返回的是批次格式）
        return query_emb.tolist()[0]

    def encode_image(self, image_path: str) -> list[float]:
        """
        仅将图像编码为向量（纯图像检索）
        Args:
            image_path: 图像文件路径
        Returns:
            编码后的向量（list 格式）
        """
        # 禁用梯度计算
        with torch.no_grad():
            # 仅对图像进行编码
            query_emb = self.model.encode(image=image_path)
        # 转换为 list 格式并返回
        return query_emb.tolist()[0]

def visualize_results(query_image_path: str, retrieved_images: list, img_height: int = 300, img_width: int = 300, row_count: int = 3) -> np.ndarray:
    """
    从检索到的图像列表创建一个全景图用于可视化
    Args:
        query_image_path: 查询图像的路径
        retrieved_images: 检索结果图像路径列表
        img_height: 每张图像的显示高度
        img_width: 每张图像的显示宽度
        row_count: 每行显示的图像数量
    Returns:
        合成的全景图像（NumPy 数组）
    """
    # 计算全景图的总宽度（每行放置 row_count 张图片）
    panoramic_width = img_width * row_count
    # 计算全景图的总高度（根据图片数量自动计算行数）
    panoramic_height = img_height * row_count
    # 创建白色背景的全景图画布，shape 为 (高, 宽, 3通道RGB)，填充白色(255)
    panoramic_image = np.full((panoramic_height, panoramic_width, 3), 255, dtype=np.uint8)
    # 创建查询图像显示区域（左侧单独显示查询图像的区域）
    query_display_area = np.full((panoramic_height, img_width, 3), 255, dtype=np.uint8)

    # ========== 处理查询图像 ==========
    # 使用 PIL 打开图像并转换为 RGB 格式
    query_pil = Image.open(query_image_path).convert("RGB")
    # 将 PIL 图像转换为 OpenCV 格式（NumPy 数组），并将 RGB 转为 BGR（::-1 反转颜色通道）
    query_cv = np.array(query_pil)[:, :, ::-1]
    # 调整查询图像大小到指定尺寸
    resized_query = cv2.resize(query_cv, (img_width, img_height))
    # 为查询图像添加蓝色边框（上下左右各10像素，颜色为蓝色 BGR=(255,0,0)）
    bordered_query = cv2.copyMakeBorder(resized_query, 10, 10, 10, 10, cv2.BORDER_CONSTANT, value=(255, 0, 0))
    # 将带边框的查询图像放置在显示区域的底部
    query_display_area[img_height * (row_count - 1):, :] = cv2.resize(bordered_query, (img_width, img_height))
    # 在查询图像上添加 "Query" 文字标签（蓝色）
    cv2.putText(query_display_area, "Query", (10, panoramic_height - 20), cv2.FONT_HERSHEY_SIMPLEX, 1, (255, 0, 0), 2)

    # ========== 处理检索到的图像 ==========
    # 遍历所有检索结果图像
    for i, img_path in enumerate(retrieved_images):
        # 计算当前图像应该放置的行号和列号
        row, col = i // row_count, i % row_count
        # 计算图像在全景图中的起始位置（像素坐标）
        start_row, start_col = row * img_height, col * img_width
        
        # 打开检索结果图像并转换为 RGB 格式
        retrieved_pil = Image.open(img_path).convert("RGB")
        # 转换为 OpenCV 格式（BGR）
        retrieved_cv = np.array(retrieved_pil)[:, :, ::-1]
        # 调整图像大小（略小于格子大小，为边框留出空间）
        resized_retrieved = cv2.resize(retrieved_cv, (img_width - 4, img_height - 4))
        # 添加黑色边框（上下左右各2像素）
        bordered_retrieved = cv2.copyMakeBorder(resized_retrieved, 2, 2, 2, 2, cv2.BORDER_CONSTANT, value=(0, 0, 0))
        # 将处理后的图像放置到全景图的对应位置
        panoramic_image[start_row:start_row + img_height, start_col:start_col + img_width] = bordered_retrieved
        
        # 在图像左上角添加索引号（红色文字）
        cv2.putText(panoramic_image, str(i), (start_col + 10, start_row + 30), cv2.FONT_HERSHEY_SIMPLEX, 1, (0, 0, 255), 2)

    # 将查询图像区域和检索结果全景图水平拼接，返回最终结果
    return np.hstack([query_display_area, panoramic_image])

# ==================== 3. 初始化客户端 ====================
print("--> 正在初始化编码器和Milvus客户端...")
# 创建编码器实例，用于将图像和文本转换为向量
encoder = Encoder(MODEL_NAME, MODEL_PATH)
# 创建 Milvus 客户端连接，用于与向量数据库进行交互
milvus_client = MilvusClient(uri=MILVUS_URI)

# ==================== 4. 创建 Milvus Collection ====================
print(f"\n--> 正在创建 Collection '{COLLECTION_NAME}'")
# 检查集合是否已存在
if milvus_client.has_collection(COLLECTION_NAME):
    # 如果存在则先删除，确保重新开始
    milvus_client.drop_collection(COLLECTION_NAME)
    print(f"已删除已存在的 Collection: '{COLLECTION_NAME}'")

# 使用 glob 模块查找所有龙图片（.png 格式）
image_list = glob(os.path.join(DATA_DIR, "dragon", "*.png"))
# 如果没有找到图片，抛出异常
if not image_list:
    raise FileNotFoundError(f"在 {DATA_DIR}/dragon/ 中未找到任何 .png 图像。")
# 对第一张图片进行编码，获取向量维度（用于定义 Collection schema）
dim = len(encoder.encode_image(image_list[0]))

# 定义 Collection 的字段结构
fields = [
    # 主键字段：自增整数 ID
    FieldSchema(name="id", dtype=DataType.INT64, is_primary=True, auto_id=True),
    # 向量字段：存储图像的向量表示，维度为 dim
    FieldSchema(name="vector", dtype=DataType.FLOAT_VECTOR, dim=dim),
    # 图像路径字段：存储图像文件的路径，最大长度 512 字符
    FieldSchema(name="image_path", dtype=DataType.VARCHAR, max_length=512),
]

# 创建集合的 Schema（定义数据结构）
schema = CollectionSchema(fields, description="多模态图文检索")
print("Schema 结构:")
print(schema)

# 根据 Schema 创建 Collection（类似于数据库中的表）
milvus_client.create_collection(collection_name=COLLECTION_NAME, schema=schema)
print(f"成功创建 Collection: '{COLLECTION_NAME}'")
# 打印 Collection 的详细信息
print("Collection 结构:")
print(milvus_client.describe_collection(collection_name=COLLECTION_NAME))

# ==================== 5. 准备并插入数据 ====================
print(f"\n--> 正在向 '{COLLECTION_NAME}' 插入数据")
# 初始化数据列表，用于批量插入
data_to_insert = []
# 遍历所有图片，使用 tqdm 显示进度条
for image_path in tqdm(image_list, desc="生成图像嵌入"):
    # 将图像编码为向量
    vector = encoder.encode_image(image_path)
    # 将向量和图像路径组成一条记录添加到列表中
    data_to_insert.append({"vector": vector, "image_path": image_path})

# 如果有数据需要插入
if data_to_insert:
    # 批量插入数据到 Milvus Collection
    result = milvus_client.insert(collection_name=COLLECTION_NAME, data=data_to_insert)
    # 打印插入成功的数据条数
    print(f"成功插入 {result['insert_count']} 条数据。")

# ==================== 6. 创建索引 ====================
print(f"\n--> 正在为 '{COLLECTION_NAME}' 创建索引")
# 准备索引参数对象
index_params = milvus_client.prepare_index_params()
# 为向量字段添加 HNSW 索引配置
index_params.add_index(
    field_name="vector",          # 要建立索引的字段名
    index_type="HNSW",             # 索引类型：HNSW（Hierarchical Navigable Small World）高性能近似最近邻搜索
    metric_type="COSINE",          # 距离度量方式：余弦相似度
    params={"M": 16,               # HNSW 参数 M：每个节点的最大连接数，越大召回率越高但内存占用更大
            "efConstruction": 256} # HNSW 参数 efConstruction：构建索引时的搜索范围，越大索引质量越高但构建时间越长
)
# 根据配置创建索引
milvus_client.create_index(collection_name=COLLECTION_NAME, index_params=index_params)
print("成功为向量字段创建 HNSW 索引。")
# 打印索引的详细信息
print("索引详情:")
print(milvus_client.describe_index(collection_name=COLLECTION_NAME, index_name="vector"))
# 将 Collection 加载到内存中，只有加载后才能进行搜索
milvus_client.load_collection(collection_name=COLLECTION_NAME)
print("已加载 Collection 到内存中。")

# ==================== 7. 执行多模态检索 ====================
print(f"\n--> 正在 '{COLLECTION_NAME}' 中执行检索")
# 指定查询图像路径（要检索的图片）
query_image_path = os.path.join(DATA_DIR, "dragon", "query.png")
# 指定查询文本（与图像结合进行多模态检索）
query_text = "一条龙"
# 将查询图像和文本一起编码为向量（多模态查询向量）
query_vector = encoder.encode_query(image_path=query_image_path, text=query_text)

# 在 Milvus 中执行向量检索
search_results = milvus_client.search(
    collection_name=COLLECTION_NAME,   # 要搜索的集合名称
    data=[query_vector],                # 查询向量（可以是多个，这里只有一个）
    output_fields=["image_path"],       # 需要返回的字段（除了距离和 ID 外）
    limit=5,                            # 返回最相似的前 5 个结果
    search_params={                     # 搜索参数
        "metric_type": "COSINE",        # 使用余弦相似度进行距离计算
        "params": {"ef": 128}           # HNSW 搜索参数 ef：搜索时的候选集大小，越大召回率越高但速度越慢
    }
)[0]  # 取第一个查询的结果（因为只传入了一个查询向量）

# 存储检索到的图像路径
retrieved_images = []
print("检索结果:")
# 遍历检索结果
for i, hit in enumerate(search_results):
    # 打印每个结果的详细信息：排名、ID、相似度距离、图像路径
    print(f"  Top {i+1}: ID={hit['id']}, 距离={hit['distance']:.4f}, 路径='{hit['entity']['image_path']}'")
    # 将图像路径添加到列表中，用于后续可视化
    retrieved_images.append(hit['entity']['image_path'])

# ==================== 8. 可视化与清理 ====================
print(f"\n--> 正在可视化结果并清理资源")
# 检查是否有检索结果
if not retrieved_images:
    print("没有检索到任何图像。")
else:
    # 调用可视化函数，生成全景对比图（左边是查询图像，右边是检索结果）
    panoramic_image = visualize_results(query_image_path, retrieved_images)
    # 定义保存结果图像的路径
    combined_image_path = os.path.join(DATA_DIR, "search_result.png")
    # 使用 OpenCV 保存全景图像到文件
    cv2.imwrite(combined_image_path, panoramic_image)
    print(f"结果图像已保存到: {combined_image_path}")
    # 使用默认图像查看器打开结果图像
    Image.open(combined_image_path).show()

# 从内存中释放 Collection（释放内存资源但不删除数据）
milvus_client.release_collection(collection_name=COLLECTION_NAME)
print(f"已从内存中释放 Collection: '{COLLECTION_NAME}'")
# 删除 Collection（完全删除数据和索引）
milvus_client.drop_collection(COLLECTION_NAME)
print(f"已删除 Collection: '{COLLECTION_NAME}'")
