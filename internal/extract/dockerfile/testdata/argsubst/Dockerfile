# ARG before the first FROM is a "meta arg": usable in FROM lines.
ARG GO_VERSION=1.26
ARG BASE=gcr.io/distroless/static-debian12

FROM golang:${GO_VERSION} AS builder
WORKDIR /app
RUN go build -o app .

# COPY --from an EXTERNAL image (not a local stage) — this is a real consumer.
COPY --from=docker.io/library/busybox:1.36 /bin/busybox /bin/busybox

FROM ${BASE}
COPY --from=builder /app/app /app
ENTRYPOINT ["/app"]
