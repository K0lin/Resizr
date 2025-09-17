# Build stage
FROM golang:1.25.1-alpine3.21 AS builder

# Set working directory
WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o resizr ./cmd/server/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates curl tzdata && \
    update-ca-certificates

# Create non-root user
RUN adduser -D -s /bin/sh appuser

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/resizr .

# Copy health check script
COPY healthcheck.sh .

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Change ownership to appuser and make health check script executable
RUN chown appuser:appuser /app/resizr /app/healthcheck.sh && \
    chmod +x /app/healthcheck.sh

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check using smart script
HEALTHCHECK --interval=30s --timeout=15s --start-period=5s --retries=3 \
    CMD ./healthcheck.sh

# Run the binary
ENTRYPOINT ["./resizr"]
