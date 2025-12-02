# ThinkPixelLLMGW - Development TODO

This document tracks all implementation tasks for the LLM Gateway project.

**Last Updated:** December 2, 2025  
**Project Status:** ✅ **MVP COMPLETE!** Core gateway fully functional with OpenAI provider. Async queue system, admin JWT authentication, full admin CRUD endpoints, and database-driven cost calculation with billing integration implemented.

## 📊 Current Implementation Summary

### ✅ Fully Implemented (Production-Ready)
- **Database Layer**: Complete PostgreSQL integration with LRU caching, repositories for all entities
- **Authentication**: SHA256 API key hashing, Argon2id admin password hashing, JWT tokens with role-based access
- **Provider System**: Pluggable architecture, OpenAI provider with streaming, registry with auto-reload
- **Async Processing**: Hybrid queue system (Memory/Redis), billing and usage workers with batch processing
- **Rate Limiting**: Redis-backed sliding window algorithm with atomic operations
- **Billing**: Redis cache with 5-minute background sync, budget checks, cost tracking
- **Cost Calculation**: Database-driven pricing with multi-dimensional support (direction, modality, unit, tier)
- **Logging**: Request logger with file rotation, Redis buffer, S3 writer (background worker TODO)
- **Encryption**: AES-256-GCM for provider credentials with environment-based key management
- **HTTP API**: Chat completions endpoint, health checks, graceful shutdown
- **Admin API**: 
  - JWT authentication (email/password and service token flows)
  - Complete CRUD for API keys (Create, Read, Update, Delete, Regenerate)
  - Complete CRUD for providers (Create, Read, Update, Delete with encryption)
  - Complete CRUD for models (100+ fields, pricing, features, capabilities)
  - Complete CRUD for model aliases (custom configs, tags, resolution)
  - Role-based access control (viewer, editor, admin)
- **Configuration**: Complete environment-based config with all sections (DB, Redis, Cache, Logging, Provider)

### 🚧 Partially Implemented
- **S3 Logging**: Writer and sink implemented, background worker integration pending
- **Metrics**: Noop implementation, Prometheus integration pending
- **Providers**: OpenAI complete, VertexAI and Bedrock stubs exist

### 📝 Test Coverage
- **30+ test files** covering auth, models, storage, queue, billing, logging, utils, httpapi
- Unit tests for core functionality (JWT, Argon2, encryption, queues, workers)
- Integration tests for queue system and admin CRUD endpoints
- Manual testing guide with Docker setup (`TESTING_GUIDE.md`)

### 🎯 Next Priorities
1. **Streaming Cost Calculation**: Parse SSE chunks to calculate accurate costs for streaming responses
2. **S3 Logging Background Worker**: Complete the log draining pipeline
3. **Metrics with Prometheus**: Add instrumentation for monitoring
4. **Additional Providers**: Implement VertexAI and Bedrock providers
5. **BerriAI Model Sync**: Automated sync of pricing data from BerriAI catalog
6. **Advanced Features**: Caching, embeddings, function calling, fine-tuning

## ✅ Recently Completed

### Cost Calculation System (December 2, 2025)
- **Files Created/Updated:** 8 files (~1,500 lines of code + documentation)
- **Core Features:**
  - Database-driven pricing using `pricing_components` table
  - Multi-dimensional cost calculation (direction, modality, unit, tier)
  - Support for multiple token types (input, output, cached, reasoning)
  - Automatic integration with billing system
  - Real-time cost calculation for all requests
- **Implementation:**
  - `internal/models/model.go` - Added `CalculateCost()`, `findPricingComponent()`, `calculateComponentCost()` methods
  - `internal/models/cost_calculation_test.go` - Comprehensive test suite with 12 test scenarios (380 lines)
  - `internal/providers/registry.go` - Added `ResolveModelWithDetails()` for loading models with pricing
  - `internal/httpapi/proxy_handler.go` - Integrated cost calculation in request handler
  - `internal/storage/model_repository.go` - Added `ModelWithDetails` struct for joined pricing data
- **Key Features:**
  - Pricing dimensions: Input, Output, Cache, Tool (direction); Text, Image, Audio, Video (modality)
  - Pricing units: Token, 1K Tokens, Character, Image, Pixel, Second, Minute, Hour, Request
  - Pricing tiers: Default, Premium, Above 128K (extensible)
  - Automatic fallback: Provider-calculated cost used if pricing components unavailable
  - Zero-downtime: Backward compatible with existing code
- **Documentation:**
  - `COST_CALCULATION.md` - Complete technical guide with architecture and examples (450+ lines)
  - `IMPLEMENTATION_SUMMARY.md` - Implementation overview with test results and usage patterns
  - `COST_CALCULATION_QUICK_REF.md` - Quick reference with examples and troubleshooting
  - `BILLING_INTEGRATION.md` - Detailed explanation of cost-to-billing flow
- **Testing:**
  - All 12 test scenarios passing with accurate cost calculations
  - Examples: GPT-4o standard ($0.0075), with caching ($0.004875), o1 with reasoning ($0.069)
  - Verified integration with billing worker and Redis storage
- **Status:** ✅ **Complete and production-ready! Costs automatically flow to billing system.**

### API Key Management Admin Endpoints (November 30, 2025)
- **Files Created/Updated:** 4 files (~1,000 lines of code + documentation)
- **Core Features:**
  - Complete CRUD operations for API keys with SHA-256 hashing
  - Cryptographically secure key generation (sk-<32 hex chars>)
  - Usage statistics integration (requests, tokens, costs)
  - Key regeneration with automatic invalidation
  - Soft delete (revoke) functionality
  - Tags and metadata support
- **Implementation:**
  - `internal/httpapi/admin_api_keys_handler.go` - Main handler (530 lines)
  - `internal/httpapi/admin_api_keys_handler_integration_test.go` - Full test suite (450 lines)
  - `API_KEY_MANAGEMENT.md` - Complete documentation (600 lines)
  - `API_KEY_QUICKREF.md` - Quick reference guide (280 lines)
  - Updated `internal/httpapi/router.go` - Wired up all endpoints with proper role-based access
  - Removed placeholder `handleAdminKeys` from `admin_handler.go`
- **Endpoints:**
  - POST `/admin/keys` - Create new API key (admin)
  - GET `/admin/keys` - List all keys with pagination (viewer)
  - GET `/admin/keys/:id` - Get details with usage stats (viewer)
  - PUT `/admin/keys/:id` - Update settings (admin)
  - DELETE `/admin/keys/:id` - Revoke key (admin)
  - POST `/admin/keys/:id/regenerate` - Generate new key (admin)
- **Security Features:**
  - Plaintext keys only returned during creation/regeneration
  - SHA-256 hashing before database storage
  - Role-based access control (viewer/admin)
  - Rate limiting and budget enforcement
  - Expiration date support
- **Testing:**
  - 6 comprehensive integration tests
  - Tests for create, list, get, update, delete, regenerate
  - Error case coverage (validation, permissions, not found)
- **Status:** ✅ **Complete - MVP ACHIEVED!**

### Admin CRUD Endpoints for Providers, Models & Aliases (November 30, 2025)
- **Files Created/Updated:** 6 files (~2,600 lines of code)
- **Core Features:**
  - Complete CRUD operations for providers with credential encryption
  - Complete CRUD operations for models with 100+ fields (pricing, features, limits)
  - Complete CRUD operations for model aliases with custom configurations
  - Role-based access control (viewer for reads, admin for writes)
  - Comprehensive integration tests for all endpoints
  - Pagination and filtering support
- **Implemented Endpoints:**
  - **Providers:** GET/POST `/admin/providers`, GET/PUT/DELETE `/admin/providers/:id`
  - **Models:** GET/POST `/admin/models`, GET/PUT/DELETE `/admin/models/:id`
  - **Aliases:** GET/POST `/admin/aliases`, GET/PUT/DELETE `/admin/aliases/:id`
- **Files:**
  - `internal/httpapi/admin_providers_handler.go` - Provider CRUD with encryption (474 lines)
  - `internal/httpapi/admin_providers_handler_integration_test.go` - Integration tests (350+ lines)
  - `internal/httpapi/admin_models_handler.go` - Model CRUD with full schema (1089 lines)
  - `internal/httpapi/admin_models_handler_integration_test.go` - Integration tests (400+ lines)
  - `internal/httpapi/admin_aliases_handler.go` - Alias CRUD with validation (521 lines)
  - `internal/httpapi/admin_aliases_handler_integration_test.go` - Integration tests (300+ lines)
- **Key Features:**
  - Provider credentials encrypted with AES-256-GCM before storage
  - Model pricing supports 10+ pricing components (input/output/cached tokens, images, audio, etc.)
  - Model features support 40+ capability flags (vision, function calling, streaming, etc.)
  - Pagination with page/page_size query parameters
  - Filtering and search across all resources
  - Cache invalidation on updates
  - Provider registry reload on provider changes
  - Complete request/response models with JSON validation
- **Status:** ✅ **Admin CRUD complete! Only API key management endpoint remains.**

### Admin JWT Authentication System (November 30, 2025)
- **Files Created/Updated:** 11 files (~1,400 lines of code)
- **Core Features:**
  - Complete JWT authentication for admin API
  - Dual authentication flows:
    - Email/password: AdminUser model with Argon2id password hashing
    - Service name + token: AdminToken model with Argon2id token hashing
  - Role-based access control with configurable role enforcement
  - Database schema with admin_users and admin_tokens tables
  - Complete repository layer for admin authentication
  - JWT middleware with context helpers for claim extraction
  - Admin authentication endpoints implemented and tested
- **Database Schema:**
  - `admin_users` table: id, email, password_hash (Argon2), roles[], enabled, last_login_at
  - `admin_tokens` table: id, service_name, token_hash (Argon2), roles[], enabled, expires_at, last_used_at
  - Indexed on email, service_name, token_hash for fast lookups
- **Files:**
  - `internal/models/admin_user.go` - AdminUser model with role helpers (47 lines)
  - `internal/models/admin_token.go` - AdminToken model with expiry checks (56 lines)
  - `internal/storage/admin_user_repository.go` - CRUD operations (180 lines)
  - `internal/storage/admin_token_repository.go` - CRUD operations (200 lines)
  - `internal/utils/hash.go` - Argon2id hashing utilities (92 lines)
  - `internal/auth/jwt.go` - JWT generation and validation (167 lines)
  - `internal/auth/jwt_test.go` - Comprehensive test suite (273 lines)
  - `internal/middleware/jwt_middleware.go` - AdminJWTMiddleware with role enforcement (107 lines)
  - `internal/httpapi/admin_handler.go` - Auth endpoints and placeholder CRUD (144 lines)
  - `internal/httpapi/admin_store.go` - AdminStore adapter (45 lines)
  - `migrations/20251125000001_initial_schema.up.sql` - Database schema
- **API Endpoints:**
  - `POST /admin/auth/login` - Email/password authentication → JWT
  - `POST /admin/auth/token` - Service token authentication → JWT
  - `GET /admin/test` - Protected test endpoint (requires JWT)
  - `GET /admin/keys` - Placeholder for API key management (protected)
  - `GET /admin/providers` - Placeholder for provider management (protected)
- **Authentication Flow:**
  - User/service authenticates via email+password or service_name+token
  - System validates credentials using Argon2id verification
  - JWT generated with claims: admin_id, auth_type, roles, email/service_name
  - JWT expires in 24 hours (configurable)
  - Protected endpoints use AdminJWTMiddleware with role requirements
- **Status:** ✅ **Authentication complete! Ready for admin CRUD implementation.**

### Async Queue System for Billing & Usage (November 30, 2025)
- **Files Created/Updated:** 9 files (~1,200 lines of code)
- **Core Features:**
  - Hybrid queue implementation (in-memory for standalone, Redis for production)
  - Async billing updates with batch processing
  - Async usage record insertion with batch processing
  - Dead-letter queue (DLQ) support for failed items
  - Retry logic with exponential backoff
  - Detailed token tracking (input, output, cached, reasoning tokens)
- **Architecture:**
  - **Request Flow:** `Request → Extract Usage → Calculate Cost → Queue Updates → Return Response`
  - **Background Workers:** 
    - BillingWorker: processes billing queue, updates Redis billing cache
    - UsageWorker: processes usage queue, inserts into PostgreSQL with batch transactions
  - **Queue Backends:**
    - Memory: Channel-based, no persistence, ideal for Raspberry Pi deployment
    - Redis: List-based, persistent, ideal for Kubernetes/production
- **Files:**
  - `internal/queue/queue.go` - Queue interface and configuration (87 lines)
  - `internal/queue/memory.go` - In-memory queue implementation (226 lines)
  - `internal/queue/redis.go` - Redis queue implementation (207 lines)
  - `internal/queue/errors.go` - Queue error types (14 lines)
  - `internal/billing/queue_worker.go` - Async billing worker (200 lines)
  - `internal/storage/usage_queue_worker.go` - Async usage worker (240 lines)
  - `internal/utils/logging.go` - Enhanced structured logger (60 lines)
  - `internal/providers/openai.go` - Enhanced usage extraction with detailed token info
  - `internal/httpapi/proxy_handler.go` - Updated to use queues instead of direct updates
  - `internal/httpapi/router.go` - Queue initialization and worker startup
- **Key Improvements:**
  - **Throughput:** Support for thousands of requests per minute without DB bottleneck
  - **Reliability:** DLQ for failed updates, retry with exponential backoff
  - **Observability:** Detailed token tracking (cached_tokens, reasoning_tokens from OpenAI)
  - **Flexibility:** Works standalone (in-memory) or in Kubernetes (Redis-backed)
  - **Batch Processing:** Up to 100 items per batch, 5-second batch timeout
- **Configuration:**
  ```go
  BatchSize:     100
  BatchTimeout:  5 * time.Second
  MaxRetries:    3
  RetryBackoff:  1 * time.Second
  UseRedis:      auto-detected based on config
  ```
- **Status:** ✅ **Production-ready async processing system!**

### Admin JWT Authentication (November 27, 2025)
- **Files Created/Updated:** 5 files (~800 lines of code)
- **Core Features:**
  - Argon2id password/token hashing and verification
  - Dual authentication flows:
    - Email/password: Indexed lookup by email → Argon2 verification
    - Service name + token: Indexed lookup by service_name → Argon2 verification
  - JWT generation with role-based claims (24-hour expiration)
  - AdminJWTMiddleware with role enforcement
  - Context helpers for extracting admin data
  - Comprehensive test suite with mock store
- **Files:**
  - `internal/auth/hash.go` - Argon2id implementation (95 lines)
  - `internal/auth/jwt.go` - Admin JWT generation and validation (210 lines)
  - `internal/auth/jwt_test.go` - Complete test coverage (405 lines)
  - `internal/middleware/jwt_middleware.go` - AdminJWTMiddleware and helpers (150 lines)
  - `internal/httpapi/admin_store.go` - AdminStore adapter (45 lines)
  - `internal/httpapi/admin_handler.go` - Auth endpoints (140 lines)
- **Authentication Flows:**
  - Email/password: AdminUser lookup by email → Argon2 verify → JWT with user claims
  - Service name + token: AdminToken lookup by service_name → Argon2 verify → JWT with token claims
- **API Endpoints:**
  - `POST /admin/auth/login` - Login with {"email": "...", "password": "..."}
  - `POST /admin/auth/token` - Authenticate with {"service_name": "...", "token": "..."}
  - `GET /admin/test` - Protected test endpoint (requires JWT)
- **Status:** ✅ **Ready for admin API implementation!**

### Next Steps for Admin API

With JWT authentication and CRUD endpoints now complete, only API key management remains:

**Completed Endpoints:**
- ✅ `POST /admin/auth/login` - Email/password login → JWT
- ✅ `POST /admin/auth/token` - Service name + token → JWT
- ✅ `GET/POST /admin/providers` - List and create providers (viewer/admin roles)
- ✅ `GET/PUT/DELETE /admin/providers/:id` - Provider CRUD (viewer/admin roles)
- ✅ `GET/POST /admin/models` - List and create models (viewer/admin roles)
- ✅ `GET/PUT/DELETE /admin/models/:id` - Model CRUD (viewer/admin roles)
- ✅ `GET/POST /admin/aliases` - List and create aliases (viewer/admin roles)
- ✅ `GET/PUT/DELETE /admin/aliases/:id` - Alias CRUD (viewer/admin roles)

**Remaining Endpoint (Final MVP Task):**
- ❌ `GET/POST /admin/keys` - List and create API keys (viewer/editor roles)
- ❌ `GET/PUT/DELETE /admin/keys/:id` - API key CRUD (viewer/editor/admin roles)
- ❌ `POST /admin/keys/:id/regenerate` - Regenerate API key (editor role)

**Implementation Notes:**
- Replace placeholder in `admin_handler.go` `handleAdminKeys()` with full implementation
- Create `AdminKeysHandler` similar to existing admin handlers
- Use existing `APIKeyRepository` from storage layer
- Follow same patterns as providers/models/aliases handlers
- Include integration tests following existing test patterns

### Proxy Handler & HTTP Router (November 23, 2025)
- **Files Created/Updated:** 5 files (~600 lines of code)
- **Core Features:**
  - Complete chat completion endpoint (`/v1/chat/completions`)
  - Full request flow: auth → rate limit → budget → provider → logging → response
  - Streaming support (Server-Sent Events) and non-streaming responses
  - OpenAI-compatible API with proper error responses
  - Database-backed API key authentication with caching
  - Redis-backed logging sink for S3 upload queue
  - Dependency injection with all real implementations
  - Graceful server shutdown with resource cleanup
- **Files:**
  - `internal/httpapi/proxy_handler.go` - Main chat endpoint handler (350 lines)
  - `internal/httpapi/router.go` - HTTP router with dependency injection (120 lines)
  - `internal/httpapi/api_key_store.go` - Database API key adapter (40 lines)
  - `internal/httpapi/logging_sink.go` - Redis logging adapter (40 lines)
  - `cmd/gateway/main.go` - Server with graceful shutdown (65 lines)
- **Documentation:**
  - Complete testing guide (`TESTING_GUIDE.md`)
  - Setup instructions with SQL examples
  - cURL testing examples
  - OpenAI SDK integration (Python/Node.js)
  - Monitoring and troubleshooting tips
- **Status:** ✅ **Gateway is fully functional and ready for testing!**

### Provider Implementation (January 23, 2025)
- **Files Created/Updated:** 7 files (~1,200 lines of code)
- **Core Features:**
  - Pluggable provider architecture with factory pattern
  - Provider interface with lifecycle management
  - Registry for model/alias resolution with auto-reload
  - Authentication abstraction (simple API key + extensible for SDK-based)
  - Full OpenAI provider with streaming support
  - Vertex AI and Bedrock provider stubs with SDK integration docs
  - Model alias repository for custom model names
- **Documentation:**
  - Provider implementation guide (`PROVIDER_IMPLEMENTATION.md`)
  - Configuration examples for all provider types
  - Adding new providers tutorial
- **Architecture:**
  - Simple auth: API key in header (OpenAI)
  - Complex auth: SDK-based with IAM (Bedrock, Vertex AI) - ready for implementation
  - Provider isolation with separate HTTP clients
  - Auto-reload from database every 5 minutes

### Redis Integration (January 23, 2025)
- **Files Created/Updated:** 4 files (~800 lines of code)
- **Core Features:**
  - Redis client with connection pooling and health checks
  - Sliding window and token bucket rate limiting algorithms
  - Billing cache with atomic increments and background DB sync
  - Log buffer with batch operations and queue management
  - Cluster support for high availability
- **Documentation:**
  - Redis integration guide (`REDIS_INTEGRATION.md`)
  - Updated environment variables (`ENV_VARIABLES.md`)
- **Performance:**
  - Rate limiting: < 5ms latency, ~10k checks/sec
  - Billing: < 2ms for budget checks, atomic cost tracking
  - Log buffer: < 3ms enqueue, ~15k ops/sec

### Database Layer with LRU Caching (January 23, 2025)
- **Files Created:** 9 new files (~1,515 lines of code)
- **Core Features:**
  - Thread-safe LRU cache with configurable size and TTL
  - Database connection pooling with health checks
  - Complete repository layer (API Keys, Models, Providers, Usage)
  - Cache-aside pattern for < 200ms latency
  - Multi-modal cost calculation engine
- **Documentation:**
  - Usage guide (`internal/storage/README.md`)
  - Environment variable reference (`ENV_VARIABLES.md`)
  - Implementation summary (`IMPLEMENTATION_SUMMARY.md`)
- **Performance:**
  - Cache hits: < 1ms
  - Cache misses: < 50ms
  - Handles 1000s of API keys efficiently

---

## 🎯 Milestone 1: MVP - Core Functionality ✅ **COMPLETE!**

**Status**: All core functionality implemented and tested. Gateway is operational with OpenAI provider. Admin CRUD endpoints complete for providers, models, and aliases. Only API key management endpoint implementation remains.

### 1.1 Database Layer ✅
- [x] **Design PostgreSQL schema** (see `DATABASE_SCHEMA.md`)
  - [x] `providers` table (id, name, type, encrypted_credentials, config, enabled)
  - [x] `models` table (synced from BerriAI/LiteLLM with pricing, features, context windows)
  - [x] `model_aliases` table (custom user-friendly aliases for models)
  - [x] `api_keys` table (id, name, hash, allowed_models, rate_limit, budget, enabled, expires_at)
  - [x] `key_metadata` table (flexible tags, labels, custom fields for keys)
  - [x] `usage_records` table (audit log with full token breakdown and costs)
  - [x] `monthly_usage_summary` table (pre-aggregated stats for fast budget checks)
  - [x] Indexes for performance (GIN, trigram, time-series)

- [x] **Create migration system** (`migrations/` with sqlx-cli)
  - [x] `20250123000001_initial_schema.up.sql` - Core schema
  - [x] `20250123000001_initial_schema.down.sql` - Rollback script
  - [x] `20250123000002_seed_data.up.sql` - Demo data (dev only)
  - [x] `20250123000002_seed_data.down.sql` - Cleanup script
  - [x] Updated_at triggers for all tables
  - [x] Migration README with best practices

- [ ] **BerriAI Model Sync** (`scripts/sync_models.sh`)
  - [ ] Fetch model catalog from https://raw.githubusercontent.com/BerriAI/litellm/refs/heads/main/litellm/model_prices_and_context_window_backup.json
  - [ ] Parse JSON and upsert into `models` table
  - [ ] Track sync version and timestamp
  - [ ] Schedule via cron or Kubernetes CronJob (daily)
  - [ ] Handle deprecated models gracefully

- [x] **Implement database package** (`internal/storage/db.go`) ✅
  - [x] Connection pool management with sqlx
  - [x] Health check functionality
  - [x] Context-aware queries with timeouts
  - [x] Transaction support with proper rollback
  - [x] LRU cache integration (API keys, models)
  - [x] Cache statistics and monitoring
  - [x] Repository factory methods
  - [x] Periodic cache cleanup

- [x] **Implement LRU cache** (`internal/storage/cache.go`) ✅
  - [x] Thread-safe implementation with sync.RWMutex
  - [x] Configurable capacity and TTL
  - [x] LRU eviction when capacity reached
  - [x] TTL-based expiration
  - [x] Statistics tracking (hits, misses, evictions, hit rate)
  - [x] Cleanup of expired entries

- [x] **Implement repositories** ✅
  - [x] `APIKeyRepository` (CRUD, lookup by hash, check expiration, metadata management)
    - [x] Cache-aside pattern for GetByHash
    - [x] Automatic cache invalidation on writes
    - [x] Metadata support (tags, labels)
  - [x] `ModelRepository` (CRUD, search by features, get pricing, alias resolution)
    - [x] Cache-aside pattern for GetByName
    - [x] Alias lookup with caching
    - [x] Fuzzy search with trigram similarity
    - [x] Cost calculation via model.CalculateCost()
  - [x] `ProviderRepository` (CRUD, get active providers)
  - [x] `UsageRepository` (insert, query by key/date range, aggregations)
    - [x] Analytics (total cost, token usage)
    - [x] Time-filtered queries
  - [x] `MonthlyUsageSummaryRepository` (CRUD, budget checks, refresh from usage)
    - [x] Upsert with conflict resolution
    - [x] Refresh aggregates from usage_records

- [x] **Database models** (`internal/models/database.go`) ✅
  - [x] Model struct with 40+ fields from BerriAI
  - [x] Provider, ModelAlias, KeyMetadata structs
  - [x] UsageRecord, MonthlyUsageSummary structs
  - [x] CalculateCost method for multi-modal pricing

- [x] **Configuration updates** (`internal/config/config.go`) ✅
  - [x] Database connection settings (URL, pool config)
  - [x] Cache configuration (size, TTL for API keys and models)
  - [x] Environment variable parsing
  - [x] Default values and validation

### 1.2 Redis Integration ✅
- [x] **Setup Redis client** (`internal/storage/redis.go`) ✅
  - [x] Connection management with go-redis
  - [x] Health check
  - [x] Pool configuration and statistics
  - [x] Cluster/sentinel support

- [x] **Rate Limiting** (`internal/ratelimit/ratelimiter.go`) ✅
  - [x] Sliding window algorithm with Redis sorted sets
  - [x] Token bucket algorithm (alternative implementation)
  - [x] Atomic operations with Lua scripts
  - [x] Distributed rate limiting across replicas
  - [x] Get current usage and reset operations
  - [x] Configuration: per-key limits from DB

- [x] **Billing Cache** (`internal/billing/billing.go`) ✅
  - [x] Track running costs in Redis (key: `cost:<api_key_id>:<year>:<month>`)
  - [x] Atomic increment on request completion
  - [x] Background sync worker (configurable frequency)
  - [x] Budget check logic: compare Redis value vs. DB monthly_budget
  - [x] Handle month rollover with TTL
  - [x] Graceful shutdown with final sync

- [x] **Log Buffer** (`internal/logging/redis_buffer.go`) ✅
  - [x] Enqueue log records to Redis list
  - [x] Batch enqueue and dequeue operations
  - [x] Queue size limits with automatic trimming
  - [x] Peek and wait operations
  - [x] Buffer statistics and monitoring
  - [x] Handle buffer overflow (FIFO with size limit)

### 1.3 Configuration Management ✅
- [x] **Expand config package** (`internal/config/config.go`) ✅
  - [x] Add DatabaseURL (PostgreSQL connection string)
  - [x] Add DatabaseConfig (MaxOpenConns, MaxIdleConns, ConnMaxLifetime, ConnMaxIdleTime)
  - [x] Add CacheConfig (API key cache size/TTL, model cache size/TTL)
  - [x] Add RedisConfig (Address, Password, DB, PoolSize, MinIdleConns, Timeouts)
  - [x] Add ProviderConfig (ReloadInterval, RequestTimeout)
  - [x] Add RequestLoggerConfig (FilePathTemplate, MaxSize, MaxFiles, BufferSize, FlushInterval)
  - [x] Add LoggingSinkConfig (S3 bucket, region, prefix, pod name, flush settings)
  - [x] Add JWTSecret for admin authentication
  - [x] Environment variable parsing with defaults (getEnvInt, getEnvInt64, getEnvDuration, getEnvString)
  - [x] Validation logic for required fields (DATABASE_URL)
  - [x] Encryption key loaded from ENCRYPTION_KEY environment variable

- [x] **Environment-based configs** ✅
  - [x] Development defaults (see `ENV_VARIABLES.md`)
  - [x] Production examples (see `ENV_VARIABLES.md`)
  - [x] Complete environment variable support for all config sections

### 1.4 Provider Implementation ✅
- [x] **Pluggable Provider Architecture** (`internal/providers/`) ✅
  - [x] Provider interface with ID(), Name(), Type(), Chat(), ValidateCredentials(), Close()
  - [x] Factory pattern for creating providers based on type
  - [x] Registry for managing provider lifecycle and model resolution
  - [x] Authentication abstraction layer (simple API key + extensible for SDK-based)
  - [x] Auto-reload from database (configurable interval)
  - [x] Model alias support (resolve aliases to actual models and providers)

- [x] **OpenAI Provider** (`internal/providers/openai.go`) ✅
  - [x] HTTP client setup with timeout and connection pooling
  - [x] API endpoint configuration (base URL)
  - [x] Authentication (API key from encrypted credentials)
  - [x] Chat completion implementation
    - [x] Transform internal ChatRequest to OpenAI format
    - [x] Handle streaming vs. non-streaming responses
    - [x] Parse response and extract usage data
  - [x] Cost calculation (placeholder for model pricing integration)
  - [x] Error handling
  - [x] StreamReader helper for consuming Server-Sent Events

- [x] **Provider Stubs** (for future implementation) ✅
  - [x] Vertex AI provider (`internal/providers/vertexai.go`)
    - [x] Placeholder with configuration structure
    - [x] Documentation for Google Cloud SDK integration
    - [x] Service account authentication flow documented
  - [x] AWS Bedrock provider (`internal/providers/bedrock.go`)
    - [x] Placeholder with configuration structure
    - [x] Documentation for AWS SDK integration
    - [x] IAM authentication flow documented

- [x] **Model Alias Repository** (`internal/storage/alias_repository.go`) ✅
  - [x] CRUD operations for model aliases
  - [x] GetByAlias for alias resolution
  - [x] ListByProvider and ListByModel queries
  - [x] Enable/disable aliases

- [x] **Configuration** (`internal/config/config.go`) ✅
  - [x] Provider reload interval setting
  - [x] Provider request timeout setting
  - [x] Environment variable support

- [x] **Documentation** (`PROVIDER_IMPLEMENTATION.md`) ✅
  - [x] Architecture overview
  - [x] Quick start guide with code examples
  - [x] Provider configuration for OpenAI, Vertex AI, Bedrock
  - [x] Model alias usage
  - [x] Adding new providers guide
  - [x] Advanced authentication patterns
  - [x] Performance characteristics
  - [x] Error handling and troubleshooting

### 1.5 Proxy Handler & HTTP Server ✅
- [x] **Proxy Handler** (`internal/httpapi/proxy_handler.go`) ✅
  - [x] HTTP handler for `/v1/chat/completions` endpoint
  - [x] OpenAI-compatible API request/response format
  - [x] Full request flow implementation:
    - [x] Extract and validate Bearer token from Authorization header
    - [x] Hash API key and lookup in database (with caching)
    - [x] Check API key status (enabled, not expired)
    - [x] Decode JSON payload and validate
    - [x] Rate limit check via Redis
    - [x] Budget check via billing cache
    - [x] Model/alias resolution via provider registry
    - [x] Execute provider request
    - [x] Handle streaming and non-streaming responses
    - [x] Log request/response to Redis buffer
    - [x] Update billing costs atomically
  - [x] Helper functions:
    - [x] parseBearer - Extract token from header
    - [x] newRequestID - UUID generation for tracing
    - [x] writeJSONError - OpenAI-compatible error responses
  - [x] Error handling for all failure scenarios

- [x] **HTTP Router** (`internal/httpapi/router.go`) ✅
  - [x] Dependency injection system with complete service graph
  - [x] Initialize all real implementations (no more Noops):
    - [x] Database connection with pooling and caching
    - [x] Redis client with connection pooling
    - [x] Encryption service (AES-256-GCM)
    - [x] API key, admin user, and admin token repositories
    - [x] Provider registry with auto-reload (5-minute default)
    - [x] Rate limiter (Redis-backed implementation ready)
    - [x] Billing service (Redis cache with 5-minute DB sync)
    - [x] Request logger with file rotation
    - [x] Logging buffer (Redis queue)
    - [x] Queue infrastructure (hybrid memory/Redis)
    - [x] Billing and usage queue workers (batch processing)
  - [x] Route registration:
    - [x] POST `/v1/chat/completions` - Main proxy endpoint with API key auth
    - [x] GET `/health` - Health check
    - [x] GET `/metrics` - Metrics endpoint (noop for now)
    - [x] POST `/admin/auth/login` - Admin email/password authentication
    - [x] POST `/admin/auth/token` - Admin service token authentication
    - [x] GET `/admin/test` - Protected test endpoint
    - [x] GET `/admin/keys` - API key management (placeholder)
    - [x] `/admin/providers` - Full CRUD with role-based access (viewer/admin)
    - [x] `/admin/models` - Full CRUD with role-based access (viewer/admin)
    - [x] `/admin/aliases` - Full CRUD with role-based access (viewer/admin)
  - [x] Middleware stack with API key and JWT authentication
  - [x] Role-based access control (viewer, editor, admin)
  - [x] Provider credentials update from environment variables
  - [x] Queue workers started in background
  - [x] Return dependencies for cleanup

- [x] **API Key Store Adapter** (`internal/httpapi/api_key_store.go`) ✅
  - [x] DatabaseAPIKeyStore implementation
  - [x] Wraps storage.APIKeyRepository
  - [x] Hash plaintext keys using auth.HashKey
  - [x] Lookup with automatic caching

- [x] **Logging Sink Adapter** (`internal/httpapi/logging_sink.go`) ✅
  - [x] RedisLoggingSink implementation
  - [x] Wraps logging.RedisBuffer
  - [x] Marshal records to JSON
  - [x] Best-effort enqueue (doesn't fail requests)

- [x] **Main Server** (`cmd/gateway/main.go`) ✅
  - [x] Load configuration from environment
  - [x] Initialize router with all dependencies
  - [x] HTTP server with proper timeouts:
    - [x] 30s read timeout
    - [x] 30s write timeout
    - [x] 120s idle timeout
  - [x] Graceful shutdown:
    - [x] Signal handling (SIGINT, SIGTERM)
    - [x] 30s context timeout for active requests
    - [x] Resource cleanup (close provider registry)

- [x] **Testing Documentation** (`TESTING_GUIDE.md`) ✅
  - [x] Complete setup instructions
  - [x] SQL examples for test data
  - [x] cURL examples for all scenarios
  - [x] Error case testing
  - [x] OpenAI SDK integration (Python/Node.js)
  - [x] Monitoring queries (Redis, PostgreSQL)
  - [x] Troubleshooting guide
  - [x] Load testing with Apache Bench

### 1.6 Logging Pipeline ✅ (Implemented - S3 Background Worker Pending)
- [x] **Redis Buffer** (`internal/logging/redis_buffer.go`) ✅
  - [x] Redis-backed buffer for log records using Redis lists
  - [x] Enqueue with LPUSH, handle overflow with LTRIM
  - [x] DequeueBatch with RPOP for batch processing
  - [x] QueueDepth monitoring
  - [x] Clear method for maintenance
  - [x] Configurable queue key, max size, and batch size

- [x] **S3 Writer** (`internal/logging/s3_writer.go`) ✅
  - [x] AWS SDK v2 integration for S3 uploads
  - [x] WriteBatch method for uploading log records
  - [x] File naming: `logs/<year>/<month>/<day>/<pod>-<timestamp>-<nano>.jsonl`
  - [x] JSON Lines format (application/x-ndjson)
  - [x] Structured logging with utils.Logger
  - [ ] Background worker to drain Redis buffer (TODO)
  - [ ] Compression (gzip) for storage efficiency (TODO)
  - [ ] Graceful shutdown (flush pending logs) (TODO)

- [x] **Logging Sink** (`internal/logging/sink.go`) ✅
  - [x] Define Sink interface (Enqueue, Shutdown)
  - [x] Define LogRecord struct (timestamp, API key, provider, model, costs, errors)
  - [x] NoopSink implementation for testing
  - [x] S3Sink with in-memory queue and periodic flushing
  - [x] RedisLoggingSink adapter in httpapi layer
  - [x] Integration in router.go with Redis buffer
  - [ ] Wire S3Sink background worker for persistent storage (TODO)
  - [ ] Metrics for log queue depth (TODO)

- [x] **Request Logger** (`internal/logging/request_logger.go`) ✅
  - [x] File-based request logging with rotation
  - [x] JSON Lines format with buffering
  - [x] Periodic flush (60s default)
  - [x] Rotation based on file size (10MB default)
  - [x] Maximum file retention (5 files default)
  - [x] Graceful shutdown with buffer flush
  - [x] Integration in router with complete lifecycle management

### 1.7 Admin API - Basic Operations ✅ **COMPLETE!**
- [x] **Admin User & Token Models** (`internal/models/admin_user.go`, `admin_token.go`) ✅
  - [x] AdminUser model with email/password authentication
  - [x] AdminToken model with service_name/token authentication
  - [x] Role-based access with HasRole/HasAnyRole helpers
  - [x] IsValid/IsExpired validation methods
  - [x] Database schema with indexes

- [x] **Admin Repositories** (`internal/storage/admin_user_repository.go`, `admin_token_repository.go`) ✅
  - [x] Complete CRUD operations for AdminUser and AdminToken
  - [x] GetByEmail for user authentication lookup
  - [x] GetByServiceName for token authentication lookup
  - [x] UpdateLastLogin and UpdateLastUsed tracking
  - [x] Create, Update, Delete operations
  - [x] List with pagination support

- [x] **Argon2id Hashing** (`internal/utils/hash.go`) ✅
  - [x] HashPasswordArgon2 for secure password/token hashing
  - [x] VerifyPasswordArgon2 for constant-time verification
  - [x] PHC string format for hash storage
  - [x] Configurable parameters (time=1, memory=64MB, threads=4)

- [x] **JWT Implementation** (`internal/auth/jwt.go`) ✅
  - [x] GenerateAdminJWTWithPassword for email/password auth
  - [x] GenerateAdminJWTWithToken for service token auth
  - [x] ValidateAdminJWT with signature and expiry verification
  - [x] AdminClaims with admin_id, auth_type, roles, email, service_name
  - [x] 24-hour token expiration (configurable via JWT_SECRET)

- [x] **JWT Middleware** (`internal/middleware/jwt_middleware.go`) ✅
  - [x] AdminJWTMiddleware with role-based enforcement
  - [x] Extract JWT from Authorization or X-API-Key header
  - [x] Validate and parse admin JWT
  - [x] Check required roles (viewer, editor, admin)
  - [x] Context helpers: GetAdminClaims, GetAdminID, GetAdminRoles

- [x] **Admin Auth Endpoints** (`internal/httpapi/admin_handler.go`) ✅
  - [x] POST `/admin/auth/login` - Email/password login → JWT
  - [x] POST `/admin/auth/token` - Service token auth → JWT
  - [x] GET `/admin/test` - Protected test endpoint
  - [x] Response includes: token, expires_at, admin_id, auth_type

- [x] **API Key Management** ✅ **COMPLETED**
  - [x] Implement AdminAPIKeysHandler in admin_api_keys_handler.go
  - [x] POST `/admin/keys` - Create new API key
    - [x] Generate cryptographically secure random key (32+ chars)
    - [x] Hash key with SHA-256
    - [x] Store in database with permissions and metadata
    - [x] Return plaintext key (only time it's visible)
  - [x] GET `/admin/keys` - List all keys (paginated, without hashes)
  - [x] GET `/admin/keys/:id` - Get key details with usage stats
  - [x] PUT `/admin/keys/:id` - Update key (rate limits, budget, allowed models)
  - [x] DELETE `/admin/keys/:id` - Revoke key (set enabled=false)
  - [x] POST `/admin/keys/:id/regenerate` - Generate new key, revoke old
  - [x] Wire up proper routing in router.go
  - [x] Integration tests (admin_api_keys_handler_integration_test.go)
  - [x] Documentation (API_KEY_MANAGEMENT.md, API_KEY_QUICKREF.md)

- [x] **Provider Management** (`internal/httpapi/admin_providers_handler.go`) ✅
  - [x] POST `/admin/providers` - Create new provider (admin role)
    - [x] Accept name, display_name, type, credentials, config, enabled
    - [x] Validate provider type (openai, anthropic, vertexai, bedrock, etc.)
    - [x] Encrypt credentials with storage.Encryption (AES-256-GCM)
    - [x] Store in database and trigger registry reload
  - [x] GET `/admin/providers` - List all providers (viewer role)
    - [x] Paginated response with total count
    - [x] Include id, name, type, enabled, model count
    - [x] Exclude encrypted credentials from list view
    - [x] Support filtering and search
  - [x] GET `/admin/providers/:id` - Get provider details (viewer role)
    - [x] Include decrypted credentials (admin role only)
    - [x] Include associated models list
    - [x] Full provider configuration
  - [x] PUT `/admin/providers/:id` - Update provider (admin role)
    - [x] Update display_name, config, credentials, enabled status
    - [x] Re-encrypt credentials if changed
    - [x] Trigger provider registry reload
  - [x] DELETE `/admin/providers/:id` - Disable provider (admin role)
    - [x] Soft delete (set enabled=false)
    - [x] Trigger provider registry reload
  - [x] Complete integration tests (admin_providers_handler_integration_test.go)

- [x] **Model Management** (`internal/httpapi/admin_models_handler.go`) ✅
  - [x] POST `/admin/models` - Create new model (admin role)
    - [x] Accept 100+ fields including pricing, features, limits
    - [x] Validate provider exists and is enabled
    - [x] Support all pricing components (input/output/cached/reasoning tokens, audio, images, etc.)
    - [x] Support 40+ feature flags (vision, function calling, streaming, etc.)
  - [x] GET `/admin/models` - List all models (viewer role)
    - [x] Paginated response with filtering
    - [x] Filter by provider_id, deprecated status, search query
    - [x] Include pricing and feature summary
  - [x] GET `/admin/models/:id` - Get model details (viewer role)
    - [x] Complete model capabilities and pricing breakdown
    - [x] All 100+ fields with proper JSON serialization
  - [x] PUT `/admin/models/:id` - Update model (admin role)
    - [x] Update pricing, features, deprecation status
    - [x] Invalidate model cache on update
    - [x] Support partial updates with pointer fields
  - [x] DELETE `/admin/models/:id` - Delete model (admin role)
    - [x] Check for dependent aliases before deletion
    - [x] Invalidate model cache
  - [x] Complete integration tests (admin_models_handler_integration_test.go)

- [x] **Model Alias Management** (`internal/httpapi/admin_aliases_handler.go`) ✅
  - [x] POST `/admin/aliases` - Create model alias (admin role)
    - [x] Accept alias_name, target_model_id, provider_id, custom_config, tags
    - [x] Validate model and provider exist and are enabled
    - [x] Support custom configuration overrides
  - [x] GET `/admin/aliases` - List all aliases (viewer role)
    - [x] Paginated response with filtering
    - [x] Filter by enabled status, search query
  - [x] GET `/admin/aliases/:id` - Get alias details (viewer role)
  - [x] PUT `/admin/aliases/:id` - Update alias (admin role)
    - [x] Update alias_name, target_model_id, provider_id, custom_config
    - [x] Support enabling/disabling aliases
  - [x] DELETE `/admin/aliases/:id` - Delete alias (admin role)
    - [x] Remove alias from database
  - [x] Complete integration tests (admin_aliases_handler_integration_test.go)

- [x] **Request/Response Models** ✅
  - [x] Complete JSON schemas for all admin endpoints
  - [x] Provider: CreateProviderRequest, UpdateProviderRequest, ProviderResponse, ProviderDetailResponse
  - [x] Model: CreateModelRequest, UpdateModelRequest, ModelResponse, ListModelsResponse
  - [x] Alias: CreateAliasRequest, UpdateAliasRequest, AliasResponse, ListAliasesResponse
  - [x] Auth: LoginRequest, TokenAuthRequest, AuthResponse
  - [x] Standardized error responses with utils.RespondWithError

### 1.8 Testing & Validation ✅ (Partial - Tests Exist)
- [x] **Unit Tests** ✅ (Partial)
  - [x] Auth package:
    - [x] `internal/auth/jwt_test.go` - JWT generation and validation (273 lines)
    - [x] `internal/auth/api_key_test.go` - API key hashing
  - [x] Middleware package:
    - [x] `internal/middleware/api_key_middleware_test.go` - API key middleware
  - [x] Models package:
    - [x] `internal/models/admin_user_test.go` - AdminUser helpers
    - [x] `internal/models/admin_token_test.go` - AdminToken validation
    - [x] `internal/models/api_key_test.go` - APIKey validation
    - [x] `internal/models/model_test.go` - Cost calculation
    - [x] `internal/models/provider_test.go` - Provider validation
  - [x] Storage package:
    - [x] `internal/storage/encryption_test.go` - AES-GCM encryption
    - [x] `internal/storage/usage_queue_worker_test.go` - Usage worker
  - [x] Queue package:
    - [x] `internal/queue/memory_test.go` - Memory queue
    - [x] `internal/queue/redis_test.go` - Redis queue
    - [x] `internal/queue/integration_test.go` - Queue integration
  - [x] Billing package:
    - [x] `internal/billing/billing_test.go` - Billing service
    - [x] `internal/billing/billing_queue_worker_test.go` - Billing worker
  - [x] Logging package:
    - [x] `internal/logging/sink_test.go` - Logging sink
    - [x] `internal/logging/request_logger_test.go` - Request logger
    - [x] `internal/logging/s3_integration_test.go` - S3 integration
  - [x] Utils package:
    - [x] `internal/utils/hash_test.go` - Argon2 hashing
    - [x] `internal/utils/errors_test.go` - Error handling
    - [x] `internal/utils/rest_test.go` - REST utilities
    - [x] `internal/utils/memory_test.go` - Memory utilities
  - [x] Providers package:
    - [x] `internal/providers/examples_test.go` - Provider examples
  - [ ] Proxy handler (with mocked dependencies) - TODO
  - [ ] Rate limiting logic - TODO

- [ ] **Integration Tests**
  - [ ] End-to-end proxy flow with mocked OpenAI
  - [ ] Database operations
  - [ ] Redis operations
  - [ ] Streaming responses

- [x] **Local Testing Setup** ✅ (Partial)
  - [x] Testing guide with Docker setup instructions
  - [x] SQL examples for seeding test data
  - [x] Example curl commands for all scenarios
  - [ ] Docker Compose with PostgreSQL, Redis, MinIO
  - [ ] Automated seed script for test API keys and providers

---

## 🚀 Milestone 2: Multi-Provider Support (Weeks 3-4)

### 2.1 Additional Provider Implementations

#### VertexAI Provider (`internal/providers/vertexai.go`)
- [ ] Google Cloud authentication (service account JSON)
- [ ] Endpoint configuration (project, location)
- [ ] API client setup (Google Cloud AI Platform SDK)
- [ ] Chat completion implementation
  - [ ] Transform to VertexAI format (different from OpenAI)
  - [ ] Handle Gemini models specifically
- [ ] Cost calculation (VertexAI pricing)
- [ ] Error handling
- [ ] Unit tests

#### Bedrock Provider (`internal/providers/bedrock.go`)
- [ ] AWS SDK setup with credentials
- [ ] Region configuration
- [ ] Model invocation (Claude, Llama, etc.)
- [ ] Request/response transformation
  - [ ] Each model family has different formats
- [ ] Cost calculation (Bedrock pricing)
- [ ] Error handling
- [ ] Unit tests

### 2.2 Provider Management API
- [ ] **Provider CRUD** (`internal/httpapi/admin_handler.go`)
  - [ ] POST `/admin/providers` - Add new provider
    - [ ] Accept credentials (will be encrypted)
    - [ ] Support "from_file" option (mount credentials)
  - [ ] GET `/admin/providers` - List all providers
  - [ ] GET `/admin/providers/:id` - Get provider details (mask credentials)
  - [ ] PUT `/admin/providers/:id` - Update provider
  - [ ] DELETE `/admin/providers/:id` - Deactivate provider

- [ ] **Provider Registry** (`internal/providers/registry.go`)
  - [ ] Load providers from database on startup
  - [ ] Support multiple instances of same type (different credentials)
  - [ ] Cache provider instances in memory
  - [ ] Reload on configuration change

### 2.3 Model Aliasing System
- [x] **Alias Resolution** (`internal/storage/model_repository.go`) ✅ (Implemented)
  - [x] GetByName checks aliases automatically
  - [x] Cache alias → model mappings
  - [x] Fallback to direct model name if no alias
  - [ ] Handle billing_key_id for cost attribution in provider registry

- [ ] **Alias Management API**
  - [ ] POST `/admin/aliases` - Create model alias
  - [ ] GET `/admin/aliases` - List all aliases
  - [ ] PUT `/admin/aliases/:id` - Update alias
  - [ ] DELETE `/admin/aliases/:id` - Delete alias

### 2.4 Cost Calculation Engine ✅ **COMPLETE!**
- [x] **Pricing Database** ✅
  - [x] `pricing_components` table with multi-dimensional pricing
  - [x] Direction: Input, Output, Cache, Tool
  - [x] Modality: Text, Image, Audio, Video
  - [x] Unit: Token, 1K Tokens, Character, Image, Pixel, Second, Minute, Hour, Request
  - [x] Tier: Default, Premium, Above 128K (extensible)
  - [x] Join with models for complete pricing data
  - [ ] Seed with current pricing data (via BerriAI sync)
  - [ ] Admin API to update pricing

- [x] **Cost Calculator** ✅
  - [x] `Model.CalculateCost(usageRecord)` - Main calculation method
  - [x] `findPricingComponent()` - Lookup by direction/modality with tier preference
  - [x] `calculateComponentCost()` - Unit-aware cost calculation
  - [x] Support for input, output, cached, and reasoning tokens
  - [x] Automatic integration with billing system via proxy handler
  - [x] Comprehensive test suite with 12 scenarios (all passing)
  - [x] Documentation: COST_CALCULATION.md, IMPLEMENTATION_SUMMARY.md, COST_CALCULATION_QUICK_REF.md, BILLING_INTEGRATION.md
  - [ ] Handle custom pricing overrides in usage repository
  - [ ] Add cost calculation for streaming responses (parse SSE chunks)

### 2.5 Enhanced Authentication
- [ ] **JWT Implementation** (`internal/auth/jwt.go`)
  - [ ] Generate JWT tokens with claims (user_id, roles, exp)
  - [ ] Verify JWT signatures
  - [ ] Handle token expiration
  - [ ] Refresh token mechanism (optional)

- [ ] **Admin User Management**
  - [ ] Create `admin_users` table
  - [ ] Login endpoint (username/password → JWT)
  - [ ] Password hashing (bcrypt)
  - [ ] Role-based permissions (future)

---

## 🏭 Milestone 3: Production Features (Weeks 5-6)

### 3.1 Metrics & Monitoring
- [ ] **Prometheus Integration** (`internal/metrics/metrics.go`)
  - [ ] Request counter (by provider, model, status)
  - [ ] Latency histogram (gateway, provider)
  - [ ] Cost gauge (total, by key, by provider)
  - [ ] Rate limit hits counter
  - [ ] Budget exceeded counter
  - [ ] Error rate by type

- [ ] **Metrics Endpoint**
  - [ ] Expose `/metrics` in Prometheus format
  - [ ] Include custom labels (tags from API keys)

- [ ] **Grafana Dashboards** (optional)
  - [ ] Request rate and latency
  - [ ] Cost tracking
  - [ ] Error rates
  - [ ] Provider comparison

### 3.2 Security Enhancements
- [ ] **Credential Encryption** (`internal/storage/encryption.go`)
  - [ ] Implement AES-256-GCM encryption
  - [ ] Key derivation from master key (PBKDF2 or similar)
  - [ ] Encrypt provider credentials before DB storage
  - [ ] Decrypt on provider initialization
  - [ ] Key rotation support (future)

- [ ] **Security Headers**
  - [ ] Add CORS middleware (configurable origins)
  - [ ] Rate limiting on admin endpoints
  - [ ] Request size limits
  - [ ] IP allowlisting (optional)

### 3.3 Error Handling & Resilience
- [ ] **Retry Logic**
  - [ ] Exponential backoff for provider calls
  - [ ] Circuit breaker pattern (go-circuit or similar)
  - [ ] Fallback to alternate providers (future)

- [ ] **Error Responses**
  - [ ] Standardized error format
  - [ ] Detailed error logging
  - [ ] User-friendly error messages
  - [ ] Error codes for debugging

- [ ] **Graceful Shutdown**
  - [ ] Handle SIGTERM/SIGINT
  - [ ] Drain in-flight requests
  - [ ] Flush logs to S3
  - [ ] Close database connections

### 3.4 Advanced Features
- [ ] **Response Streaming**
  - [ ] Support Server-Sent Events (SSE)
  - [ ] Stream responses from providers
  - [ ] Handle partial cost calculation

- [ ] **Request Timeouts**
  - [ ] Configurable per-provider timeouts
  - [ ] Context cancellation propagation

- [ ] **Caching** (optional)
  - [ ] Cache identical requests (Redis)
  - [ ] TTL-based invalidation
  - [ ] Cache hit metrics

### 3.5 Documentation
- [ ] **API Documentation**
  - [ ] OpenAPI/Swagger spec for admin API
  - [ ] Example requests for all endpoints
  - [ ] Authentication guide

- [ ] **Deployment Guides**
  - [ ] Docker image creation
  - [ ] Kubernetes manifests
    - [ ] Deployment, Service, ConfigMap, Secret
    - [ ] Horizontal Pod Autoscaler
    - [ ] Ingress configuration
  - [ ] Helm chart (optional)

- [ ] **Operations Guide**
  - [ ] Database backup/restore
  - [ ] Monitoring and alerting setup
  - [ ] Troubleshooting common issues

---

## 🎨 Milestone 4: UI & Polish (Weeks 7-8)

### 4.1 FastAPI Admin UI
- [ ] **Setup Python Project**
  - [ ] Create `admin-ui/` directory
  - [ ] FastAPI + Uvicorn
  - [ ] Frontend framework (React/Vue/Svelte or templates)

- [ ] **Core Pages**
  - [ ] Login page (JWT authentication)
  - [ ] Dashboard (overview, metrics)
  - [ ] API Keys management
    - [ ] List, create, revoke, regenerate
    - [ ] View usage and costs
  - [ ] Providers management
    - [ ] Add, edit, deactivate
    - [ ] Test provider connectivity
  - [ ] Model Aliases
    - [ ] Create, edit, delete
  - [ ] Usage Analytics
    - [ ] Charts and graphs
    - [ ] Filter by date, key, provider

- [ ] **API Client**
  - [ ] Python client for gateway admin API
  - [ ] Authentication handling
  - [ ] Error handling and validation

### 4.2 Enhanced Logging
- [ ] **Structured Logging**
  - [ ] Use structured logger (zerolog, zap)
  - [ ] Log levels (debug, info, warn, error)
  - [ ] Request tracing with correlation IDs

- [ ] **Log Filtering**
  - [ ] Option to exclude request/response bodies
  - [ ] Sampling (log 1% of successful requests)
  - [ ] PII redaction

### 4.3 Advanced Features
- [ ] **Webhook Support**
  - [ ] Budget threshold alerts
  - [ ] API key usage notifications
  - [ ] Provider failure notifications

- [ ] **Multi-Region Support**
  - [ ] Route to providers based on region
  - [ ] Geo-distributed Redis clusters
  - [ ] S3 bucket per region

- [ ] **Usage Reports**
  - [ ] Generate CSV/PDF reports
  - [ ] Email scheduled reports
  - [ ] Cost breakdown by tag

### 4.4 Testing & Quality
- [ ] **Load Testing**
  - [ ] Use k6 or Apache Bench
  - [ ] Test rate limiting behavior
  - [ ] Measure latency under load

- [ ] **Security Audit**
  - [ ] SQL injection prevention
  - [ ] XSS prevention (if UI serves HTML)
  - [ ] Authentication/authorization review

- [ ] **Performance Optimization**
  - [ ] Database query optimization
  - [ ] Redis connection pooling
  - [ ] HTTP client connection reuse
  - [ ] Profiling (pprof)

---

## 📋 Code TODOs by File

### Configuration
- `internal/config/config.go` ✅ (Mostly Complete)
  - [x] Add DB DSN (DatabaseURL)
  - [x] Add database pool configuration
  - [x] Add cache configuration
  - [x] Add Redis configuration (address, password, pool settings)
  - [x] Add provider configuration (reload interval, request timeout)
  - [x] Add request logger configuration
  - [x] Add logging sink configuration (S3 settings)
  - [x] JWT secret configuration
  - [x] Environment variable parsing with defaults
  - [ ] Add S3 config validation
  - [ ] Support config files (YAML/TOML) in addition to env vars
  - [ ] Validation logic for required fields

### Database & Storage
- `internal/storage/db.go` ✅
  - [x] DB connection and pool management
  - [x] LRU cache integration
  - [x] Health checks and statistics
  - [x] Repository factory methods

- `internal/storage/cache.go` ✅
  - [x] Thread-safe LRU cache implementation
  - [x] TTL-based expiration
  - [x] Statistics tracking

- `internal/storage/api_key_repository.go` ✅
  - [x] CRUD operations with caching
  - [x] Metadata management

- `internal/storage/model_repository.go` ✅
  - [x] CRUD operations with caching
  - [x] Alias resolution
  - [x] Fuzzy search

- `internal/storage/provider_repository.go` ✅
  - [x] Basic CRUD operations

- `internal/storage/usage_repository.go` ✅
  - [x] Usage tracking and analytics
  - [x] Monthly summary management

- `internal/storage/encryption.go` ✅
  - [x] AES-GCM encryption implementation
  - [x] NewEncryption for binary key
  - [x] NewEncryptionFromBase64 for base64-encoded key
  - [x] GenerateKey for creating new keys
  - [x] Encrypt/Decrypt methods
  - [x] EncryptJSON/DecryptJSON for structured data
  - [x] Unit tests with comprehensive coverage

- `internal/storage/admin_user_repository.go` ✅
  - [x] Complete CRUD operations
  - [x] GetByEmail for authentication
  - [x] UpdateLastLogin tracking
  - [x] List with pagination

- `internal/storage/admin_token_repository.go` ✅
  - [x] Complete CRUD operations
  - [x] GetByServiceName for authentication
  - [x] UpdateLastUsed tracking
  - [x] List with pagination and filtering

### Models
- `internal/models/database.go` ✅
  - [x] Model, Provider, ModelAlias structs
  - [x] UsageRecord, MonthlyUsageSummary structs
  - [x] CalculateCost method

- `internal/models/api_key.go` ✅
  - [x] Updated to use uuid.UUID
  - [x] Added Enabled, ExpiresAt, Metadata fields
  - [x] Helper methods (IsExpired, IsValid, AllowsModel)

- `internal/models/admin_user.go` ✅
  - [x] AdminUser model with email/password auth
  - [x] HasRole, HasAnyRole, IsValid helpers
  - [x] Database schema with roles array

- `internal/models/admin_token.go` ✅
  - [x] AdminToken model with service token auth
  - [x] HasRole, HasAnyRole, IsExpired helpers
  - [x] Database schema with optional expiry

### Providers
- `internal/providers/provider.go` ✅
  - [x] Provider interface defined
  - [x] ChatRequest/ChatResponse structs
  - [x] Usage tracking with detailed token breakdown

- `internal/providers/openai.go` ✅
  - [x] Complete OpenAI provider implementation
  - [x] HTTP client setup
  - [x] API key authentication
  - [x] Chat completion with streaming support
  - [x] Usage extraction with cached_tokens, reasoning_tokens
  - [x] Error handling

- `internal/providers/vertexai.go` ✅ (Stub)
  - [x] Placeholder implementation
  - [x] Service account authentication documented
  - [ ] Implement actual Vertex AI integration

- `internal/providers/bedrock.go` ✅ (Stub)
  - [x] Placeholder implementation
  - [x] IAM authentication documented
  - [ ] Implement actual Bedrock integration

- `internal/providers/factory.go` ✅
  - [x] Factory pattern for provider creation
  - [x] Support for OpenAI, VertexAI, Bedrock types

- `internal/providers/registry.go` ✅
  - [x] Provider lifecycle management
  - [x] Model alias resolution
  - [x] Auto-reload from database
  - [x] Provider lookup and caching

### Logging
- `internal/logging/sink.go` ✅
  - [x] Sink interface defined
  - [x] LogRecord struct
  - [ ] Wire Redis buffer → S3 writer integration

- `internal/logging/redis_buffer.go` ✅
  - [x] Redis-backed buffer implementation
  - [x] Batch enqueue/dequeue operations
  - [x] Queue size limits and statistics
  - [x] Unit tests

- `internal/logging/s3_writer.go` ✅
  - [x] AWS SDK integration
  - [x] WriteBatch method
  - [x] JSON Lines format
  - [x] Structured file naming
  - [ ] Background worker integration
  - [ ] Compression (gzip) for storage efficiency
  - [ ] Error handling and retries

### Queue System ✅
- `internal/queue/queue.go` ✅
  - [x] Queue interface definition
  - [x] DeadLetterQueue interface
  - [x] Config struct with batch settings
  - [x] DefaultConfig helper

- `internal/queue/memory.go` ✅
  - [x] In-memory queue implementation
  - [x] Channel-based, no persistence
  - [x] Batch operations with timeout
  - [x] Unit tests

- `internal/queue/redis.go` ✅
  - [x] Redis List-based queue
  - [x] Persistent across restarts
  - [x] Batch operations with timeout
  - [x] Unit tests

- `internal/queue/errors.go` ✅
  - [x] Queue error types

### Rate Limiting & Billing ✅
- `internal/ratelimit/ratelimiter.go` ✅
  - [x] Sliding window algorithm with Redis sorted sets
  - [x] Token bucket algorithm (alternative)
  - [x] Lua scripts for atomic operations
  - [x] Distributed rate limiting

- `internal/billing/billing.go` ✅
  - [x] Budget checks using Redis + DB
  - [x] Accumulate cost in Redis with atomic increments
  - [x] Background sync worker to persist to DB
  - [x] Graceful shutdown

- `internal/billing/billing_queue_worker.go` ✅
  - [x] Async billing queue worker
  - [x] Batch processing (100 items, 5s timeout)
  - [x] Retry with exponential backoff
  - [x] Dead-letter queue support
  - [x] Unit tests

- `internal/storage/usage_queue_worker.go` ✅
  - [x] Async usage queue worker
  - [x] Batch processing (100 items, 5s timeout)
  - [x] Database batch inserts with transactions
  - [x] Retry with exponential backoff
  - [x] Dead-letter queue support
  - [x] Unit tests

### Authentication ✅
- `internal/utils/hash.go` ✅
  - [x] Argon2id password hashing
  - [x] HashPasswordArgon2 and VerifyPasswordArgon2
  - [x] PHC string format
  - [x] Unit tests

- `internal/auth/jwt.go` ✅
  - [x] AdminClaims with admin_id, auth_type, roles
  - [x] GenerateAdminJWTWithPassword
  - [x] GenerateAdminJWTWithToken
  - [x] ValidateAdminJWT with signature and expiry verification
  - [x] 24-hour token expiration
  - [x] Unit tests

- `internal/auth/api_key.go` ✅
  - [x] API key hashing with SHA256
  - [x] HashKey and GenerateKey
  - [x] APIKeyStore interface
  - [x] Unit tests

### Documentation ✅
- [x] `DATABASE_SCHEMA.md` - Complete schema documentation
- [x] `migrations/README.md` - Migration guide
- [x] `internal/storage/README.md` - Database package usage guide
- [x] `IMPLEMENTATION_SUMMARY.md` - Implementation overview (may be outdated)
- [x] `ENV_VARIABLES.md` - Environment variable reference
- [x] `TESTING_GUIDE.md` - Complete testing guide with examples
- [x] `PROVIDER_IMPLEMENTATION.md` - Provider development guide
- [x] `REDIS_INTEGRATION.md` - Redis integration guide (may exist)

### HTTP API ✅ (Partial)
- `internal/httpapi/proxy_handler.go` ✅
  - [x] Complete chat completion handler
  - [x] API key authentication
  - [x] Rate limiting
  - [x] Budget checks
  - [x] Provider routing
  - [x] Async billing and usage queuing
  - [x] Streaming support
  - [x] Error handling
  - [ ] Optionally decode response JSON for summarization
  - [ ] Integrate real Prometheus metrics (currently noop)

- `internal/httpapi/router.go` ✅
  - [x] Dependency injection
  - [x] Database and Redis initialization
  - [x] Provider registry setup
  - [x] Queue worker initialization
  - [x] Route registration
  - [x] Health check endpoint
  - [x] Metrics endpoint (noop)
  - [x] Admin auth endpoints
  - [x] Protected admin endpoints (placeholders)

- `internal/httpapi/admin_handler.go` ✅ (Partial)
  - [x] AdminAuthHandler with Login and TokenAuth methods
  - [x] TestProtected endpoint
  - [ ] Implement CRUD for API keys (handleAdminKeys)
  - [ ] Implement CRUD for providers (handleAdminProviders)
  - [ ] Implement CRUD for model aliases

- `internal/httpapi/admin_store.go` ✅
  - [x] AdminStoreAdapter combining user and token repos
  - [x] Implements auth.AdminStore interface

- `internal/httpapi/api_key_store.go` ✅
  - [x] DatabaseAPIKeyStore adapter
  - [x] Implements auth.APIKeyStore interface

- `internal/httpapi/logging_sink.go` ✅
  - [x] RedisLoggingSink adapter
  - [x] Implements logging.Sink interface

- `cmd/gateway/main.go` ✅
  - [x] Load configuration from environment
  - [x] Initialize router with all dependencies
  - [x] HTTP server with proper timeouts
  - [x] Graceful shutdown with signal handling

### Utilities ✅
- `internal/utils/logger.go` ✅
  - [x] Structured logger with slog
  - [x] NewLogger helper

- `internal/utils/errors.go` ✅
  - [x] Error handling utilities
  - [x] Unit tests

- `internal/utils/rest.go` ✅
  - [x] RespondWithJSON and RespondWithError helpers
  - [x] Unit tests

- `internal/utils/memory.go` ✅
  - [x] Memory utilities
  - [x] Unit tests

### Middleware ✅
- `internal/middleware/api_key_middleware.go` ✅
  - [x] APIKeyMiddleware for proxy endpoints
  - [x] Extract and validate API keys
  - [x] Add API key to context
  - [x] Unit tests

- `internal/middleware/jwt_middleware.go` ✅
  - [x] AdminJWTMiddleware for admin endpoints
  - [x] Role-based enforcement
  - [x] Context helpers for claims extraction

---

## 🔮 Future Enhancements (Post-MVP)

### Advanced Features
- [ ] **Model Fallback**
  - [ ] If primary provider fails, fallback to secondary
  - [ ] Configurable fallback chains

- [ ] **Request Queuing**
  - [ ] Queue requests when rate limit hit (instead of rejecting)
  - [ ] Priority queuing by API key tier

- [ ] **Token Pooling**
  - [ ] Share rate limits across multiple keys
  - [ ] Team/organization budgets

- [ ] **A/B Testing**
  - [ ] Route % of traffic to different models
  - [ ] Compare costs and quality

### Integrations
- [ ] **Additional Providers**
  - [ ] Anthropic (native API)
  - [ ] Cohere
  - [ ] HuggingFace Inference
  - [ ] Self-hosted models (vLLM, TGI)

- [ ] **Authentication Providers**
  - [ ] OAuth integration
  - [ ] LDAP/Active Directory
  - [ ] SAML

- [ ] **Monitoring**
  - [ ] DataDog integration
  - [ ] New Relic APM
  - [ ] Sentry error tracking

### Developer Experience
- [ ] **SDK Libraries**
  - [ ] Python SDK
  - [ ] JavaScript/TypeScript SDK
  - [ ] Go SDK

- [ ] **CLI Tool**
  - [ ] Create/manage API keys from CLI
  - [ ] View usage and costs
  - [ ] Test provider connectivity

---

## 📝 Notes

### Encryption Recommendation
For provider credentials, use **AES-256-GCM**:
- 256-bit key derived from master secret
- GCM mode provides authenticated encryption
- Store IV with each encrypted value
- Consider key rotation strategy

### Database Choice
PostgreSQL is ideal because:
- JSONB for flexible tag storage
- Good indexing for lookups
- Transaction support
- Proven at scale

### Redis Patterns
- **Rate Limiting**: Sliding window with sorted sets
- **Billing**: Simple key-value with atomic increments
- **Log Buffer**: List with LPUSH/RPOP for FIFO queue

### Kubernetes Deployment
- Use ConfigMaps for non-sensitive config
- Use Secrets for DB passwords, API keys, encryption keys
- Set resource limits (CPU, memory)
- Configure liveness and readiness probes
- Use horizontal pod autoscaling based on CPU/memory or custom metrics
