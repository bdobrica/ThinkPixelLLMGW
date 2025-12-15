# Bootstrap Admin Setup Guide

This guide explains how to create the initial admin user for the LLM Gateway's admin API.

## Overview

The LLM Gateway uses admin users (stored in `admin_users` table) and service accounts (stored in `admin_tokens` table) for administrative access. These use Argon2 password hashing and JWT authentication.

Before you can use the admin API endpoints, you need to create at least one bootstrap admin user. This guide provides a secure method for doing so in Kubernetes deployments.

## Method: Kubernetes Init Job (Recommended)

This method uses a one-time Kubernetes Job that runs before the gateway pods start. This approach:

- ✅ Prevents race conditions in multi-pod deployments
- ✅ Keeps credentials isolated from long-running gateway pods
- ✅ Follows standard Kubernetes initialization patterns
- ✅ Is idempotent (can be safely re-run)

### Prerequisites

- Kubernetes cluster with `kubectl` configured
- Docker image built with both `gateway` and `init-admin` binaries
- Database accessible from Kubernetes cluster
- Namespace created (we'll use `default` in examples)

### Step 1: Create Bootstrap Credentials Secret

Create a Kubernetes Secret containing the bootstrap admin credentials:

```bash
kubectl create secret generic llm-gateway-bootstrap \
  --from-literal=email=admin@example.com \
  --from-literal=password=your-secure-password \
  --namespace=default
```

**Security Notes:**
- Use a strong password (minimum 8 characters)
- Store the password in a secure password manager
- You can delete this secret after the bootstrap user is created

### Step 2: Deploy the Init Job

Apply the init job configuration:

```bash
kubectl apply -f k8s-init-admin-job.yaml
```

The job will:
1. Connect to the database
2. Check if any admin users already exist
3. If none exist, create the bootstrap admin user
4. Exit successfully

### Step 3: Verify Job Completion

Check the job status:

```bash
kubectl get jobs llm-gateway-init-admin -n default
```

View the job logs:

```bash
kubectl logs job/llm-gateway-init-admin -n default
```

Expected output on first run:
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
```

If admin users already exist:
```
INFO: Found 1 existing admin user(s). Bootstrap not needed.
Existing users:
  - admin@example.com (enabled) - Roles: [admin]

Exiting successfully (no action taken)
```

### Step 4: Deploy Gateway Pods

Once the init job completes successfully, deploy your gateway:

```bash
kubectl apply -f gateway-deployment.yaml
```

### Step 5: Log In and Create Additional Admins

1. Use the bootstrap credentials to authenticate:

```bash
curl -X POST http://gateway-service/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "your-secure-password"
  }'
```

2. Save the returned JWT token

3. Create additional admin users through the API:

```bash
curl -X POST http://gateway-service/admin/users \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "another-admin@example.com",
    "password": "another-secure-password",
    "roles": ["admin"]
  }'
```

### Step 6: Security Cleanup (Optional)

After creating additional admin users, enhance security:

1. **Delete the bootstrap secret:**
```bash
kubectl delete secret llm-gateway-bootstrap -n default
```

2. **Consider rotating the bootstrap admin:**
   - Create a new admin with a different email
   - Log in with the new admin
   - Disable or delete the original bootstrap admin through the API

3. **Delete the completed job:**
```bash
kubectl delete job llm-gateway-init-admin -n default
```

## Kubernetes Configuration Reference

### Full k8s-init-admin-job.yaml

See the provided `k8s-init-admin-job.yaml` file in the repository root for a complete example including:

- Secret definition (with base64-encoded example values)
- Job definition with proper security context
- Example Deployment configurations
- Optional initContainer for job synchronization

### Required Environment Variables

The init-admin tool requires:

| Variable | Description | Required |
|----------|-------------|----------|
| `ADMIN_BOOTSTRAP_EMAIL` | Email for bootstrap admin | Yes |
| `ADMIN_BOOTSTRAP_PASSWORD` | Password for bootstrap admin (min 8 chars) | Yes |
| `DATABASE_URL` | PostgreSQL connection string | Yes |
| `JWT_SECRET` | JWT signing secret (loaded by config, not used) | No* |

*JWT_SECRET is loaded by the config package but not actually used by init-admin

### Database Connection String Format

```
postgres://username:password@host:port/database?sslmode=disable
```

Example for Cloud SQL:
```
postgres://gateway:password@10.0.0.5:5432/llmgateway?sslmode=require
```

## Re-running the Init Job

The init job is **idempotent**. It's safe to run multiple times:

- If admin users exist, it exits successfully without making changes
- If the specific email already exists, it skips creation
- You can use it to bootstrap different environments (dev, staging, prod)

To re-run:

```bash
kubectl delete job llm-gateway-init-admin -n default
kubectl apply -f k8s-init-admin-job.yaml
```

## Troubleshooting

### Job Fails to Connect to Database

Check database connectivity:
```bash
kubectl run -it --rm debug --image=postgres:15-alpine --restart=Never -- \
  psql "$DATABASE_URL"
```

Verify the `DATABASE_URL` secret is correct:
```bash
kubectl get secret llm-gateway-db -n default -o yaml
```

### "Invalid Credentials" Error

Verify secret values:
```bash
kubectl get secret llm-gateway-bootstrap -n default -o jsonpath='{.data.email}' | base64 -d
```

### Job Completes but No Admin Created

Check job logs for details:
```bash
kubectl logs job/llm-gateway-init-admin -n default
```

Common issues:
- Database connection failed
- Email format invalid
- Password too short (< 8 characters)
- Admin users already exist

### Pod Starts Before Init Job Completes

Use an initContainer or Helm hooks to ensure proper ordering. See the example in `k8s-init-admin-job.yaml`.

## Local Development / Docker Compose

For local development, you can run the init-admin tool directly:

```bash
# Build the binary
cd llm_gateway
go build -o init-admin ./cmd/init-admin

# Set environment variables
export ADMIN_BOOTSTRAP_EMAIL="admin@localhost"
export ADMIN_BOOTSTRAP_PASSWORD="devpassword123"
export DATABASE_URL="postgres://llmgateway:llmgateway_dev_password@localhost:5432/llmgateway?sslmode=disable"
export JWT_SECRET="dev-secret"

# Run the tool
./init-admin
```

Or using Docker Compose:

```yaml
services:
  init-admin:
    image: your-registry/llm-gateway:latest
    command: ["/app/init-admin"]
    environment:
      ADMIN_BOOTSTRAP_EMAIL: admin@localhost
      ADMIN_BOOTSTRAP_PASSWORD: devpassword123
      DATABASE_URL: postgres://llmgateway:llmgateway_dev_password@postgres:5432/llmgateway?sslmode=disable
      JWT_SECRET: dev-secret
    depends_on:
      - postgres
```

Run once:
```bash
docker-compose run --rm init-admin
```

## Security Best Practices

1. **Strong Passwords**: Use passwords with at least 12 characters, including mixed case, numbers, and symbols

2. **Secret Management**: Consider using external secret managers:
   - AWS Secrets Manager
   - Azure Key Vault
   - HashiCorp Vault
   - Sealed Secrets for Kubernetes

3. **Rotate Credentials**: Change the bootstrap admin password after initial setup

4. **Principle of Least Privilege**: Create service accounts with limited roles for automated tools

5. **Audit Logging**: Monitor admin API access through your gateway logs

6. **Delete Bootstrap Resources**: Remove the bootstrap secret and job after successful initialization

## Next Steps

After creating your bootstrap admin:

1. **Access the Admin API**: See `API_DOCUMENTATION.md` for available endpoints
2. **Create API Keys**: Use `/admin/keys` endpoints to create client API keys
3. **Configure Providers**: Set up LLM provider credentials via `/admin/providers`
4. **Set Up Models**: Configure model aliases and routing via `/admin/models`
5. **Create Service Accounts**: For automated tools, create admin tokens via `/admin/tokens`

For more information, see:
- `DATABASE_SCHEMA.md` - Database schema including admin tables
- `ENV_VARIABLES.md` - All available environment variables
- `QUICKSTART.md` - General gateway setup guide
