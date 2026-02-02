# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
# ca-certificates is needed for HTTPS requests if your app makes them
RUN apk add --no-cache git ca-certificates tzdata

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# -ldflags="-w -s" reduces binary size by stripping debug info
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o mizuflow-server ./cmd/server

# Stage 2: Create a minimal runtime image
FROM alpine:latest

WORKDIR /app

# Copy necessary files from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/mizuflow-server .

# Copy config file (Optional: In production, you might mount this or use ENV vars)
# We copy it here for "out of the box" demo experience
COPY config/config.yaml ./config/config.yaml

# Create a non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Expose the application port
EXPOSE 8080

# Run the binary
CMD ["./mizuflow-server"]
