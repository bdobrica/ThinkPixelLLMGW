#!/bin/bash

# Script to generate encryption keys for provider credentials
# Usage: ./generate-encryption-key.sh [key-size]
# Key size can be 16 (AES-128), 24 (AES-192), or 32 (AES-256, default)

KEY_SIZE=${1:-32}

if [ "$KEY_SIZE" != "16" ] && [ "$KEY_SIZE" != "24" ] && [ "$KEY_SIZE" != "32" ]; then
    echo "Error: Key size must be 16, 24, or 32 bytes"
    echo "Usage: $0 [16|24|32]"
    exit 1
fi

# Generate random bytes and encode as base64
ENCRYPTION_KEY=$(openssl rand -base64 $KEY_SIZE | tr -d '\n')

echo "Generated AES-$((KEY_SIZE * 8)) encryption key:"
echo ""
echo "$ENCRYPTION_KEY"
echo ""
echo "Add this to your environment variables or .env file:"
echo "ENCRYPTION_KEY=$ENCRYPTION_KEY"
echo ""
echo "Or use with Go code:"
echo ""
echo "  encryption, err := storage.NewEncryptionFromBase64(\"$ENCRYPTION_KEY\")"
