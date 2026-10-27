FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/mtv2-api . && CGO_ENABLED=0 GOOS=linux go build -o /out/mtv2ctl ./cmd/mtv2ctl
# Fixed legacy artifact supplies initialization mappings and existing runtime dependencies.
FROM ghcr.io/neiro-o/foodjudgeneko-api@sha256:9ea667a49446b2c1f1811bd01751544813ca5fe247c488bfaefd8731048db9e4 AS legacy
FROM legacy
USER root
RUN cp /app/mtv2ctl /app/mtv2ctl-legacy
COPY --from=build /out/mtv2-api /app/mtv2
COPY --from=build /out/mtv2ctl /app/mtv2ctl
USER 10001

