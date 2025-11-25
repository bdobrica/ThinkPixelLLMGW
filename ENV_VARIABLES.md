# Environment Variables Configuration

This file documents all environment variables used by the LLM Gateway.

## Required Variables

### Database Connection
```bash
# PostgreSQL connection string (REQUIRED)
# Format: postgres://user:password@host:port/database?sslmode=disable
DATABASE_URL=postgres://postgres:password@localhost:5432/llmgateway?sslmode=disable
```

## Optional Variables

### HTTP Server
```bash
# HTTP server port (default: 8080)
GATEWAY_HTTP_PORT=8080
```

### Database Connection Pool
```bash
# Maximum number of open connections to the database (default: 25)
DB_MAX_OPEN_CONNS=25

# Maximum number of idle connections in the pool (default: 5)
DB_MAX_IDLE_CONNS=5

# Maximum lifetime of a connection (default: 5m)
# Valid units: ns, us, ms, s, m, h
DB_CONN_MAX_LIFETIME=5m

# Maximum time a connection can be idle (default: 1m)
DB_CONN_MAX_IDLE_TIME=1m
```

### Cache Configuration

#### API Key Cache
```bash
# Number of API keys to cache (default: 1000)
# Recommendation: Set to 2-3x your active API key count
CACHE_API_KEY_SIZE=1000

# Time-to-live for cached API keys (default: 5m)
# Lower values = fresher data, higher DB load
# Higher values = better performance, staler data
CACHE_API_KEY_TTL=5m
```

#### Model Cache
```bash
# Number of models to cache (default: 500)
# Recommendation: Set to 2x your active model count
CACHE_MODEL_SIZE=500

# Time-to-live for cached models (default: 15m)
# Models change less frequently than API keys, so longer TTL is reasonable
CACHE_MODEL_TTL=15m
```

### Redis Configuration

#### Connection Settings
```bash
# Redis server address (default: localhost:6379)
REDIS_ADDRESS=localhost:6379

# Redis password (default: empty/no auth)
REDIS_PASSWORD=

# Redis database number 0-15 (default: 0)
REDIS_DB=0
```

#### Pool Settings
```bash
# Maximum number of socket connections (default: 10)
REDIS_POOL_SIZE=10

# Minimum number of idle connections (default: 2)
REDIS_MIN_IDLE_CONNS=2
```

#### Timeout Settings
```bash
# Dial timeout for establishing new connections (default: 5s)
REDIS_DIAL_TIMEOUT=5s

# Timeout for socket reads (default: 3s)
REDIS_READ_TIMEOUT=3s

# Timeout for socket writes (default: 3s)
REDIS_WRITE_TIMEOUT=3s
```

### Provider Configuration
```bash
# How often to reload providers from database (default: 5m)
# Set to 0 to disable auto-reload (manual reload via API only)
PROVIDER_RELOAD_INTERVAL=5m

# Default timeout for provider requests (default: 60s)
# This is the maximum time to wait for a provider response
PROVIDER_REQUEST_TIMEOUT=60s
```

## Example Configurations

### Development Environment
```bash
# .env.development
GATEWAY_HTTP_PORT=8080
DATABASE_URL=postgres://postgres:devpass@localhost:5432/llmgateway_dev?sslmode=disable

# Smaller pool for development
DB_MAX_OPEN_CONNS=10
DB_MAX_IDLE_CONNS=2

# Smaller cache for development
CACHE_API_KEY_SIZE=100
CACHE_API_KEY_TTL=1m
CACHE_MODEL_SIZE=50
CACHE_MODEL_TTL=5m

# Redis (local)
REDIS_ADDRESS=localhost:6379
REDIS_PASSWORD=
REDIS_POOL_SIZE=5
REDIS_MIN_IDLE_CONNS=1

# Provider settings
PROVIDER_RELOAD_INTERVAL=1m
PROVIDER_REQUEST_TIMEOUT=30s
```

### Production Environment
```bash
# .env.production
GATEWAY_HTTP_PORT=8080
DATABASE_URL=postgres://llmgateway:secure_password@db.example.com:5432/llmgateway?sslmode=require

# Larger pool for production load
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=10m
DB_CONN_MAX_IDLE_TIME=2m

# Larger cache for production
CACHE_API_KEY_SIZE=2000
CACHE_API_KEY_TTL=5m
CACHE_MODEL_SIZE=1000
CACHE_MODEL_TTL=15m

# Redis (production)
REDIS_ADDRESS=redis.example.com:6379
REDIS_PASSWORD=secure_redis_password
REDIS_POOL_SIZE=20
REDIS_MIN_IDLE_CONNS=5
REDIS_DIAL_TIMEOUT=5s
REDIS_READ_TIMEOUT=3s
REDIS_WRITE_TIMEOUT=3s

# Provider settings
PROVIDER_RELOAD_INTERVAL=5m
PROVIDER_REQUEST_TIMEOUT=60s
```

### High-Traffic Environment
```bash
# .env.high-traffic
GATEWAY_HTTP_PORT=8080
DATABASE_URL=postgres://llmgateway:secure_password@db.example.com:5432/llmgateway?sslmode=require

# Maximum pool size
DB_MAX_OPEN_CONNS=100
DB_MAX_IDLE_CONNS=25
DB_CONN_MAX_LIFETIME=15m
DB_CONN_MAX_IDLE_TIME=5m

# Large cache to minimize DB queries
CACHE_API_KEY_SIZE=5000
CACHE_API_KEY_TTL=10m
CACHE_MODEL_SIZE=2000
CACHE_MODEL_TTL=30m
```

## Tuning Guidelines

### Cache Size Tuning

**API Key Cache Size**:
- Monitor cache statistics (hit rate, evictions)
- Target: > 95% hit rate
- If evictions are high, increase `CACHE_API_KEY_SIZE`
- Formula: `CACHE_API_KEY_SIZE = active_keys * 2`

**Model Cache Size**:
- Models are relatively static
- Target: Cache all active models
- Formula: `CACHE_MODEL_SIZE = total_models * 1.5`

### Cache TTL Tuning

**API Key TTL**:
- Lower values (1-5m): Faster propagation of changes (enable/disable, rate limits)
- Higher values (10-15m): Better performance, less DB load
- Recommendation: Start with 5m, adjust based on change frequency

**Model TTL**:
- Models change infrequently (pricing updates, new models)
- Recommendation: 15-30m is safe for most workloads
- Can be increased to 1h+ if model changes are rare

### Connection Pool Tuning

**Max Open Connections**:
- Set based on database capacity and concurrent request volume
- PostgreSQL default: 100 connections
- Formula: `DB_MAX_OPEN_CONNS = available_db_connections / num_app_instances`
- Example: 100 DB connections, 4 app instances → 25 connections per instance

**Max Idle Connections**:
- Keep connections ready for bursts
- Typical ratio: 20% of max open connections
- Example: `DB_MAX_OPEN_CONNS=25` → `DB_MAX_IDLE_CONNS=5`

### Connection Lifetime

**Max Lifetime**:
- Recycle connections to avoid long-lived connections
- Helps with database connection management
- Typical: 5-15 minutes

**Max Idle Time**:
- Close connections that haven't been used
- Saves database resources during low traffic
- Typical: 1-5 minutes

## Monitoring

To verify your configuration is optimal, monitor these metrics:

### Cache Metrics
```bash
# Check cache hit rate (target: > 95%)
curl http://localhost:8080/health

# Look for:
# - api_key_cache_hit_rate
# - model_cache_hit_rate
# - api_key_cache_evictions (should be low)
# - model_cache_evictions (should be low)
```

### Database Metrics
```bash
# Check connection pool usage
# Look for:
# - open_connections (should be < max_open_conns)
# - wait_count (should be low)
# - max_idle_closed (indicates idle timeout is working)
```

## Security Notes

1. **DATABASE_URL**: Contains credentials - never commit to version control
2. **SSL Mode**: Use `sslmode=require` in production
3. **Credentials**: Use strong passwords (20+ characters)
4. **Environment**: Use `.env` files or secrets management (AWS Secrets Manager, HashiCorp Vault)

## Docker Example

```yaml
# docker-compose.yml
version: '3.8'
services:
  gateway:
    image: llmgateway:latest
    environment:
      - GATEWAY_HTTP_PORT=8080
      - DATABASE_URL=postgres://postgres:password@db:5432/llmgateway?sslmode=disable
      - DB_MAX_OPEN_CONNS=25
      - DB_MAX_IDLE_CONNS=5
      - CACHE_API_KEY_SIZE=1000
      - CACHE_API_KEY_TTL=5m
      - CACHE_MODEL_SIZE=500
      - CACHE_MODEL_TTL=15m
    ports:
      - "8080:8080"
    depends_on:
      - db
  
  db:
    image: postgres:16
    environment:
      - POSTGRES_DB=llmgateway
      - POSTGRES_PASSWORD=password
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

## Kubernetes Example

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: llmgateway-config
data:
  GATEWAY_HTTP_PORT: "8080"
  DB_MAX_OPEN_CONNS: "50"
  DB_MAX_IDLE_CONNS: "10"
  CACHE_API_KEY_SIZE: "2000"
  CACHE_API_KEY_TTL: "5m"
  CACHE_MODEL_SIZE: "1000"
  CACHE_MODEL_TTL: "15m"

---
# secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: llmgateway-secret
type: Opaque
stringData:
  DATABASE_URL: postgres://user:password@postgres.default.svc.cluster.local:5432/llmgateway?sslmode=require
```

## Troubleshooting

### Issue: High cache miss rate
**Solution**: Increase `CACHE_API_KEY_SIZE` or `CACHE_MODEL_SIZE`

### Issue: Stale data in cache
**Solution**: Decrease `CACHE_API_KEY_TTL` or `CACHE_MODEL_TTL`

### Issue: Database connection pool exhausted
**Solution**: Increase `DB_MAX_OPEN_CONNS` or decrease request rate

### Issue: Too many idle connections
**Solution**: Decrease `DB_MAX_IDLE_CONNS` or `DB_CONN_MAX_IDLE_TIME`

### Issue: High memory usage
**Solution**: Decrease cache sizes or implement cache eviction policies
