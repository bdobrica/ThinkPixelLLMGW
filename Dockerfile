# Build stage
FROM golang:1.24-alpine@sha256:8bee1901f1e530bfb4a7850aa7a479d17ae3a18beb6e09064ed54cfd245b7191 AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY llm_gateway/go.mod llm_gateway/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY llm_gateway/ ./

# Build static binaries (gateway and init-admin)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /build/gateway \
    ./cmd/gateway

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /build/init-admin \
    ./cmd/init-admin

# Runtime stage
FROM alpine:3.22@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 gateway && \
    adduser -D -u 1000 -G gateway gateway

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/gateway /app/gateway
COPY --from=builder /build/init-admin /app/init-admin

# Copy migrations (if needed for runtime)
COPY --from=builder /build/internal/storage/migrations /app/migrations

# Create log directory with proper permissions
RUN mkdir -p /var/log/llm-gateway && \
    chown -R gateway:gateway /app /var/log/llm-gateway

# Switch to non-root user
USER gateway

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ready || exit 1

# Run the binary
ENTRYPOINT ["/app/gateway"]
