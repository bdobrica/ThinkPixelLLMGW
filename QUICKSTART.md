# Quick Start Guide - ThinkPixelLLMGW

## Overview

This guide will help you get started with development on ThinkPixelLLMGW.

## Prerequisites

- **Go**: 1.23 or higher ([install](https://go.dev/dl/))
- **Docker**: For running PostgreSQL, Redis, MinIO ([install](https://docs.docker.com/get-docker/))
- **Git**: For version control
- **IDE**: VS Code, GoLand, or your preferred editor

## Quick Setup (5 minutes)

### 1. Clone the Repository

```bash
git clone https://github.com/bdobrica/ThinkPixelLLMGW.git
cd ThinkPixelLLMGW/llm-gateway
```

### 2. Install Go Dependencies

```bash
go mod download
```

### 3. Start Infrastructure (Docker Compose)

Create `docker-compose.yml` in the project root:

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: llmgateway
      POSTGRES_USER: llmgateway
      POSTGRES_PASSWORD: llmgateway_dev_password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio_data:/data

volumes:
  postgres_data:
  redis_data:
  minio_data:
```

Start services:

```bash
docker-compose up -d
```

Verify services are running:

```bash
docker-compose ps
```

### 4. Configure Environment

Create `.env` file in `llm-gateway/`:

```bash
# HTTP Server
GATEWAY_HTTP_PORT=8080

# Database
DATABASE_URL=postgres://llmgateway:llmgateway_dev_password@localhost:5432/llmgateway?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379/0

# S3 / MinIO
S3_ENDPOINT=http://localhost:9000
S3_BUCKET=llm-logs
S3_REGION=us-east-1
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin

# JWT (for admin API)
JWT_SECRET=dev-secret-change-in-production

# Encryption (32 bytes hex for AES-256)
ENCRYPTION_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

### 5. Run the Gateway

```bash
go run cmd/gateway/main.go
```

You should see:

```
llm-gateway listening on :8080
```

### 6. Test the Proxy Endpoint

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer demo-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Note**: This currently uses placeholder implementations. You'll get a stubbed response.

---

## Development Workflow

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/auth/...

# Verbose output
go test -v ./...
```

### Code Style

Format code before committing:

```bash
go fmt ./...
```

Run linter (install golangci-lint first):

```bash
golangci-lint run
```

### Database Migrations (Once Implemented)

```bash
# Run migrations
go run cmd/migrate/main.go up

# Rollback
go run cmd/migrate/main.go down

# Create new migration
go run cmd/migrate/main.go create add_users_table
```

### Debugging

**With VS Code**:

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch Gateway",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/llm-gateway/cmd/gateway",
      "env": {
        "GATEWAY_HTTP_PORT": "8080"
      }
    }
  ]
}
```

Press F5 to start debugging.

**With GoLand**:

Right-click `cmd/gateway/main.go` → Debug 'go build'

---

## Next Steps for Development

Based on `TODO.md` and `DEVELOPMENT_PLAN.md`, the immediate priorities are:

### Week 1-2: Foundation

1. **Database Schema** (Start Here!)
   - Create `internal/storage/migrations/001_initial_schema.sql`
   - Define tables: api_keys, providers, model_aliases, usage_history
   - Implement repository interfaces

2. **Redis Integration**
   - Rate limiter using Redis
   - Billing cache with atomic increments
   - Log buffer with Redis lists

3. **Configuration**
   - Expand `internal/config/config.go` with all settings
   - Add validation

**Task Breakdown in TODO.md** - See Phase 1 for detailed subtasks.

### Suggested First Pull Request

**Title**: "Add PostgreSQL schema and migrations"

**Changes**:
- `internal/storage/migrations/001_initial_schema.sql`
- `internal/storage/db.go` (connection setup)
- `internal/storage/repositories.go` (basic CRUD)
- Update `go.mod` with database driver
- Tests for repositories

This gives you a solid foundation to build on.

---

## Useful Commands

### Docker Compose

```bash
# Start services
docker-compose up -d

# Stop services
docker-compose stop

# View logs
docker-compose logs -f postgres
docker-compose logs -f redis

# Remove everything (fresh start)
docker-compose down -v
```

### PostgreSQL Access

```bash
# Connect to database
docker exec -it <postgres-container-name> psql -U llmgateway -d llmgateway

# Inside psql
\dt          # List tables
\d api_keys  # Describe api_keys table
SELECT * FROM api_keys;
\q           # Quit
```

### Redis Access

```bash
# Connect to Redis CLI
docker exec -it <redis-container-name> redis-cli

# Inside redis-cli
KEYS *                           # List all keys
GET cost:api-key-123:2024-01    # Get billing data
LRANGE log_buffer 0 10          # View log buffer
```

### MinIO Access

Open browser: http://localhost:9001
- Username: `minioadmin`
- Password: `minioadmin`

Create bucket `llm-logs` in the UI.

---

## Common Issues

### Port Already in Use

If you get "port already in use" errors:

```bash
# Find process using port 8080
lsof -i :8080  # macOS/Linux
netstat -ano | findstr :8080  # Windows

# Kill the process or change port in .env
```

### Database Connection Failed

Check PostgreSQL is running:

```bash
docker-compose ps
```

Test connection:

```bash
docker exec -it <postgres-container-name> psql -U llmgateway -d llmgateway -c "SELECT 1;"
```

### Redis Connection Failed

Verify Redis is accessible:

```bash
docker exec -it <redis-container-name> redis-cli PING
# Should return: PONG
```

---

## Project Structure Reference

```
llm-gateway/
├── cmd/
│   └── gateway/
│       └── main.go              # Application entry point
├── internal/
│   ├── auth/                    # Authentication & authorization
│   │   ├── api_key.go          # API key lookup
│   │   ├── jwt.go              # JWT handling
│   │   └── hash.go             # Hashing utilities
│   ├── billing/                 # Cost tracking & budgets
│   │   └── billing.go
│   ├── config/                  # Configuration management
│   │   └── config.go
│   ├── httpapi/                 # HTTP handlers
│   │   ├── router.go           # Route setup
│   │   ├── proxy_handler.go    # Proxy endpoint
│   │   ├── admin_handler.go    # Admin API
│   │   └── jwt_middleware.go   # JWT middleware
│   ├── logging/                 # Logging pipeline
│   │   ├── sink.go
│   │   ├── redis_buffer.go
│   │   └── s3_writer.go
│   ├── metrics/                 # Metrics (Prometheus)
│   │   └── metrics.go
│   ├── models/                  # Data models
│   │   ├── api_key.go
│   │   ├── provider.go
│   │   ├── alias.go
│   │   └── errors.go
│   ├── providers/               # LLM provider implementations
│   │   ├── provider.go         # Interface
│   │   ├── registry.go         # Provider registry
│   │   ├── openai.go
│   │   ├── vertexai.go
│   │   └── bedrock.go
│   ├── ratelimit/              # Rate limiting
│   │   └── ratelimiter.go
│   └── storage/                # Database & encryption
│       ├── db.go
│       ├── encryption.go
│       └── migrations/
├── go.mod
└── go.sum
```

---

## Learning Resources

### Go
- [Official Go Tutorial](https://go.dev/tour/)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go by Example](https://gobyexample.com/)

### PostgreSQL with Go
- [pgx driver documentation](https://github.com/jackc/pgx)
- [sqlx tutorial](http://jmoiron.github.io/sqlx/)

### Redis with Go
- [go-redis documentation](https://redis.uptrace.dev/)
- [Redis commands reference](https://redis.io/commands/)

### Testing in Go
- [Testing package docs](https://pkg.go.dev/testing)
- [Advanced Go testing](https://quii.gitbook.io/learn-go-with-tests/)

---

## Getting Help

- **Documentation**: See `README.md`, `TODO.md`, `DEVELOPMENT_PLAN.md`
- **Code Comments**: Most files have TODO comments explaining what needs to be implemented
- **Issues**: Check GitHub Issues (or create your own tracking system)

---

## Next: Your First Contribution

Ready to start coding? Pick a task from `TODO.md` Phase 1 and:

1. Create a feature branch: `git checkout -b feature/database-schema`
2. Implement the feature with tests
3. Run tests: `go test ./...`
4. Commit changes: `git commit -m "Add PostgreSQL schema and migrations"`
5. Push and create PR (if working with a team)

**Recommended Starting Point**: Database schema (see TODO.md section 1.1)

Good luck building ThinkPixelLLMGW! 🚀
