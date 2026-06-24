FROM --platform=$BUILDPLATFORM golang:1.26 AS builder
ARG TARGETOS TARGETARCH
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o agentd ./cmd/agentd

FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/agentd /agentd
COPY --from=builder /app/static /static
EXPOSE 8080
ENTRYPOINT ["/agentd"]
