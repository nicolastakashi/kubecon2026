# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o packetsorter cmd/packetsorter/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache sqlite-libs

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/packetsorter .

# Create data directory for SQLite
RUN mkdir -p /data

# Expose port
EXPOSE 8080

# Set environment variables
ENV DB_PATH=/data/parcels.db

# Run the application
CMD ["./packetsorter"]