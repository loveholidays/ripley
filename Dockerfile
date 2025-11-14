# Use a minimal base image since binaries are pre-built
FROM alpine:latest

# Install ca-certificates for HTTPS connections
RUN apk add --no-cache ca-certificates

WORKDIR /app/

# Copy pre-built binaries from the build context
COPY ripley /app/ripley
COPY tools/linkerdxripley/linkerdxripley /app/linkerdxripley

# Ensure binaries are executable
RUN chmod +x /app/ripley /app/linkerdxripley

ENTRYPOINT ["/app/ripley"]
