# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o loan-dynamic-api .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/loan-dynamic-api .

# Copy assets (fonts for slip generation)
COPY --from=builder /app/assets ./assets

# Expose port (default 8080 as per main.go)
EXPOSE 8080

# Run the application
CMD ["./loan-dynamic-api"]
