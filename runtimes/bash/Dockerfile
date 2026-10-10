FROM alpine:3.24

LABEL org.opencontainers.image.title="lightning-runtime-bash" \
      org.opencontainers.image.description="Lightning sandbox runtime: Bash 5 with GNU coreutils" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT"

RUN apk add --no-cache bash bc coreutils findutils gawk grep sed \
 && addgroup -S -g 10001 runner \
 && adduser -S -D -H -u 10001 -G runner -h /tmp -s /sbin/nologin runner \
 && install -d -o runner -g runner -m 0700 /workspace

COPY --chmod=0755 common/run /opt/lightning/run
COPY --chmod=0644 bash/lang.sh /opt/lightning/lang.sh

ENV HOME=/tmp LANG=C.UTF-8
WORKDIR /workspace
USER 10001:10001
CMD ["/opt/lightning/run"]
