# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /workspace

# Install dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
ARG VERSION=0.1.0
ARG BUILD_DATE
ARG GIT_COMMIT
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE} -X main.GitCommit=${GIT_COMMIT}" -o aggregator ./cmd/aggregator

# Runtime stage
FROM alpine:3.20

WORKDIR /

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary
COPY --from=builder /workspace/aggregator /aggregator

# Expose gRPC port
EXPOSE 50051

# Expose metrics port
EXPOSE 8080

# Run as non-root user
RUN adduser -D -g '' k8swatch
USER k8swatch

ENTRYPOINT ["/aggregator"]
