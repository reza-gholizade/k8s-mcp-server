# Use the official Golang image with the latest version for building
FROM --platform=$BUILDPLATFORM golang:alpine AS builder

# Install git (required for go modules) and update all packages
RUN apk --no-cache upgrade && apk --no-cache add git ca-certificates

# Set the working directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Set GOTOOLCHAIN to auto to allow automatic toolchain upgrades
ENV GOTOOLCHAIN=auto

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build arguments for cross-compilation
ARG TARGETOS
ARG TARGETARCH

# Build the Go app with cross-compilation support
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-w -s" -o k8s-mcp-server main.go

# Use a minimal base image instead of scratch for better compatibility
FROM alpine:3.21

# Install ca-certificates and curl for health checks
RUN apk --no-cache add ca-certificates curl

# Create a non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Copy the binary from the builder stage
COPY --from=builder /app/k8s-mcp-server /usr/local/bin/k8s-mcp-server

# Make the binary executable
RUN chmod +x /usr/local/bin/k8s-mcp-server

# Switch to non-root user
USER appuser

# Expose the port the app runs on
EXPOSE 8080

# Set default environment variables
ENV SERVER_MODE=sse
ENV SERVER_PORT=8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/ || exit 1

# Command to run the executable
ENTRYPOINT ["/usr/local/bin/k8s-mcp-server"]
