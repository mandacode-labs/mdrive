#syntax=docker/dockerfile:1.4

# Build arguments
ARG VERSION=0.0.1
ARG GIT_COMMIT=unknown

############################
# 1. Build Stage
############################
FROM golang:1.26.1-alpine3.23 AS builder

# Install build dependencies (nodejs for openapi bundling)
RUN apk add --no-cache \
      git \
      gcc \
      musl-dev \
      nodejs \
      npm

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

# Copy source code
COPY . .

# Bundle OpenAPI spec to JSON
RUN npx @apidevtools/swagger-cli bundle api/openapi.yaml --outfile api/openapi.bundled.json --type json

# Accept build args from buildx for multi-platform builds
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG GIT_COMMIT

# Build the binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -a \
    -ldflags="-w -s \
              -X main.version=${VERSION}" \
    -o retrowin-server ./cmd/retrowin-server

############################
# 2. Runtime Stage
############################
FROM alpine:3.23.0 AS runtime

ARG VERSION

# OCI labels
LABEL org.opencontainers.image.title="Retrowin Server" \
      org.opencontainers.image.description="Retrowin API Server" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/mandacode-labs/retrowin-go"

# Install runtime dependencies
RUN apk add --no-cache \
      ca-certificates \
      tzdata \
      curl

# Install atlas CLI for versioned migrations (v1.1.0)
ARG ATLAS_VERSION=v1.1.0
RUN curl -sSfLo /usr/local/bin/atlas \
      "https://release.ariga.io/atlas/atlas-linux-amd64-${ATLAS_VERSION}" && \
    chmod +x /usr/local/bin/atlas

# Create non-root user
RUN adduser -D -u 1001 retrowin

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/retrowin-server /app/retrowin-server

# Copy bundled openapi spec
COPY --from=builder /build/api/openapi.bundled.json /app/api/openapi.bundled.json

# Create config directory
RUN mkdir -p /app/config && chown -R retrowin:retrowin /app

# Switch to non-root user
USER retrowin

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set default environment
ENV PORT=8080 \
    HTTP_OPENAPI_PATH=/app/api/openapi.bundled.json \
    GIN_MODE=release

# Run
ENTRYPOINT ["/app/retrowin-server"]
CMD ["serve"]
