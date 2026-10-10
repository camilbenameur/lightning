FROM oven/bun:1.4-alpine AS bun

# Bun is a single binary: copy it onto plain Alpine, leaving bunx and the rest behind.
FROM alpine:3.24

LABEL org.opencontainers.image.title="lightning-runtime-typescript" \
      org.opencontainers.image.description="Lightning sandbox runtime: TypeScript (Bun 1.4)" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT"

RUN apk add --no-cache libgcc libstdc++ \
 && addgroup -S -g 10001 runner \
 && adduser -S -D -H -u 10001 -G runner -h /tmp -s /sbin/nologin runner \
 && install -d -o runner -g runner -m 0700 /workspace

COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun
COPY --chmod=0755 common/run /opt/lightning/run
COPY --chmod=0644 typescript/lang.sh /opt/lightning/lang.sh

ENV HOME=/tmp LANG=C.UTF-8 DO_NOT_TRACK=1
WORKDIR /workspace
USER 10001:10001
CMD ["/opt/lightning/run"]
