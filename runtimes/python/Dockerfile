FROM python:3.14-alpine3.24

LABEL org.opencontainers.image.title="lightning-runtime-python" \
      org.opencontainers.image.description="Lightning sandbox runtime: Python 3.14" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT"

# The official image ships no bytecode and the sandbox root filesystem is
# read-only, so precompile the standard library once instead of on every import.
RUN python3 -m compileall -q -j 0 /usr/local/lib/python3.14 \
 && addgroup -S -g 10001 runner \
 && adduser -S -D -H -u 10001 -G runner -h /tmp -s /sbin/nologin runner \
 && install -d -o runner -g runner -m 0700 /workspace

COPY --chmod=0755 common/run /opt/lightning/run
COPY --chmod=0644 python/lang.sh /opt/lightning/lang.sh

ENV HOME=/tmp LANG=C.UTF-8
WORKDIR /workspace
USER 10001:10001
CMD ["/opt/lightning/run"]
