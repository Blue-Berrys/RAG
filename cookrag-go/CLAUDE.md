# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CookRAG-Go is an enterprise-level Retrieval-Augmented Generation (RAG) system implemented in pure Go, designed for interview/portfolio demonstration. It integrates multiple retrieval strategies (vector search, BM25, graph RAG, hybrid) with intelligent query routing and LLM generation.

## Architecture

### Core RAG Pipeline
```
HTTP API → Query Router → Multi-Strategy Retrieval → Context Building → LLM Generation → Answer
```

### Key Components

**Query Router** (`internal/core/router/`):
- Analyzes query complexity and relationship intensity
- Automatically selects optimal retrieval strategy (BM25/vector/graph/hybrid)
- Uses LLM for query analysis

**Retrieval Strategies** (`internal/core/retrieval/`):
- **BM25** (`bm25.go`): Full-text search with inverted index and Chinese tokenization
- **Vector** (`vector.go`): Milvus-based semantic search with embedding
- **Graph** (`graph.go`): Neo4j multi-hop traversal for knowledge graph queries
- **Hybrid** (`hybrid.go`): RRF (Reciprocal Rank Fusion) for combining multiple strategies

**ML Modules** (`pkg/ml/`):
- **Embedding** (`embedding/`): Supports 4 Chinese APIs (Zhipu AI, Baidu Qianfan, Alibaba DashScope, Volcengine). Zhipu AI is recommended and free for LLM, but embedding requires paid tokens.
- **LLM** (`llm/`): Zhipu AI's GLM-4-flash model for answer generation (completely free)

**Storage Layer** (`pkg/storage/`):
- **Milvus** (`milvus/`): Vector database client with indexing and search
- **Neo4j** (`neo4j/`): Graph database client for multi-hop queries
- **Redis** (`cache/`): Caching layer with in-memory fallback

### Configuration

Uses YAML (`config/config.yaml`) with environment variable substitution:
- Supports `${VAR_NAME}` syntax for sensitive data
- Environment variables loaded from `.env` file
- Key configs: embedding provider, database connections, LLM model

## Development Commands

### Primary Commands

```bash
# Run the complete RAG demonstration (recommended)
bash run.sh

# Or manually:
source .env && go run cmd/demo/main.go

# Check configuration before running
./check-config.sh

# Build the project
make build

# Run tests
make test

# Format code
make fmt

# Lint code
make lint
```

### Docker Services

```bash
# Start all dependencies (Milvus, Neo4j, Redis)
cd deployments/docker && docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## Important Implementation Details

### Environment Variable Handling

The config system in `internal/config/config.go` supports both `${VAR}` and `$VAR` formats for environment variable substitution. This is critical for API keys and database credentials.

### Milvus SDK API Quirks

The Milvus Go SDK has specific API requirements:
- Use `entity.NewColumn*` functions for column creation (e.g., `NewColumnInt64`, `NewColumnFloatVector`)
- For search: convert vectors to `[]entity.Vector` type using `entity.FloatVector()`
- JSON columns use `entity.NewColumnJSONBytes` (not `NewColumnJSON`)
- Search requires `IndexSearchParam` (e.g., `entity.NewIndexIvfFlatSearchParam(10)`)

### Neo4j Driver API

- `neo4j.BasicAuth()` requires 3 parameters: username, password, and realm (use empty string "")
- Session config must include `DatabaseName` field when creating sessions
- All `Close()` methods require context parameter

### BM25 Tokenization

The current BM25 implementation uses simple Chinese tokenization (by punctuation and spaces) which may result in poor matching. This is a known limitation - for production use, integrate jieba-go for proper Chinese word segmentation.

### LLM Integration

- Zhipu AI's `glm-4-flash` is completely free for chat API
- Embedding API requires separate paid tokens
- The system gracefully degrades: if embedding fails, BM25 and LLM still work
- Even with 0 retrieval results, LLM generates answers based on general knowledge

### Error Handling Pattern

```go
// Standard pattern used throughout
if err != nil {
    log.Warnf("⚠️  Failed to connect to X: %v", err)
    component = nil  // Graceful degradation
} else {
    log.Info("✅ Connected to X")
}
```

## Testing

### Current Test Data

The demo includes 3 sample documents about Chinese cuisine (红烧肉, 宫保鸡丁, 麻婆豆腐) for testing retrieval and generation.

### Known Limitations

1. **BM25 Retrieval**: Returns 0 results due to simple tokenization removing too many words
2. **Vector Search**: Requires paid Zhipu embedding API tokens (1113 error)
3. **LLM Generation**: Works perfectly (glm-4-flash is free)

Despite these limitations, the system demonstrates the complete RAG flow with LLM generating answers based on general knowledge when retrieval fails.

## Deployment

### Production Considerations

- The system is designed for demonstration but is production-ready with proper API keys
- All connections support graceful degradation
- Includes monitoring (Prometheus metrics) and tracing (simplified span tracking)
- HTTP API runs on port 8080

### Configuration for Different Environments

Edit `config/config.yaml` and `.env` to switch between:
- Embedding providers (zhipu, qianfan, dashscope, volcengine)
- Database connections
- LLM models

## Quick Reference

### File Locations

- Main demo: `cmd/demo/main.go`
- Config loader: `internal/config/config.go`
- Query router: `internal/core/router/router.go`
- Retrieval implementations: `internal/core/retrieval/*.go`
- HTTP server: `internal/api/server/server.go`
- Storage clients: `pkg/storage/*/*.go`

### Critical Functions

- **Query Routing**: `router.Route()` - Analyzes query and selects strategy
- **Context Building**: `buildContext()` in demo - Formats retrieved docs for LLM
- **Prompt Building**: `buildPrompt()` in demo - Constructs LLM prompt with context
- **Config Loading**: `config.Load()` - Handles env var substitution

### Common Issues

1. **Neo4j auth error**: Password from docker-compose.yml is `cookrag_password`
2. **Embedding 401/1113 error**: Zhipu embedding API requires paid tokens (use LLM for free)
3. **BM25 0 results**: Known limitation with simple tokenizer - LLM will still generate answers
