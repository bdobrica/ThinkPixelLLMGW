# Build stage
FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS builder

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
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

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
