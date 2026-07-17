# syntax=docker/dockerfile:1.7

FROM golang:1.26.5-alpine3.23 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/offerpilot-api ./cmd/api

FROM alpine:3.23 AS runtime

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 offerpilot \
    && adduser -S -D -H -u 10001 -G offerpilot -s /sbin/nologin offerpilot

COPY --from=build --chown=10001:10001 /out/offerpilot-api /usr/local/bin/offerpilot-api

ENV OFFERPILOT_HTTP_ADDR=0.0.0.0:8080

USER 10001:10001

EXPOSE 8080

HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=12 \
    CMD wget -q -T 2 -O /dev/null http://127.0.0.1:8080/health/ready || exit 1

ENTRYPOINT ["/usr/local/bin/offerpilot-api"]