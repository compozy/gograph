# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN make build

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S gograph && \
    adduser -u 1001 -S gograph -G gograph

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/bin/gograph /usr/local/bin/gograph

# Create config directory
RUN mkdir -p /home/gograph/.gograph && \
    chown -R gograph:gograph /home/gograph

# Switch to non-root user
USER gograph

# Set environment variables
ENV HOME=/home/gograph
ENV GOGRAPH_CONFIG_DIR=/home/gograph/.gograph

# Expose MCP server port (when HTTP transport is available)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD gograph version || exit 1

# Default command
ENTRYPOINT ["gograph"]
CMD ["--help"] 