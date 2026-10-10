FROM node:24-alpine3.24 AS node

# Only the node binary is needed: npm, npx, corepack and yarn are useless
# without network access, so start from plain Alpine and leave them behind.
FROM alpine:3.24

LABEL org.opencontainers.image.title="lightning-runtime-javascript" \
      org.opencontainers.image.description="Lightning sandbox runtime: JavaScript (Node.js 24)" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT"

RUN apk add --no-cache libgcc libstdc++ \
 && addgroup -S -g 10001 runner \
 && adduser -S -D -H -u 10001 -G runner -h /tmp -s /sbin/nologin runner \
 && install -d -o runner -g runner -m 0700 /workspace

COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --chmod=0755 common/run /opt/lightning/run
COPY --chmod=0644 javascript/lang.sh /opt/lightning/lang.sh

ENV HOME=/tmp LANG=C.UTF-8
WORKDIR /workspace
USER 10001:10001
CMD ["/opt/lightning/run"]
