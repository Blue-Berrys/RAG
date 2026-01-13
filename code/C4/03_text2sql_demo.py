# 导入操作系统相关功能的模块
import os
# 导入系统功能模块，用于操作 Python 运行时环境
import sys
# 导入 SQLite3 数据库模块
import sqlite3

# 将当前目录下的 'text2sql' 文件夹添加到 Python 的模块搜索路径中
# 这样可以直接从 text2sql 文件夹中导入自定义模块
sys.path.append(os.path.join(os.path.dirname(__file__), 'text2sql'))

# 从自定义模块中导入 SimpleText2SQLAgent 类
from text2sql.text2sql_agent import SimpleText2SQLAgent


def setup_demo():
    """设置演示环境：初始化代理、数据库和知识库"""
    print("=== Text2SQL框架演示 ===\n")
    
    # 检查环境变量中是否设置了 DeepSeek 的 API 密钥
    api_key = "sk-5de384802e84440c8d332ab1f9bbd860"
    if not api_key:
        print("先设置DEEPSEEK_API_KEY环境变量")
        return None
    
    # 创建用于演示的临时 SQLite 数据库
    print("创建演示数据库...")
    db_path = create_demo_database()
    
    # 初始化 Text2SQL 代理对象
    print("初始化Text2SQL代理...")
    agent = SimpleText2SQLAgent(api_key=api_key)
    
    # 连接到刚刚创建的 SQLite 数据库
    print("连接数据库...")
    if not agent.connect_database(db_path):
        print("数据库连接失败!")
        return None
    
    # 加载知识库：包含数据库 Schema（模式）定义和相关知识
    # 这有助于 LLM 更好地理解数据库结构和业务逻辑
    print("加载知识库...")
    try:
        agent.load_knowledge_base()
        print("知识库加载成功!")
    except Exception as e:
        print(f"知识库加载失败: {str(e)}")
        return None
    
    return agent, db_path


def create_demo_database():
    """创建一个包含用户、产品和订单表的演示数据库"""
    db_path = "text2sql_demo.db"
    
    # 如果数据库文件已存在，则先删除它以便重新创建
    if os.path.exists(db_path):
        os.remove(db_path)
    
    # 连接并创建数据库文件
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    # 创建用户表 (users)
    cursor.execute("""
        CREATE TABLE users (
            id INTEGER PRIMARY KEY,   -- 用户唯一标识
            name TEXT NOT NULL,       -- 姓名
            email TEXT UNIQUE,        -- 邮箱
            age INTEGER,              -- 年龄
            city TEXT                 -- 城市
        )
    """)
    
    # 创建产品表 (products)
    cursor.execute("""
        CREATE TABLE products (
            id INTEGER PRIMARY KEY,   -- 产品唯一标识
            name TEXT NOT NULL,       -- 产品名称
            category TEXT,            -- 类别（如：电子产品、服装）
            price REAL,               -- 价格
            stock INTEGER             -- 库存数量
        )
    """)
    
    # 创建订单表 (orders)
    cursor.execute("""
        CREATE TABLE orders (
            id INTEGER PRIMARY KEY,   -- 订单唯一标识
            user_id INTEGER,          -- 关联用户 ID
            product_id INTEGER,       -- 关联产品 ID
            quantity INTEGER,         -- 购买数量
            order_date TEXT,          -- 订单日期
            total_price REAL,         -- 订单总额
            FOREIGN KEY (user_id) REFERENCES users(id),    -- 外键关联
            FOREIGN KEY (product_id) REFERENCES products(id) -- 外键关联
        )
    """)
    
    # 准备示例数据：用户信息
    users_data = [
        (1, '张三', 'zhangsan@email.com', 25, '北京'),
        (2, '李四', 'lisi@email.com', 32, '上海'),
        (3, '王五', 'wangwu@email.com', 28, '广州'),
        (4, '赵六', 'zhaoliu@email.com', 35, '深圳'),
        (5, '陈七', 'chenqi@email.com', 29, '杭州'),
    ]
    
    # 准备示例数据：产品信息
    products_data = [
        (1, 'iPhone 15', '电子产品', 7999.0, 50),
        (2, 'MacBook Pro', '电子产品', 12999.0, 20),
        (3, 'Nike运动鞋', '服装', 599.0, 100),
        (4, '办公椅', '家具', 899.0, 30),
        (5, '台灯', '家具', 199.0, 80),
        (6, 'iPad', '电子产品', 3999.0, 40),
        (7, 'Adidas外套', '服装', 399.0, 60),
    ]
    
    # 准备示例数据：订单信息
    orders_data = [
        (1, 1, 1, 1, '2024-01-15', 7999.0),
        (2, 2, 3, 2, '2024-01-16', 1198.0),
        (3, 3, 5, 1, '2024-01-17', 199.0),
        (4, 1, 2, 1, '2024-01-18', 12999.0),
        (5, 4, 4, 1, '2024-01-19', 899.0),
        (6, 5, 6, 1, '2024-01-20', 3999.0),
        (7, 2, 7, 1, '2024-01-21', 399.0),
    ]
    
    # 批量插入数据到各个表
    cursor.executemany("INSERT INTO users VALUES (?, ?, ?, ?, ?)", users_data)
    cursor.executemany("INSERT INTO products VALUES (?, ?, ?, ?, ?)", products_data)
    cursor.executemany("INSERT INTO orders VALUES (?, ?, ?, ?, ?, ?)", orders_data)
    
    # 提交事务并关闭连接
    conn.commit()
    conn.close()
    
    print(f"演示数据库已创建: {db_path}")
    return db_path


def run_demo_queries(agent):
    """通过自然语言向代理提问并展示结果"""
    # 定义一组用于测试的自然语言问题
    demo_questions = [
        "查询所有用户的姓名和邮箱",
        "年龄大于30的用户有哪些",
        "哪些产品的库存少于50",
        "查询来自北京的用户的所有订单",
        "统计每个城市的用户数量",
        "查询价格在500-8000之间的产品"
    ]
    
    print("\n开始运行演示查询...\n")
    
    success_count = 0
    
    # 遍历每个问题并执行查询
    for i, question in enumerate(demo_questions, 1):
        print(f"问题 {i}: {question}")
        print("-" * 60)
        
        try:
            # 核心步骤：调用代理的 query 方法，该方法会将文本转为 SQL 并执行
            result = agent.query(question)
            
            # 如果查询成功执行
            if result["success"]:
                print(f"成功! SQL: {result['sql']}")
                
                # 处理返回的查询结果数据
                if isinstance(result["results"], dict) and "rows" in result["results"]:
                    count = result["results"]["count"]
                    print(f"返回 {count} 行数据")
                    
                    # 仅显示前 2 行数据，以免输出过长
                    if count > 0:
                        for j, row in enumerate(result["results"]["rows"][:2]):
                            # 将字典格式的一行数据转为字符串显示
                            row_str = " | ".join(f"{k}: {v}" for k, v in row.items())
                            print(f"  {j+1}. {row_str}")
                        
                        if count > 2:
                            print(f"  ... 还有 {count - 2} 行")
                else:
                    # 如果结果不是预期的列表格式，直接打印
                    print(f"结果: {result['results']}")
                
                success_count += 1
                
            else:
                # 如果 SQL 执行或生成失败，打印错误信息和失败的 SQL
                print(f"失败: {result['error']}")
                print(f"SQL: {result['sql']}")
                
        except Exception as e:
            # 捕获运行时的意外异常
            print(f"执行错误: {str(e)}")
        
        print() # 换行以便区分不同问题
    
    # 输出成功统计信息
    total_count = len(demo_questions)
    print(f"完成! 成功率: {success_count}/{total_count}")


def cleanup(agent, db_path):
    """释放资源并删除临时数据库文件"""
    print("\n清理资源...")
    
    # 调用代理的清理方法（如关闭数据库连接）
    if agent:
        agent.cleanup()
    
    # 删除演示用的 SQLite 数据库文件
    if os.path.exists(db_path):
        os.remove(db_path)
        print(f"已删除演示数据库: {db_path}")


def main():
    """演示程序入口函数"""
    # 1. 设置演示环境
    setup_result = setup_demo()
    
    if setup_result is None:
        return
    
    # 获取初始化好的代理和数据库路径
    agent, db_path = setup_result
    
    try:
        # 2. 运行演示查询
        run_demo_queries(agent)
        
    finally:
        # 3. 无论成功还是异常，最后都要执行清理工作
        cleanup(agent, db_path)


if __name__ == "__main__":
    # 执行主函数
    main() 
 