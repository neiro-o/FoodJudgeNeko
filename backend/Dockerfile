FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /out/mtv2 . && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /out/mtv2ctl ./cmd/mtv2ctl

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/* && \
    useradd --uid 10001 --create-home mtv2 && mkdir -p /app/cache && chown -R mtv2:mtv2 /app
WORKDIR /app
COPY --from=builder /out/ /app/
USER mtv2
ENV GIN_MODE=release MTV2_CONFIG=/etc/mtv2/config.yml
EXPOSE 8080
STOPSIGNAL SIGTERM
CMD ["/app/mtv2"]
