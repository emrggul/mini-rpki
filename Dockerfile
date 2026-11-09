FROM golang:1.26.1-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /mini-rpki ./cmd/mini-rpki
FROM alpine:3.23
LABEL org.opencontainers.image.source="https://github.com/emrggul/mini-rpki"
RUN addgroup -g 10001 mini && adduser -D -H -u 10001 -G mini mini && apk add --no-cache iproute2 su-exec && mkdir -p /data /run/minirpki && chown mini:mini /data /run/minirpki
COPY --from=build /mini-rpki /usr/local/bin/mini-rpki
COPY docker/shell.sh /etc/mini-rpki-shell.sh
COPY docker/entrypoint.sh /usr/local/bin/mini-rpki-entrypoint
RUN chmod 0755 /usr/local/bin/mini-rpki-entrypoint
ENV ENV=/etc/mini-rpki-shell.sh
ENV MINIRPKI_SOCKET=/run/minirpki/control.sock
# 8282 is the plaintext lab listener. 324 is the IANA-assigned RTR TLS port.
# A process opens one transport or the other, never both.
EXPOSE 8282 324
VOLUME /data
HEALTHCHECK --interval=15s --timeout=5s CMD ["mini-rpki", "status"]
ENTRYPOINT ["/usr/local/bin/mini-rpki-entrypoint"]
CMD ["serve"]
