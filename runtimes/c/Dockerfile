FROM alpine:3.24

LABEL org.opencontainers.image.title="lightning-runtime-c" \
      org.opencontainers.image.description="Lightning sandbox runtime: C17 (GCC 15)" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT"

COPY common/linebuf.c /tmp/

# The fixed compile command never uses LTO, GCC plugins or sanitizers: drop
# them, which halves the size of the toolchain. linebuf.o is linked into every
# program (see common/linebuf.c).
RUN apk add --no-cache gcc musl-dev \
 && rm -rf /usr/bin/lto-dump /usr/libexec/gcc/*/*/lto1 /usr/lib/gcc/*/*/plugin /usr/lib/lib*san* \
 && install -d /opt/lightning \
 && gcc -O2 -c -o /opt/lightning/linebuf.o /tmp/linebuf.c \
 && rm /tmp/linebuf.c \
 && addgroup -S -g 10001 runner \
 && adduser -S -D -H -u 10001 -G runner -h /tmp -s /sbin/nologin runner \
 && install -d -o runner -g runner -m 0700 /workspace

COPY --chmod=0755 common/run /opt/lightning/run
COPY --chmod=0644 c/lang.sh /opt/lightning/lang.sh

ENV HOME=/tmp LANG=C.UTF-8
WORKDIR /workspace
USER 10001:10001
CMD ["/opt/lightning/run"]
