# Examples

This directory contains example programs demonstrating various features of the LLM Gateway.

## Running Examples

Each example is a standalone program. Run them individually:

### S3 Logging Example

Demonstrates how to use the S3-based logging sink to buffer and flush request logs to S3.

**Prerequisites:**
- AWS credentials configured (via `~/.aws/credentials`, IAM role, or environment variables)
- S3 bucket created
- Required environment variables set

**Environment Variables:**
```bash
export LOGGING_SINK_ENABLED=true
export LOGGING_SINK_S3_BUCKET=my-test-bucket
export LOGGING_SINK_S3_REGION=us-east-1
export LOGGING_SINK_S3_PREFIX=logs/
export LOGGING_SINK_BUFFER_SIZE=10000
export LOGGING_SINK_FLUSH_SIZE=100
export LOGGING_SINK_FLUSH_INTERVAL=5m
export POD_NAME=demo-pod

# Also required (can use minimal values for testing)
export DATABASE_URL=postgres://user:pass@localhost/dbname
```

**Run:**
```bash
go run examples/s3_logging_example.go
```

**What it does:**
1. Creates an S3 sink with configured buffer and flush settings
2. Enqueues 10 sample log records
3. Demonstrates automatic flushing based on size/time
4. Shows graceful shutdown with buffer flush

### Encryption Example

Demonstrates field-level encryption for sensitive data.

**Run:**
```bash
go run examples/encryption_example.go
```

## Notes

- Each example can be run independently
- The compile errors you might see in the IDE are expected (multiple `main()` functions)
- In production, you would typically use only one of these patterns in your main application
