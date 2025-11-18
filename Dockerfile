# Stage 1: Build the binaries
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the main ripley binary
RUN go build -o ripley main.go

# Build the linkerdxripley tool (separate module)
RUN cd tools/linkerdxripley && go build -o ../../linkerdxripley .

# Stage 2: Create the final minimal image
FROM alpine:latest

# Install ca-certificates for HTTPS connections
RUN apk add --no-cache ca-certificates

WORKDIR /app/

# Copy the built binaries from the builder stage
COPY --from=builder /build/ripley /app/ripley
COPY --from=builder /build/linkerdxripley /app/linkerdxripley

# Ensure binaries are executable
RUN chmod +x /app/ripley /app/linkerdxripley

ENTRYPOINT ["/app/ripley"]
