# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder

WORKDIR /app

ARG VERSION=dev
ARG COMMIT=unknown

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -buildvcs=false \
    -ldflags="-s -w -X github.com/garrettladley/pkgsite-mcp/internal/version.Version=${VERSION} -X github.com/garrettladley/pkgsite-mcp/internal/version.Commit=${COMMIT}" \
    -o /pkgsite-mcp ./cmd/pkgsite-mcp

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /pkgsite-mcp /pkgsite-mcp

EXPOSE 8080

ENTRYPOINT ["/pkgsite-mcp"]
CMD ["serve", "--transport", "http"]
