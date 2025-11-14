# Start fresh from a smaller image
FROM alpine:3.15.0
RUN apk add ca-certificates

RUN go build -v -o ripley main.go

COPY ripley /usr/bin/ripley
ENTRYPOINT ["/usr/bin/ripley"]
