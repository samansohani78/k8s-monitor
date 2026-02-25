#!/bin/bash
# Script to generate Go code from protobuf definitions
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc

set -e

PROTO_DIR="proto"
OUT_DIR="internal/pb"

# Check for required tools
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed. Install protobuf-compiler."
    echo "  Ubuntu: apt-get install protobuf-compiler"
    echo "  macOS:  brew install protobuf"
    exit 1
fi

if ! command -v protoc-gen-go &> /dev/null; then
    echo "Error: protoc-gen-go is not installed."
    echo "  Run: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    exit 1
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Error: protoc-gen-go-grpc is not installed."
    echo "  Run: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
    exit 1
fi

# Create output directory
mkdir -p "$OUT_DIR"

# Generate Go code
echo "Generating Go code from protobuf definitions..."
protoc \
    --go_out="$OUT_DIR" \
    --go_opt=paths=source_relative \
    --go-grpc_out="$OUT_DIR" \
    --go-grpc_opt=paths=source_relative \
    "$PROTO_DIR"/*.proto

echo "Go code generated successfully in $OUT_DIR"
