# Start fresh from a smaller image
FROM golang:1.23-alpine
RUN apk add ca-certificates

RUN go build -v -o ripley main.go

COPY ripley /usr/bin/ripley
ENTRYPOINT ["/usr/bin/ripley"]
