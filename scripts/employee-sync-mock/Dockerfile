# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o employee-sync-mock main.go

# Run stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from build stage
COPY --from=builder /app/employee-sync-mock .

# Expose port
EXPOSE 8098

# Run application
CMD ["./employee-sync-mock"]