# Start fresh from a smaller image
FROM alpine
RUN apk add ca-certificates

COPY ripley /usr/bin/ripley
ENTRYPOINT ["/usr/bin/ripley"]