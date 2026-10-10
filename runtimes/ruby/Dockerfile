FROM ruby:3.4-alpine3.24

LABEL org.opencontainers.image.title="lightning-runtime-ruby" \
      org.opencontainers.image.description="Lightning sandbox runtime: Ruby 3.4" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT"

RUN addgroup -S -g 10001 runner \
 && adduser -S -D -H -u 10001 -G runner -h /tmp -s /sbin/nologin runner \
 && install -d -o runner -g runner -m 0700 /workspace

COPY --chmod=0755 common/run /opt/lightning/run
COPY --chmod=0644 ruby/lang.sh ruby/sync.rb /opt/lightning/

ENV HOME=/tmp LANG=C.UTF-8
WORKDIR /workspace
USER 10001:10001
CMD ["/opt/lightning/run"]
