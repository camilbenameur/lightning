# syntax=docker/dockerfile:1

# Lightning API image. Build context: repository root (see .dockerignore).
#   docker build --build-arg VERSION=1.2.3 --build-arg COMMIT=$(git rev-parse HEAD) -t lightning-api .

# Cross-compile on the build platform: multi-arch images need no emulation for Go.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /out/lightning ./cmd/lightning

# Static, shell-less runtime running as uid 65532.
FROM gcr.io/distroless/static-debian13:nonroot

ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="lightning-api" \
      org.opencontainers.image.description="Lightning API: runs untrusted code in locked-down, throw-away Docker containers" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}"

COPY --from=build /out/lightning /lightning

# distroless "nonroot"
USER 65532:65532
# 8080: public HTTP API (behind Caddy). 9090: Prometheus metrics (keep private).
EXPOSE 8080 9090
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 CMD ["/lightning", "healthcheck"]
ENTRYPOINT ["/lightning"]
