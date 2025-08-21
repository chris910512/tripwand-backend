# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o tripwand-backend \
    ./cmd/main.go

# Final stage - minimal runtime image
FROM gcr.io/distroless/static-debian12:nonroot

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /app/tripwand-backend /tripwand-backend

# Use non-root user
USER nonroot:nonroot

# Expose port
EXPOSE 8080

# Set environment
ENV TZ=Asia/Seoul

# Run the binary
ENTRYPOINT ["/tripwand-backend"]