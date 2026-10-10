FROM rust:1.98-alpine3.24 AS rust

# Keep rustc and the standard library of the host target only: no cargo,
# rustdoc, bundled lld, sanitizer runtimes, std dylib or debug info.
RUN sysroot=$(rustc --print sysroot) target=$(rustc --print host-tuple) \
 && mkdir -p /toolchain/bin /toolchain/lib/rustlib/"$target" \
 && cp "$sysroot"/bin/rustc /toolchain/bin/ \
 && cp "$sysroot"/lib/librustc_driver-*.so /toolchain/lib/ \
 && cp -r "$sysroot"/lib/rustlib/"$target"/lib /toolchain/lib/rustlib/"$target"/ \
 && rm /toolchain/lib/rustlib/"$target"/lib/*san.a /toolchain/lib/rustlib/"$target"/lib/libstd-*.so \
 && strip --strip-debug /toolchain/bin/rustc /toolchain/lib/librustc_driver-*.so

FROM alpine:3.24

LABEL org.opencontainers.image.title="lightning-runtime-rust" \
      org.opencontainers.image.description="Lightning sandbox runtime: Rust 1.98 (edition 2024)" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT"

# rustc links with binutils' ld directly, using the C runtime and musl libc
# that ship with the standard library, so no C compiler is needed.
RUN apk add --no-cache binutils libgcc \
 && addgroup -S -g 10001 runner \
 && adduser -S -D -H -u 10001 -G runner -h /tmp -s /sbin/nologin runner \
 && install -d -o runner -g runner -m 0700 /workspace

COPY --from=rust /toolchain /usr/local
COPY --chmod=0755 common/run /opt/lightning/run
COPY --chmod=0644 rust/lang.sh /opt/lightning/lang.sh

ENV HOME=/tmp LANG=C.UTF-8
WORKDIR /workspace
USER 10001:10001
CMD ["/opt/lightning/run"]
