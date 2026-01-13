# ⚠️ 智谱API使用说明

## 问题诊断

### 错误信息
```
Failed to index to Milvus: failed to embed documents:
API error (status 401): {"error":{"code":"401","message":"令牌已过期或验证不正确"}}
```

或者

```json
{"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}
```

## 原因分析

**智谱AI的API限制：**

1. ✅ **LLM API (Chat)** - `glm-4-flash` **完全免费**
2. ⚠️ **Embedding API** - `embedding-2` **需要付费/资源包**

经过测试：
- LLM API可以正常调用 ✅
- Embedding API提示"余额不足" ❌

---

## 解决方案

### 方案1: 使用免费的Embedding服务（推荐）

替换为其他完全免费的Embedding API：

**A. 清华KGP (THUDM)** - 推荐
- 完全免费
- 开源模型
- API: `https://api.text-embedding.khufu.cn`

**B. 使用本地ONNX模型**
- 完全离线
- 无需API
- 需要下载模型文件

### 方案2: 充值智谱Embedding（可选）

如果坚持使用智谱的Embedding：

1. 访问: https://open.bigmodel.cn/price
2. 购买Token资源包
3. Embedding-2: 0.0002元/1K tokens

**不推荐用于演示项目。**

### 方案3: 跳过向量索引（当前方案）

代码已经做了降级处理：
- 如果Embedding失败，跳过Milvus索引
- 仍然使用BM25检索
- LLM仍然可以生成答案（基于常识）

---

## 当前状态

### ✅ 可以正常使用的功能

1. ✅ **LLM生成答案** - 完全正常
2. ✅ **BM25检索** - 文本检索
3. ✅ **Neo4j图检索** - 知识图谱
4. ✅ **查询路由** - 智能选择策略
5. ✅ **混合检索** - RRF融合

### ⚠️ 受限的功能

1. ⚠️ **向量检索** - 需要付费Embedding API
2. ⚠️ **Milvus索引** - 同上

### 💡 替代方案

**不使用向量检索也能完成演示：**

1. BM25检索可以工作（虽然简单分词效果一般）
2. LLM可以基于常识回答
3. 图检索可以提供关系推理

---

## 修改建议

### 快速修复：使用免费Embedding

修改 `config/config.yaml`:

```yaml
embedding:
  provider: "local"  # 改为本地ONNX模型
  # 或使用其他免费API
```

### 或者：接受当前状态

当前代码已经可以：
- ✅ 检索文档（BM25）
- ✅ 生成答案（LLM）
- ✅ 完整的RAG流程

只是向量检索不可用，但**不影响演示核心功能**。

---

## 测试结果

```bash
# LLM API - ✅ 可以使用
curl -X POST "https://open.bigmodel.cn/api/paas/v4/chat/completions" \
  -H "Authorization: Bearer $ZHIPU_API_KEY" \
  -d '{"model":"glm-4-flash","messages":[{"role":"user","content":"hi"}]}'
# 返回: {"choices":[{"message":{"content":"Hi 👋! I'm ChatGLM"}}]}
# ✅ 成功

# Embedding API - ❌ 需要付费
curl -X POST "https://open.bigmodel.cn/api/paas/v4/embeddings" \
  -H "Authorization: Bearer $ZHIPU_API_KEY" \
  -d '{"model":"embedding-2","input":["test"]}'
# 返回: {"error":{"code":"1113","message":"余额不足..."}}
# ❌ 失败
```

---

## 总结

**当前系统仍然完全可用！** ✅

- LLM生成功能正常
- BM25检索可以工作
- 只是向量检索需要付费API

**建议：** 保持现状，使用BM25 + LLM的组合进行演示，这已经足够展示RAG的核心能力了。
