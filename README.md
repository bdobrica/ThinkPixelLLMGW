# ThinkPixelLLMGW

An enterprise-grade LLM Gateway for managing multi-provider LLM access with authentication, rate limiting, cost tracking, and comprehensive logging.

## Overview

ThinkPixelLLMGW is a production-ready gateway service that provides:
- **Unified API**: OpenAI-compatible API for multiple LLM providers
- **Multi-Provider Support**: OpenAI, Google VertexAI, AWS Bedrock (extensible)
- **Cost Management**: Per-key budgets and real-time cost tracking
- **Rate Limiting**: Configurable per-key request limits
- **Audit Logging**: Comprehensive request/response logging to S3
- **Metrics & Monitoring**: Prometheus-compatible metrics for latency, costs, and usage
- **Model Aliasing**: Create custom model names with provider-specific routing
- **Scalability**: Kubernetes-ready with Redis-backed distributed state

## 📚 Documentation

- **[Quick Start Guide](QUICKSTART.md)** - Get up and running in 5 minutes
- **[Architecture Documentation](ARCHITECTURE.md)** - Detailed system design and data flows
- **[Development Plan](DEVELOPMENT_PLAN.md)** - Roadmap and implementation strategy
- **[TODO List](TODO.md)** - Comprehensive task tracking for all features

## Architecture

```
┌─────────────┐
│  Clients    │
└──────┬──────┘
       │ Bearer <API-Key>
       ▼
┌─────────────────────────────────────────┐
│         LLM Gateway (Go)                │
│  ┌────────────────────────────────┐    │
│  │  Proxy Handler                 │    │
│  │  1. Auth (API Key hash lookup) │    │
│  │  2. Rate Limiting (Redis)      │    │
│  │  3. Budget Check (Redis)       │    │
│  │  4. Model Resolution (DB)      │    │
│  │  5. Provider Call              │    │
│  │  6. Logging (Redis → S3)       │    │
│  │  7. Billing Update (Redis)     │    │
│  └────────────────────────────────┘    │
│                                         │
│  ┌────────────────────────────────┐    │
│  │  Admin API (JWT Protected)     │    │
│  │  - API Key Management          │    │
│  │  - Provider Management         │    │
│  │  - Model Alias Management      │    │
│  └────────────────────────────────┘    │
└────────┬───────────┬──────────┬────────┘
         │           │          │
    ┌────▼────┐ ┌───▼────┐ ┌──▼──────┐
    │ PostgreSQL│ │ Redis  │ │ S3/Minio│
    │ (Config) │ │(Runtime)│ │ (Logs)  │
    └──────────┘ └────────┘ └─────────┘
```

### Components

#### 1. **Proxy Service** (`/v1/chat/completions`)
- OpenAI-compatible API endpoint
- API key authentication via Bearer token
- Model-to-provider resolution
- Request forwarding with provider-specific transformations
- Response streaming support (future)

#### 2. **Admin API** (`/admin/*`)
- JWT-based authentication
- CRUD operations for:
  - API Keys (create, revoke, regenerate, tag)
  - Providers (add, configure, credentials)
  - Model Aliases (create mappings)

#### 3. **Storage Layer**
- **PostgreSQL**: API keys, providers, aliases, budgets, historical costs
- **Redis**: Rate limiting, real-time cost tracking, log buffering
- **S3**: Long-term request/response audit logs

#### 4. **Provider Plugins**
- OpenAI (GPT-4, GPT-3.5, etc.)
- Google VertexAI (Gemini, PaLM)
- AWS Bedrock (Claude, Llama, etc.)
- Extensible provider interface

## Features

### Current Status

#### ✅ Implemented
- [x] HTTP server with routing
- [x] Request flow architecture
- [x] Data models (API keys, providers, aliases)
- [x] Interface-based design for all services
- [x] JWT middleware for admin protection
- [x] Basic proxy handler structure
- [x] Placeholder implementations for all components

#### 🔨 In Progress / Planned

**Phase 1: Core Functionality** (Essential for MVP)
- [ ] PostgreSQL integration with migrations
- [ ] Redis integration (rate limiting, billing, log buffer)
- [ ] Provider implementations (OpenAI, VertexAI, Bedrock)
- [ ] Admin API endpoints (key/provider/alias CRUD)
- [ ] Logging pipeline (Redis buffer → S3 writer)
- [ ] Configuration management (env vars, config files)

**Phase 2: Production Features**
- [ ] Prometheus metrics integration
- [ ] Provider credential encryption (AES-256-GCM)
- [ ] Cost calculation per provider/model
- [ ] Budget enforcement logic
- [ ] API key regeneration
- [ ] Comprehensive error handling

**Phase 3: UI & Polish**
- [ ] FastAPI Python admin UI
- [ ] Response streaming support
- [ ] Enhanced logging (structured, filterable)
- [ ] Webhook support for budget alerts
- [ ] Multi-region support

### API Key Features
- **Authentication**: SHA-256 hashed keys in database
- **Permissions**: Model allowlist per key
- **Rate Limiting**: Requests per minute (Redis-backed)
- **Budgets**: Monthly USD limits with real-time tracking
- **Tags**: AWS-style tags for organization and metrics
- **Lifecycle**: Create, revoke, regenerate operations

### Provider Management
- **Multiple Credentials**: Multiple provider instances for cost tracking
- **Secure Storage**: Encrypted credentials in DB or file-mounted
- **Model Aliasing**: Map custom names to provider models
- **Cost Tracking**: Per-provider usage and costs

### Logging & Observability
- **Audit Logs**: Every request logged to S3 with:
  - Request/response payloads
  - Latency metrics (gateway + provider)
  - Cost per request
  - API key metadata and tags
- **Metrics**: Prometheus-compatible metrics:
  - Request rate by provider/model/key
  - Latency percentiles (p50, p95, p99)
  - Cost metrics
  - Error rates
- **Distributed Tracing**: Request ID tracking across components

## Getting Started

### Prerequisites
- Go 1.23+
- PostgreSQL 14+
- Redis 7+
- S3-compatible storage (AWS S3, MinIO, etc.)
- (Optional) Python 3.11+ for admin UI

### Configuration

Environment variables:
```bash
GATEWAY_HTTP_PORT=8080
DATABASE_URL=postgres://user:pass@localhost/llmgateway
REDIS_URL=redis://localhost:6379
S3_ENDPOINT=https://s3.amazonaws.com
S3_BUCKET=llm-logs
S3_ACCESS_KEY=...
S3_SECRET_KEY=...
JWT_SECRET=...
ENCRYPTION_KEY=...  # 32-byte hex for AES-256
```

### Running Locally

```bash
cd llm-gateway
go mod download
go run cmd/gateway/main.go
```

### Example Usage

**Proxy Request:**
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer demo-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Admin API:**
```bash
# Create API key
curl -X POST http://localhost:8080/admin/keys \
  -H "Authorization: Bearer <jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production Key",
    "allowed_models": ["gpt-4", "gpt-3.5-turbo"],
    "rate_limit_per_minute": 60,
    "monthly_budget_usd": 1000.0,
    "tags": {"env": "production", "team": "backend"}
  }'
```

## Development Roadmap

See [TODO.md](TODO.md) for detailed task tracking.

### Milestone 1: MVP (Weeks 1-2)
- Database schema and migrations
- Redis integration
- OpenAI provider implementation
- Basic admin API (key creation/lookup)
- Simple logging to S3

### Milestone 2: Multi-Provider (Weeks 3-4)
- VertexAI and Bedrock providers
- Model aliasing system
- Cost calculation engine
- Full admin API (providers, aliases)

### Milestone 3: Production Ready (Weeks 5-6)
- Prometheus metrics
- Credential encryption
- Budget enforcement
- Rate limiting
- Error handling and retries

### Milestone 4: UI & Polish (Weeks 7-8)
- FastAPI admin UI
- Enhanced monitoring
- Documentation
- Deployment guides (Kubernetes, Docker)

## Project Structure

```
ThinkPixelLLMGW/
├── README.md                   # This file - project overview
├── QUICKSTART.md              # 5-minute setup guide
├── ARCHITECTURE.md            # Detailed system architecture
├── DEVELOPMENT_PLAN.md        # 8-week implementation roadmap
├── TODO.md                    # Comprehensive task tracking
├── PROJECT_SUMMARY.md         # Current state analysis
├── LICENSE                    # License file
└── llm-gateway/               # Main Go application
    ├── cmd/
    │   └── gateway/
    │       └── main.go        # Application entry point
    ├── internal/
    │   ├── auth/              # API key & JWT authentication
    │   │   ├── api_key.go    # API key lookup
    │   │   ├── jwt.go        # JWT handling
    │   │   ├── hash.go       # Hashing utilities
    │   │   └── errors.go     # Auth errors
    │   ├── billing/           # Cost tracking & budget enforcement
    │   │   └── billing.go
    │   ├── config/            # Configuration management
    │   │   └── config.go
    │   ├── httpapi/           # HTTP handlers & routing
    │   │   ├── router.go     # Route setup
    │   │   ├── proxy_handler.go    # Proxy endpoint
    │   │   ├── admin_handler.go    # Admin API
    │   │   └── jwt_middleware.go   # JWT middleware
    │   ├── logging/           # Redis buffer & S3 writer
    │   │   ├── sink.go
    │   │   ├── redis_buffer.go
    │   │   └── s3_writer.go
    │   ├── metrics/           # Prometheus metrics
    │   │   └── metrics.go
    │   ├── models/            # Data models
    │   │   ├── api_key.go
    │   │   ├── provider.go
    │   │   ├── alias.go
    │   │   └── errors.go
    │   ├── providers/         # LLM provider implementations
    │   │   ├── provider.go   # Interface
    │   │   ├── registry.go   # Provider registry
    │   │   ├── openai.go     # OpenAI implementation
    │   │   ├── vertexai.go   # VertexAI implementation
    │   │   └── bedrock.go    # Bedrock implementation
    │   ├── ratelimit/         # Redis-based rate limiting
    │   │   └── ratelimiter.go
    │   └── storage/           # Database & encryption
    │       ├── db.go
    │       ├── encryption.go
    │       └── migrations/    # Database migrations
    └── go.mod
```

## Contributing

This is a personal project but suggestions and feedback are welcome!

## License

See [LICENSE](LICENSE) file.
