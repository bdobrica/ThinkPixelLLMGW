# Bootstrap administrator

The `init-admin` command creates the first human administrator directly in PostgreSQL. Run it once before exposing the Admin API.

## Behavior and limitations

- The command creates one enabled user with the `admin` role and an Argon2id password hash.
- It is idempotent only in the bootstrap sense: if any administrator already exists, it exits successfully without creating or changing a user.
- It never prints or stores the plaintext password.
- The current Admin API has no `/admin/users` or `/admin/tokens` management endpoints. Additional administrators, password changes, disablement, and service-token provisioning are not currently self-service operations.

Because the command uses the gateway’s shared configuration loader, it currently requires all four gateway secrets/configuration values even though it only connects to PostgreSQL:

| Variable | Requirement |
|---|---|
| `ADMIN_BOOTSTRAP_EMAIL` | Initial administrator email |
| `ADMIN_BOOTSTRAP_PASSWORD` | Password of at least 8 characters; use 12 or more in practice |
| `DATABASE_URL` | PostgreSQL connection string |
| `JWT_SECRET` | At least 32 characters |
| `METRICS_AUTH_TOKEN` | At least 32 characters |
| `ENCRYPTION_KEY` | Exactly 64 hexadecimal characters |

## Docker Compose

First configure and start the stack as described in [Quick start](quickstart.md). Then run:

```bash
docker compose run --rm \
  -e ADMIN_BOOTSTRAP_EMAIL=admin@example.com \
  -e ADMIN_BOOTSTRAP_PASSWORD='replace-with-a-strong-password' \
  --entrypoint /app/init-admin \
  gateway
```

The gateway service already receives `DATABASE_URL`, `JWT_SECRET`, `METRICS_AUTH_TOKEN`, and `ENCRYPTION_KEY` from Compose and `.env`.

Verify login:

```bash
curl --fail-with-body http://localhost:8080/admin/auth/login \
  -H 'Content-Type: application/json' \
  --data '{"email":"admin@example.com","password":"replace-with-a-strong-password"}'
```

The response contains an admin JWT in `token`. Validate it with:

```bash
curl --fail-with-body http://localhost:8080/admin/me \
  -H 'Authorization: Bearer <token>'
```

## Run locally

With PostgreSQL and all migrations already available:

```bash
cd llm_gateway
go build -o bin/init-admin ./cmd/init-admin

export ADMIN_BOOTSTRAP_EMAIL='admin@example.com'
export ADMIN_BOOTSTRAP_PASSWORD='replace-with-a-strong-password'
export DATABASE_URL='postgres://gateway:password@localhost:5432/gateway?sslmode=disable'
export JWT_SECRET='0123456789abcdef0123456789abcdef'
export METRICS_AUTH_TOKEN='abcdef0123456789abcdef0123456789'
export ENCRYPTION_KEY='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'

./bin/init-admin
```

Generate unique secrets rather than using these deterministic examples.

## Kubernetes Job

The repository-root `k8s-init-admin-job.yaml` is a hardened Job example. It expects three existing Secrets:

```bash
kubectl create secret generic llm-gateway-bootstrap \
  --from-literal=email=admin@example.com \
  --from-literal=password='replace-with-a-strong-password'

kubectl create secret generic llm-gateway-db \
  --from-literal=url='postgres://gateway:password@postgres.default.svc.cluster.local:5432/gateway?sslmode=require'

kubectl create secret generic llm-gateway-secrets \
  --from-literal=jwt-secret='generate-at-least-32-random-characters' \
  --from-literal=metrics-auth-token='generate-a-different-32-character-token' \
  --from-literal=encryption-key='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
```

Replace every example value. Prefer your cluster’s approved external secret manager instead of literal shell arguments, which may be retained in shell history.

Pin the Job image to the qualified immutable digest, set the namespace, and apply it:

```bash
kubectl apply -f k8s-init-admin-job.yaml
kubectl wait --for=condition=complete --timeout=120s job/llm-gateway-init-admin
kubectl logs job/llm-gateway-init-admin
```

Deploy gateway workloads only after successful completion. The repository does not provide a production gateway Deployment, Service, or Ingress.

After verifying login, delete the short-lived bootstrap credential and completed Job:

```bash
kubectl delete secret llm-gateway-bootstrap
kubectl delete job llm-gateway-init-admin
```

Retain the database and gateway application secrets. The Job has `automountServiceAccountToken: false`, runs as a non-root user, drops Linux capabilities, uses a read-only root filesystem, and sets the default seccomp profile. Adapt resource limits and policy labels to the target cluster.

## Re-running and recovery

To run the Job again, delete the completed Job object and reapply the manifest. If an administrator row already exists—including a disabled one—the command intentionally creates nothing.

If bootstrap credentials are lost after creation, do not delete production database rows casually or inject a known hash. Restore access through an approved, audited database-administration procedure or implement the missing administrator-management capability. Record the action and rotate affected credentials.

## Troubleshooting

### Configuration validation fails

Errors naming `JWT_SECRET`, `METRICS_AUTH_TOKEN`, or `ENCRYPTION_KEY` occur before the database connection. Confirm the referenced Secret keys exist and meet the length/format rules.

### Database connection fails

Check DNS, network policy, TLS parameters, credentials, and that all migrations have run. Inspect only Secret metadata during routine diagnosis; avoid printing decoded secrets into terminals or tickets.

### No administrator is created

Read the Job output. A successful message reporting existing users means bootstrap was skipped by design. Query `admin_users` through a restricted database identity if ownership/status must be confirmed.

### Workload starts before bootstrap

A standalone Job does not automatically order a Deployment. Gate rollout in CI/CD or Helm on Job success. Do not add a long-running init container that repeatedly handles the bootstrap password.

For API-key, provider, model, and alias administration after login, see the endpoint table in the root [README](../README.md) and [Provider configuration](providers.md).
