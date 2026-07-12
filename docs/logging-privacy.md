# Audit Logging Privacy and Retention

The gateway emits rotating local request metadata and optional operational records buffered in Redis and archived to S3. By default, known credential headers are omitted, credential query parameters are redacted, local request bodies contain only SHA-256 and byte count, and S3 request/response payloads are omitted. Provider, model, timing, cost, status, and request identifiers remain available.

## Body policy

`AUDIT_BODY_MODE` applies to both streams: `none` omits bodies; `hash` (default) hashes local bodies and omits S3 payloads; `redacted` retains JSON up to `AUDIT_MAX_BODY_BYTES` after recursively replacing common credential fields plus `AUDIT_SENSITIVE_FIELDS`. Oversized or non-JSON content falls back to hash/size. `AUDIT_SAMPLE_RATE` accepts `0` through `1`; do not sample when regulations require a complete trail.

Credential-like `Bearer ...` and `sk-...` strings are removed from retained header/error text. Prompts can still contain personal or confidential prose in fields operators did not configure. Use `none` or `hash` unless content retention has a documented purpose and lawful basis.

## Storage, access, and deletion

- Local files are `0640` in directories created as `0750`; `REQUEST_LOGGER_MAX_FILES` bounds rotation. Restrict the owning group and securely delete retired volumes.
- Redis is a transient queue capped by `LOGGING_SINK_BUFFER_SIZE`; records are removed after successful S3 upload. Protect Redis and separately expire its backups/persistence files.
- S3 objects use gzip and server-side AES-256 encryption. Configure bucket lifecycle expiration (including non-current versions), least-privilege gateway write access, separate read/delete roles, access logging, and appropriate regional placement.

Retention duration is deployment-specific. Record it alongside lifecycle configuration, test deletion, cover replicas/backups, and document how deletion requests map from request/API-key identifiers. Changing the prefix does not delete old objects. Before enabling `redacted`, review regional transfer rules, tenant contracts, employee access, incident response, and provider terms.
