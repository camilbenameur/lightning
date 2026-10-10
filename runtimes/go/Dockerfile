FROM golang:1.27-alpine3.24 AS go

# Keep only what `go build` needs for a single-file, cgo-free program: drop the
# toolchain's own sources, tests and test data, docs, and the cgo, vet, cover,
# fix and PGO tools.
WORKDIR /usr/local/go
RUN rm -rf api bin/gofmt doc lib/fips140 lib/hg lib/wasm misc test src/cmd \
      pkg/tool/*/cgo pkg/tool/*/cover pkg/tool/*/fix pkg/tool/*/preprofile pkg/tool/*/vet \
 && find src -type d -name testdata -prune -exec rm -rf {} + \
 && find src -name '*_test.go' -delete

FROM alpine:3.24

LABEL org.opencontainers.image.title="lightning-runtime-go" \
      org.opencontainers.image.description="Lightning sandbox runtime: Go 1.27" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT"

COPY --from=go /usr/local/go /usr/local/go

ENV PATH=/usr/local/go/bin:$PATH \
    CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off \
    GOCACHE=/tmp/gocache

# Build the whole standard library once, with the sandbox's settings, into a
# read-only cache that every run reuses (see go/lang.sh), so that a run only
# compiles the program itself, whatever it imports. Its trim.txt goes: each run
# writes its own. The go config in XDG_CONFIG_HOME turns telemetry off.
RUN addgroup -S -g 10001 runner \
 && adduser -S -D -H -u 10001 -G runner -h /tmp -s /sbin/nologin runner \
 && install -d -o runner -g runner -m 0700 /workspace \
 && export XDG_CONFIG_HOME=/opt/lightning/config \
 && go telemetry off \
 && GOCACHE=/opt/lightning/gocache go build std \
 && rm /opt/lightning/gocache/trim.txt

COPY --chmod=0755 common/run /opt/lightning/run
COPY --chmod=0644 go/lang.sh /opt/lightning/lang.sh

ENV HOME=/tmp LANG=C.UTF-8
WORKDIR /workspace
USER 10001:10001
CMD ["/opt/lightning/run"]
