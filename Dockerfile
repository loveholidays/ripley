FROM golang:1.19-alpine as build

ADD . /src/

RUN cd /src && \
    go mod download && \
    go build -o /ripley . && \
    go build -o /dummyweb etc/dummyweb.go

##################################
# Start fresh from a smaller image
##################################
FROM alpine:latest
RUN apk add ca-certificates

COPY --from=build /ripley /usr/bin/ripley
# COPY --from=build /dummyweb /usr/bin/dummyweb

ENTRYPOINT ["/usr/bin/ripley"]
