#!/bin/bash

# Generate the AES-256 key required by the gateway's ENCRYPTION_KEY setting.
# Usage: ./generate-encryption-key.sh [32]

KEY_SIZE=${1:-32}

if [ "$KEY_SIZE" != "32" ]; then
    echo "Error: the gateway requires a 32-byte AES-256 key"
    echo "Usage: $0 [32]"
    exit 1
fi

# Gateway configuration decodes ENCRYPTION_KEY as hexadecimal, so 32 random
# bytes must be represented by exactly 64 hexadecimal characters.
ENCRYPTION_KEY=$(openssl rand -hex "$KEY_SIZE")

echo "Generated AES-256 encryption key (64 hexadecimal characters):"
echo ""
echo "$ENCRYPTION_KEY"
echo ""
echo "Add this to your environment variables or .env file:"
echo "ENCRYPTION_KEY=$ENCRYPTION_KEY"
echo ""
