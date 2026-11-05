FROM golang:1.26.1-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /mini-rpki ./cmd/mini-rpki
FROM alpine:3.23
RUN addgroup -g 10001 mini && adduser -D -H -u 10001 -G mini mini && mkdir -p /data /run/minirpki && chown mini:mini /data /run/minirpki
COPY --from=build /mini-rpki /usr/local/bin/mini-rpki
COPY docker/shell.sh /etc/mini-rpki-shell.sh
ENV ENV=/etc/mini-rpki-shell.sh
ENV MINIRPKI_SOCKET=/run/minirpki/control.sock
USER 10001:10001
EXPOSE 8282
VOLUME /data
HEALTHCHECK --interval=15s --timeout=5s CMD ["mini-rpki", "status"]
ENTRYPOINT ["mini-rpki"]
CMD ["serve"]
