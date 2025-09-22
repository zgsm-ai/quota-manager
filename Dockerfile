# Build stage
FROM golang:1.23-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o quota-manager cmd/main.go

# Run stage
FROM alpine:latest AS runtime

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from build stage
COPY --from=builder /app/quota-manager .
COPY --from=builder /app/config.yaml .

# Expose port
EXPOSE 8080

# Run application
CMD ["./quota-manager"]