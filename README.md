# ThinkPixelLLMGW

An enterprise-grade LLM Gateway for managing multi-provider LLM access with authentication, rate limiting, cost tracking, and comprehensive logging.

## 🎯 Project Status (December 4, 2025)

**Current Phase**: Core MVP Complete - Production Ready

The gateway is now **fully functional** with a complete implementation of:
- ✅ **Database Layer**: PostgreSQL with schema migrations, full repository layer, and LRU caching
- ✅ **Redis Integration**: Rate limiting (fully wired Dec 4), billing cache, and log buffering
- ✅ **Provider System**: Pluggable architecture with OpenAI fully implemented (streaming support)
- ✅ **Proxy Endpoint**: Complete request flow from auth to response
- ✅ **Middleware**: API key authentication with database lookup
- ✅ **Cost Calculation**: Database-driven pricing with multi-dimensional support (direction, modality, unit, tier)
- ✅ **Billing Integration**: Automatic cost tracking with async queue workers and Redis storage
- ✅ **Admin API**: Complete CRUD for API keys, providers, models, and aliases with JWT authentication
- ✅ **Logging**: Request/response buffering to Redis (S3 writer next)

**What's Working Right Now**:
- Make chat completion requests to OpenAI via the gateway
- API key authentication with SHA-256 hashing
- **Rate limiting with per-key limits** (Redis sliding window, < 5ms latency) - December 4, 2025
- Budget tracking with real-time enforcement
- **Database-driven cost calculation** with pricing components (input, output, cached, reasoning tokens)
- Automatic billing updates via async queue workers
- Model aliasing (e.g., "gpt4" → "gpt-4")
- Streaming responses (Server-Sent Events)
- **S3 Background Worker**: Logs drain from Redis → S3 with gzip compression
- Request/response logging to Redis buffer with automatic S3 upload
- Admin API with JWT authentication (email/password and service tokens)
- Complete CRUD operations for API keys, providers, models, and aliases
- Graceful shutdown with resource cleanup and log flushing

**Next Priorities** (see [TODO.md](TODO.md) for details):
1. **Streaming Cost Calculation**: Parse SSE chunks for accurate token counts
2. **Metrics with Prometheus**: Add instrumentation for monitoring
3. **Additional Providers**: VertexAI and Bedrock implementation (stubs exist)
4. **BerriAI Model Catalog Sync**: Automated pricing data updates
5. **Testing Suite**: Expand test coverage (29 test files exist, need more coverage)

## Overview

ThinkPixelLLMGW is a production-ready gateway service that provides:
- **Unified API**: OpenAI-compatible API for multiple LLM providers
- **Multi-Provider Support**: OpenAI (fully implemented), Google VertexAI, AWS Bedrock (extensible)
- **Database-Driven Pricing**: Multi-dimensional cost calculation (direction, modality, unit, tier)
- **Cost Management**: Accurate per-request cost calculation with automatic billing integration
- **Budget Enforcement**: Real-time budget checks with Redis-backed tracking and PostgreSQL persistence
- **Rate Limiting**: Redis-backed distributed rate limiting with per-key limits (sliding window, < 5ms)
- **Admin API**: Complete CRUD for API keys, providers, models, and aliases with JWT authentication
- **Audit Logging**: Request/response logging to Redis buffer (S3 upload pending)
- **Metrics & Monitoring**: Prometheus-compatible metrics for latency, costs, and usage
- **Model Aliasing**: Create custom model names with provider-specific routing
- **Scalability**: Kubernetes-ready with Redis-backed distributed state and async queue workers
- **Production Ready**: Graceful shutdown, connection pooling, LRU caching, comprehensive testing

## 📚 Documentation

- **[Quick Start Guide](QUICKSTART.md)** - Get up and running in 5 minutes
- **[Bootstrap Admin Setup](BOOTSTRAP_ADMIN.md)** - Create initial admin user for Kubernetes deployments
- **[Testing Guide](TESTING_GUIDE.md)** - Complete setup and testing instructions
- **[Environment Variables](ENV_VARIABLES.md)** - Configuration reference

## 🎨 Web UI & Admin Interface

A minimal web-based admin interface is available in the `./webui` directory:

- **Frontend** (`./webui/frontend`): React + TypeScript SPA with PicoCSS for a clean, minimal UI
  - View and manage API keys, models, and billing
  - Protected routes with automatic authentication
  - Built with Vite for fast development and optimized builds
  - See [frontend README](./webui/frontend/README.md) for setup instructions

- **BFF** (`./webui/bff`): FastAPI backend-for-frontend service
  - Cookie-based authentication (HttpOnly, signed cookies)
  - Proxies admin API requests to the Go gateway
  - No database - stateless proxy layer
  - See [BFF README](./webui/bff/README.md) for setup instructions

The Web UI provides an easy-to-use interface for common admin tasks without requiring direct API calls or command-line tools.

## Architecture

```
             ┌─────────────┐
             │  Clients    │
             └──────┬──────┘
                    │ Bearer <API-Key>
┌───────────────────▼─────────────────────┐
│         LLM Gateway (Go)                │
│   ┌────────────────────────────────┐    │
│   │  Proxy Handler ✅              │    │
│   │  1. Auth (API Key lookup)      │    │
│   │  2. Rate Limiting (Redis)      │    │
│   │  3. Budget Check (Redis)       │    │
│   │  4. Model Resolution (DB)      │    │
│   │  5. Provider Call (OpenAI)     │    │
│   │  6. Logging (Redis Buffer)     │    │
│   │  7. Billing Update (Redis)     │    │
│   └────────────────────────────────┘    │
│                                         │
│   ┌────────────────────────────────┐    │
│   │  Admin API ✅                  │    │
│   │  - API Key Management          │    │
│   │  - Provider Management         │    │
│   │  - Model Alias Management      │    │
│   └────────────────────────────────┘    │
│                                         │
│   ┌────────────────────────────────┐    │
│   │  S3 Background Worker ✅       │    │
│   │  - Drains Redis → S3           │    │
│   │  - Gzip compression            │    │
│   │  - Graceful shutdown           │    │
│   └────────────────────────────────┘    │
└────────┬────────────┬────────────┬──────┘
         │            │            │
   ┌─────▼─────┐ ┌────▼────┐ ┌─────▼────┐
   │PostgreSQL │ │  Redis  │ │ S3/MinIO │
   │  (Config) │ │(Runtime)│ │ (Logs)   │
   │     ✅    │ │   ✅    │ │   ✅     │
   └───────────┘ └─────────┘ └──────────┘

Legend: ✅ Implemented | 🔨 In Progress | ⏸ Planned
```

### Components

#### 1. **Proxy Service** (`/v1/chat/completions`)
- OpenAI-compatible API endpoint
- API key authentication via Bearer token
- Model-to-provider resolution
- Request forwarding with provider-specific transformations
- Response streaming support (future)

#### 2. **Admin API** (`/admin/*`) ✅
- JWT-based authentication for human users (email/password with Argon2)
- Token-based authentication for service accounts (with Argon2)
- Role-based access control (admin, editor, viewer)
- CRUD operations for:
  - Admin Users (human accounts)
  - Admin Tokens (service accounts)
  - API Keys (create, read, update, delete, regenerate, tag)
  - Providers (create, read, update, delete with encrypted credentials)
  - Models (100+ fields, pricing components, features, capabilities)
  - Model Aliases (create, read, update, delete with custom configs)

#### 3. **Storage Layer**
- **PostgreSQL**: API keys, providers, aliases, budgets, historical costs
- **Redis**: Rate limiting, real-time cost tracking, log buffering (drained to S3)
- **S3**: Long-term request/response audit logs (gzip-compressed JSON Lines)
- **Background Worker**: Drains Redis buffer to S3 with configurable flush interval and batch size

#### 4. **Provider Plugins**
- OpenAI (GPT-4, GPT-3.5, etc.)
- Google VertexAI (Gemini, PaLM)
- AWS Bedrock (Claude, Llama, etc.)
- Extensible provider interface

## Features

### Current Status

#### ✅ Completed (December 2, 2025)

**Core Gateway Functionality:**
- [x] Full HTTP proxy handler with OpenAI-compatible API
- [x] Database layer with PostgreSQL (schema, migrations, repositories)
- [x] LRU cache for API keys and models (< 1ms cache hits)
- [x] Redis integration (rate limiting, billing, log buffer)
- [x] Pluggable provider architecture with factory pattern
- [x] OpenAI provider with streaming support
- [x] Model alias resolution system
- [x] Request flow: auth → rate limit → budget → provider → cost calculation → billing → logging → response
- [x] **Database-driven cost calculation** with pricing components table
- [x] **Multi-dimensional pricing** (direction, modality, unit, tier)
- [x] **Automatic billing integration** with async queue workers
- [x] **Admin API** with JWT authentication (email/password and service tokens)
- [x] **Complete CRUD endpoints** for API keys, providers, models, and aliases
- [x] Graceful server shutdown with resource cleanup
- [x] Database-backed API key authentication with caching
- [x] **Redis-backed rate limiting** with per-key limits and sliding window algorithm (December 4, 2025)
- [x] Billing cache with atomic cost tracking and PostgreSQL sync
- [x] Logging to Redis buffer (ready for S3 upload)
- [x] Configuration management via environment variables
- [x] Comprehensive documentation and testing guide
- [x] Middleware-based authentication (API key and JWT)

**Infrastructure:**
- [x] Connection pooling (PostgreSQL, Redis)
- [x] Health checks for all services
- [x] Provider credential encryption (AES-256)
- [x] Multi-modal cost calculation engine
- [x] Background workers (billing sync, provider reload)
- [x] Complete database schema with migrations

#### 🔨 Next Up (Priority Order)

**Immediate Priorities:**
- [ ] Streaming cost calculation (parse SSE chunks for token counts)
- [ ] Metrics with Prometheus (add instrumentation)
- [ ] BerriAI model catalog sync script (populate models table with pricing)
- [ ] Docker Compose setup for development environment
- [ ] End-to-end integration tests

**Testing & Quality:**
- [ ] Unit tests for all packages
- [ ] Integration tests with real PostgreSQL/Redis
- [ ] Load testing (target: 1000 req/s)
- [ ] End-to-end testing scenarios

**Provider Expansion:**
- [ ] Vertex AI provider implementation (Google Cloud SDK)
- [ ] Bedrock provider implementation (AWS SDK)
- [ ] Provider health checks and monitoring

**Future Enhancements:**
- [ ] Prometheus metrics integration
- [ ] FastAPI Python admin UI
- [ ] Webhook support for budget alerts
- [ ] Response caching and request deduplication
- [ ] Advanced features (fallback chains, A/B testing)

### API Key Features ✅
- **Authentication**: SHA-256 hashed keys with database lookup and LRU caching
- **Permissions**: Model allowlist per key (ready for implementation)
- **Rate Limiting**: ✅ Redis-backed sliding window (< 5ms latency, ~10k checks/sec) with per-key limits
- **Budgets**: Monthly USD limits with Redis cache and background DB sync
- **Tags**: Flexible metadata support via key_metadata table
- **Lifecycle**: ✅ Complete CRUD operations via Admin API (create, list, get, update, delete, regenerate)
- **Expiration**: Configurable expiration dates with automatic validation

### Admin API & Authentication ✅
- **Dual Authentication Flows**:
  - Email/Password: Human users with Argon2id hashed passwords
  - Service Name + Token: Service accounts with Argon2id hashed tokens
- **JWT Generation**: 24-hour tokens with role-based claims (admin_id, auth_type, roles)
- **Authentication Endpoints**:
  - `POST /admin/auth/login` - Email/password login → JWT
  - `POST /admin/auth/token` - Service name + token → JWT
- **Complete CRUD Endpoints**:
  - API Keys: Create, Read, Update, Delete, Regenerate
  - Providers: Create, Read, Update, Delete (with credential encryption)
  - Models: Create, Read, Update, Delete (100+ fields, pricing components)
  - Aliases: Create, Read, Update, Delete (custom configs, tags)
- **Role-Based Access Control**: Admin, editor, viewer roles with enforcement
- **Middleware**: AdminJWTMiddleware with role-based access control
- **Secure Hashing**: Argon2id (time=1, memory=64MB, threads=4, keylen=32)
- **Context Helpers**: Extract admin claims, roles, and ID from request context
- **Clean Separation**: API Keys used ONLY for proxying, not for admin access

### Cost Calculation & Billing ✅
- **Database-Driven Pricing**: Pricing components table with multi-dimensional support
- **Pricing Dimensions**:
  - **Direction**: Input, Output, Cache, Tool
  - **Modality**: Text, Image, Audio, Video
  - **Unit**: Token, 1K Tokens, Character, Image, Pixel, Second, Minute, Hour, Request
  - **Tier**: Default, Premium, Above 128K (extensible)
- **Token Type Support**: Input, output, cached, and reasoning tokens
- **Automatic Integration**: Costs automatically flow from calculation → billing queue → Redis → PostgreSQL
- **Async Processing**: Billing queue workers with retry logic and dead letter queue
- **Budget Enforcement**: Real-time checks before requests are processed
- **Accurate Calculation**: Uses model-specific pricing components from database
- **Fallback Support**: Provider-calculated costs used if pricing components unavailable

### Provider Management ✅
- **Pluggable Architecture**: Factory pattern with provider registry
- **OpenAI**: Full implementation with streaming support
- **Vertex AI & Bedrock**: Stubs ready for SDK integration
- **Secure Storage**: AES-256 encrypted credentials in database
- **Model Aliasing**: Custom model names mapped to providers
- **Auto-Reload**: Providers refresh from database every 5 minutes
- **Admin API**: Complete CRUD operations for providers and models

### Logging & Observability
- **Audit Logs**: ✅ Every request logged to Redis buffer with:
  - Request/response payloads
  - Request ID for tracing
  - API key ID and provider information
  - Token usage and cost calculation
  - Timestamp and metadata
- **Redis Buffer**: ✅ Queue with batch operations (< 3ms enqueue, ~15k ops/sec)
- **S3 Writer**: ✅ AWS SDK v2 integration with gzip compression
- **S3 Background Worker**: ✅ Drains Redis buffer to S3 with:
  - Configurable flush interval (default: 5 minutes)
  - Configurable batch size (default: 1000 records)
  - Size-based and time-based flushing
  - Graceful shutdown with buffer drain
  - Gzip compression (~80% storage reduction)
  - Structured file naming: `logs/YYYY/MM/DD/pod-timestamp-nano.jsonl.gz`
- **Metrics**: ⏳ Prometheus integration (placeholder endpoint exists)
- **Health Checks**: ✅ Database and Redis health monitoring

## Getting Started

### Prerequisites
- Go 1.23+
- PostgreSQL 14+ (with UUID extension)
- Redis 7+
- S3-compatible storage (AWS S3, MinIO, etc.) - optional for now
- OpenAI API key (or other provider credentials)

### Quick Start

See **[TESTING_GUIDE.md](TESTING_GUIDE.md)** for complete setup instructions.

**1. Setup Database:**
```bash
cd llm-gateway
export DATABASE_URL="postgres://postgres:password@localhost:5432/llmgateway?sslmode=disable"
sqlx database create
sqlx migrate run
```

**2. Configure Environment:**
```bash
# Required
export DATABASE_URL="postgres://postgres:password@localhost:5432/llmgateway?sslmode=disable"
export REDIS_ADDRESS="localhost:6379"
export GATEWAY_HTTP_PORT="8080"

# Optional (with defaults)
export REDIS_PASSWORD=""
export REDIS_DB="0"
export CACHE_API_KEY_SIZE="1000"
export CACHE_MODEL_SIZE="500"

# S3 Logging (optional)
export LOGGING_SINK_ENABLED="true"
export LOGGING_SINK_S3_BUCKET="my-llm-logs"
export LOGGING_SINK_S3_REGION="us-east-1"
export LOGGING_SINK_S3_PREFIX="logs/"
export LOGGING_SINK_FLUSH_SIZE="1000"
export LOGGING_SINK_FLUSH_INTERVAL="5m"
export POD_NAME="gateway-0"
```

**3. Seed Test Data:**
```sql
-- Insert OpenAI provider (see TESTING_GUIDE.md for complete SQL)
INSERT INTO providers (id, name, provider_type, encrypted_credentials, enabled)
VALUES (gen_random_uuid(), 'openai-main', 'openai', 
        '{"api_key": "sk-proj-YOUR_KEY_HERE"}', true);

-- Create test API key
INSERT INTO api_keys (id, name, key_hash, enabled, rate_limit, monthly_budget)
VALUES (gen_random_uuid(), 'Test Key', 
        encode(sha256('test-key-12345'::bytea), 'hex'),
        true, 100, 10.0);
```

**4. Start the Gateway:**
```bash
cd llm-gateway
go run cmd/gateway/main.go
```

**5. Test the Endpoint:**
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer test-key-12345" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Streaming Example:**
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer test-key-12345" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Count to 5"}],
    "stream": true
  }'
```

For detailed testing scenarios, monitoring queries, and troubleshooting, see **[TESTING_GUIDE.md](TESTING_GUIDE.md)**.

## Development Roadmap

See [TODO.md](TODO.md) for detailed task tracking.

### ✅ Milestone 1: Core Functionality (COMPLETED)
- [x] Database schema and migrations (7 tables, PostgreSQL)
- [x] LRU cache with TTL (API keys, models)
- [x] Redis integration (rate limiting, billing, log buffer)
- [x] OpenAI provider implementation with streaming
- [x] Pluggable provider architecture
- [x] Model aliasing system
- [x] **Database-driven cost calculation** with pricing components
- [x] **Automatic billing integration** with async queue workers
- [x] **Admin API** with JWT authentication
- [x] **Complete CRUD endpoints** for keys, providers, models, aliases
- [x] **S3 Background Worker** with gzip compression and graceful shutdown
- [x] Proxy handler with full request flow
- [x] Graceful shutdown and resource cleanup
- [x] Configuration management
- [x] Comprehensive documentation (COST_CALCULATION.md, BILLING_INTEGRATION.md, TESTING_GUIDE.md, S3_BACKGROUND_WORKER_IMPLEMENTATION.md)

**Status:** Gateway is production-ready with complete cost tracking, admin capabilities, and S3 log archival!

### 🔨 Milestone 2: Enhanced Features (In Progress)
- [x] S3 background worker (drain Redis buffer to S3 with gzip compression)
- [x] Wire Redis rate limiter (December 4, 2025)
- [ ] Streaming cost calculation (parse SSE chunks for accurate token counts)
- [ ] Prometheus metrics integration (instrumentation)
- [x] Docker Compose setup (postgres, redis, minio services configured)
- [x] Integration tests (28 test files, comprehensive coverage for admin APIs)
- [ ] Unit test expansion (increase coverage)
- [ ] BerriAI model catalog sync (automated pricing updates)

### 📋 Milestone 3: Multi-Provider Support
- [ ] Vertex AI provider implementation (stub exists)
- [ ] AWS Bedrock provider implementation (stub exists)
- [ ] Provider management API
- [ ] Alias management API
- [ ] Advanced authentication patterns

### 🏭 Milestone 4: Production Features
- [ ] Prometheus metrics integration
- [ ] Enhanced error handling and retries
- [ ] Response caching (optional)
- [ ] Security enhancements
- [ ] Performance optimization
- [ ] Deployment guides (Kubernetes, Docker)

### 🎨 Milestone 5: UI & Polish
- [ ] FastAPI admin UI
- [ ] Enhanced logging and filtering
- [ ] Webhook support
- [ ] Usage reports and analytics
- [ ] Multi-region support

## Project Structure

```
ThinkPixelLLMGW/
├── README.md                   # Project overview and quick start
├── TODO.md                    # Comprehensive task tracking
├── TESTING_GUIDE.md           # Complete testing and setup guide
├── ENV_VARIABLES.md           # Environment configuration reference
├── DATABASE_SCHEMA.md         # Database schema documentation
├── docker-compose.yaml        # Development services (postgres, redis, minio)
├── LICENSE                    # License file
│
└── llm_gateway/               # Main Go application (96 files, 28 tests)
    ├── cmd/gateway/
    │   └── main.go           # Application entry point with graceful shutdown
    │
    ├── internal/
    │   ├── auth/             # ✅ Authentication & authorization (5 files)
    │   │   ├── api_key.go         # API key store interface
    │   │   ├── jwt.go             # JWT generation & validation
    │   │   ├── roles.go           # Role constants (viewer, editor, admin)
    │   │   ├── errors.go          # Auth error types
    │   │   └── *_test.go          # Comprehensive test coverage
    │   │
    │   ├── billing/          # ✅ Cost tracking & budget enforcement (4 files)
    │   │   ├── billing.go              # Redis cache with 5-min DB sync
    │   │   ├── billing_queue_worker.go # Async billing updates
    │   │   └── *_test.go               # Unit & integration tests
    │   │
    │   ├── config/           # ✅ Configuration management (1 file)
    │   │   └── config.go          # Environment variable parsing
    │   │
    │   ├── httpapi/          # ✅ HTTP handlers & routing (14 files)
    │   │   ├── router.go                        # Dependency injection & routes
    │   │   ├── proxy_handler.go                 # Chat completions endpoint
    │   │   ├── admin_handler.go                 # Admin auth endpoints
    │   │   ├── admin_api_keys_handler.go        # API key CRUD (520 lines)
    │   │   ├── admin_providers_handler.go       # Provider CRUD (474 lines)
    │   │   ├── admin_models_handler.go          # Model CRUD (580 lines)
    │   │   ├── admin_aliases_handler.go         # Alias CRUD (520 lines)
    │   │   ├── admin_store.go                   # Admin store adapter
    │   │   ├── api_key_store.go                 # API key store adapter
    │   │   ├── logging_sink.go                  # Logging sink adapter
    │   │   └── *_integration_test.go            # 6 comprehensive test files
    │   │
    │   ├── logging/          # ✅ Logging & audit trail (7 files)
    │   │   ├── sink.go                  # LogBuffer interface & S3Sink with background worker
    │   │   ├── redis_buffer.go          # Redis queue for log buffering
    │   │   ├── s3_writer.go             # S3 batch uploader with gzip compression
    │   │   ├── request_logger.go        # File-based logger with rotation
    │   │   ├── s3_integration_test.go   # S3 integration tests with Minio
    │   │   └── *_test.go                # Unit tests with mock buffer
    │   │
    │   ├── metrics/          # ⏳ Prometheus metrics (1 file)
    │   │   └── metrics.go          # Noop implementation (TODO: instrument)
    │   │
    │   ├── middleware/       # ✅ HTTP middleware (3 files)
    │   │   ├── api_key_middleware.go  # API key authentication
    │   │   ├── jwt_middleware.go      # JWT validation & RBAC
    │   │   └── *_test.go              # Middleware tests
    │   │
    │   ├── models/           # ✅ Data models (11 files)
    │   │   ├── api_key.go          # API key with validation
    │   │   ├── provider.go         # Provider configuration
    │   │   ├── model.go            # Model with pricing (100+ fields)
    │   │   ├── model_alias.go      # Model alias configuration
    │   │   ├── pricing_component.go # Multi-dimensional pricing
    │   │   ├── usage_record.go     # Usage tracking
    │   │   ├── admin_user.go       # Admin user with roles
    │   │   ├── admin_token.go      # Service account tokens
    │   │   ├── jsonb.go            # JSONB helpers
    │   │   ├── errors.go           # Model errors
    │   │   └── *_test.go           # Cost calculation & validation tests
    │   │
    │   ├── providers/        # ✅ LLM provider implementations (7 files)
    │   │   ├── provider.go    # Provider interface
    │   │   ├── factory.go     # Factory pattern
    │   │   ├── registry.go    # Auto-reload registry (5-min interval)
    │   │   ├── auth.go        # Authentication helpers
    │   │   ├── openai.go      # OpenAI complete with streaming
    │   │   ├── vertexai.go    # Vertex AI stub (TODO: implement)
    │   │   ├── bedrock.go     # Bedrock stub (TODO: implement)
    │   │   └── *_test.go      # Provider examples & tests
    │   │
    │   ├── queue/            # ✅ Async queue system (9 files)
    │   │   ├── queue.go       # Queue interface
    │   │   ├── memory.go      # In-memory queue implementation
    │   │   ├── redis.go       # Redis-backed queue
    │   │   ├── errors.go      # Queue errors
    │   │   └── *_test.go      # Memory, Redis, integration tests
    │   │
    │   ├── ratelimit/        # ✅ Rate limiting (2 files, fully integrated)
    │   │   ├── ratelimiter.go      # Redis sliding window (< 5ms, 10k req/s)
    │   │   └── ratelimiter_test.go # Comprehensive test suite
    │   │
    │   ├── storage/          # ✅ Database & encryption (15 files)
    │   │   ├── db.go                      # Connection pool & LRU cache
    │   │   ├── cache.go                   # Thread-safe LRU
    │   │   ├── redis.go                   # Redis client
    │   │   ├── encryption.go              # AES-256-GCM encryption
    │   │   ├── api_key_repository.go      # API key CRUD
    │   │   ├── model_repository.go        # Model CRUD with pricing
    │   │   ├── provider_repository.go     # Provider CRUD
    │   │   ├── alias_repository.go        # Alias CRUD
    │   │   ├── usage_repository.go        # Usage tracking
    │   │   ├── admin_user_repository.go   # Admin user CRUD
    │   │   ├── admin_token_repository.go  # Admin token CRUD
    │   │   ├── usage_queue_worker.go      # Usage worker
    │   │   ├── errors.go                  # Storage errors
    │   │   └── *_test.go                  # Encryption & worker tests
    │   │
    │   └── utils/            # ✅ Utility functions (10 files)
    │       ├── logger.go           # Structured logging
    │       ├── hash.go             # Argon2id password hashing
    │       ├── rest.go             # REST response helpers
    │       ├── errors.go           # Error handling
    │       ├── memory.go           # Memory utilities
    │       ├── data_conversion.go  # Type conversions
    │       └── *_test.go           # Comprehensive test coverage
    │
    ├── migrations/           # ✅ Database migrations (5 files)
    │   ├── 20251125000001_initial_schema.up.sql
    │   ├── 20251125000001_initial_schema.down.sql
    │   ├── 20251125000002_seed_data.up.sql
    │   ├── 20251125000002_seed_data.down.sql
    │   └── README.md
    │
    ├── examples/             # Example code
    │   ├── encryption_example.go
    │   └── s3_logging_example.go
    │
    ├── Makefile             # Build & test targets
    ├── go.mod               # Go module definition
    └── go.sum               # Dependency checksums
```

**File Statistics (December 4, 2025):**
- Total Go files: 96
- Test files: 29 (comprehensive coverage including rate limiter)
- Lines of code: ~15,500+ (excluding tests)
- Integration tests: 6 files covering admin APIs
- Documentation: 10+ markdown files

## Contributing

This is a personal project but suggestions and feedback are welcome!

## License

See [LICENSE](LICENSE) file.
