# init-admin - Bootstrap Admin User Tool

This tool creates the initial admin user for the LLM Gateway's admin API.

## Purpose

Before you can use the admin API endpoints, you need at least one admin user. This tool creates the bootstrap admin user by connecting directly to the database and using Argon2 password hashing.

## Features

- ✅ **Idempotent**: Safe to run multiple times - checks if admins exist first
- ✅ **Secure**: Uses Argon2id password hashing
- ✅ **Validates Input**: Checks email format and password strength
- ✅ **Kubernetes-Ready**: Designed to run as a Kubernetes Job
- ✅ **Clear Feedback**: Provides detailed output about what it's doing

## Usage

### Environment Variables

| Variable | Description | Required | Example |
|----------|-------------|----------|---------|
| `ADMIN_BOOTSTRAP_EMAIL` | Email for bootstrap admin | Yes | `admin@example.com` |
| `ADMIN_BOOTSTRAP_PASSWORD` | Password (min 8 chars) | Yes | `SecurePass123!` |
| `DATABASE_URL` | PostgreSQL connection string | Yes | `postgres://user:pass@host:5432/db` |
| `JWT_SECRET` | JWT signing secret required by the shared config loader (min 32 chars) | Yes | generated secret |
| `METRICS_AUTH_TOKEN` | Metrics bearer token required by the shared config loader (min 32 chars) | Yes | generated token |
| `ENCRYPTION_KEY` | AES key required by the shared config loader (64 hexadecimal chars) | Yes | generated key |

### Running Locally

```bash
# Build the binary
go build -o init-admin ./cmd/init-admin

# Set environment variables
export ADMIN_BOOTSTRAP_EMAIL="admin@localhost"
export ADMIN_BOOTSTRAP_PASSWORD="devpassword123"
export DATABASE_URL="postgres://llmgateway:password@localhost:5432/llmgateway?sslmode=disable"
export JWT_SECRET="0123456789abcdef0123456789abcdef"
export METRICS_AUTH_TOKEN="abcdef0123456789abcdef0123456789"
export ENCRYPTION_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

# Run the tool
./init-admin
```

### Running in Docker

```bash
docker run --rm \
  -e ADMIN_BOOTSTRAP_EMAIL=admin@example.com \
  -e ADMIN_BOOTSTRAP_PASSWORD=your-secure-password \
  -e DATABASE_URL=postgres://user:pass@host:5432/db \
  -e JWT_SECRET=0123456789abcdef0123456789abcdef \
  -e METRICS_AUTH_TOKEN=abcdef0123456789abcdef0123456789 \
  -e ENCRYPTION_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef \
  your-registry/llm-gateway:latest \
  /app/init-admin
```

### Running in Kubernetes

See [`docs/bootstrap-admin.md`](../../../docs/bootstrap-admin.md) for the required bootstrap, database, and gateway Secrets and the repository-root `k8s-init-admin-job.yaml` example.

## Behavior

### First Run (No Admins Exist)

```
LLM Gateway - Bootstrap Admin Initialization
================================================
Connecting to database...
Database connection established
Checking for existing admin users...
Creating bootstrap admin user: admin@example.com

==================================================
SUCCESS: Bootstrap admin user created successfully!
==================================================
Email: admin@example.com
ID: 12345678-1234-1234-1234-123456789abc
Roles: [admin]
Created: 2025-12-15T10:30:00Z

You can now log in to the admin panel with these credentials.
IMPORTANT: Store these credentials securely; self-service password changes are not implemented.

For security, you should now:
1. Remove ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_PASSWORD from your environment
2. Verify login and restrict the Admin API to trusted networks
3. Use an approved, audited database procedure if bootstrap access must be recovered
```

### Subsequent Runs (Admins Already Exist)

```
LLM Gateway - Bootstrap Admin Initialization
================================================
Connecting to database...
Database connection established
Checking for existing admin users...
INFO: Found 2 existing admin user(s). Bootstrap not needed.
Existing users:
  - admin@example.com (enabled) - Roles: [admin]
  - user@example.com (enabled) - Roles: [admin, viewer]

Exiting successfully (no action taken)
```

## Error Handling

The tool exits with code 1 and displays an error message for:

- **Missing environment variables**: `ADMIN_BOOTSTRAP_EMAIL` or `ADMIN_BOOTSTRAP_PASSWORD` not set
- **Invalid email format**: Email doesn't contain `@` or is malformed
- **Weak password**: Password is less than 8 characters
- **Database connection failure**: Cannot connect to PostgreSQL
- **Database query errors**: Issues reading or writing to the database

## Security Notes

1. **Password Hashing**: Uses Argon2id with these parameters:
   - Time cost: 1
   - Memory: 64MB
   - Threads: 4
   - Output length: 32 bytes

2. **Credentials Storage**: 
   - Passwords are never stored in plaintext
   - Only the Argon2 hash is persisted to the database
   - Environment variables should be cleared after use

3. **Admin Role**: Bootstrap admin gets the `admin` role with full privileges

## Integration with Gateway

After running this tool:

1. **Start the gateway**: The gateway pods can now start normally
2. **Login via API**: Use `/admin/auth/login` with bootstrap credentials
3. **Manage gateway resources**: Use the implemented key, provider, model, and alias endpoints
4. **Plan administrator lifecycle**: User/service-token management endpoints are not implemented; use an approved audited recovery procedure if bootstrap access is lost

## Development

### Building

```bash
go build -o init-admin ./cmd/init-admin
```

### Testing

```bash
# Start a test database
docker run --rm -d \
  -p 5433:5432 \
  -e POSTGRES_DB=testdb \
  -e POSTGRES_USER=test \
  -e POSTGRES_PASSWORD=test \
  --name test-postgres \
  postgres:15-alpine

# Run migrations
# (use your migration tool here)

# Test the tool
export ADMIN_BOOTSTRAP_EMAIL="test@example.com"
export ADMIN_BOOTSTRAP_PASSWORD="testpass123"
export DATABASE_URL="postgres://test:test@localhost:5433/testdb?sslmode=disable"
export JWT_SECRET="0123456789abcdef0123456789abcdef"
export METRICS_AUTH_TOKEN="abcdef0123456789abcdef0123456789"
export ENCRYPTION_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

./init-admin

# Clean up
docker stop test-postgres
```

## Troubleshooting

### "Invalid email format"

Ensure your email:
- Contains exactly one `@` symbol
- Has at least one character before and after `@`
- Is at least 3 characters total

### "Password must be at least 8 characters long"

Use a password with 8 or more characters. Consider using:
- Mixed case letters
- Numbers
- Special characters

### "Failed to connect to database"

Check:
- `DATABASE_URL` is correctly formatted
- Database host is reachable
- Credentials are correct
- Database exists
- Migrations have been run

### "Failed to hash password"

This is a rare error indicating a problem with the cryptographic random number generator. Check:
- System entropy (/dev/urandom on Linux)
- File system permissions

## Related Documentation

- `BOOTSTRAP_ADMIN.md` - Complete guide for Kubernetes deployments
- `k8s-init-admin-job.yaml` - Kubernetes Job example
- `DATABASE_SCHEMA.md` - Database schema including admin_users table
