FROM alpine:3.24

LABEL org.opencontainers.image.title="lightning-runtime-cpp" \
      org.opencontainers.image.description="Lightning sandbox runtime: C++20 (GCC 15)" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT"

COPY common/linebuf.c /tmp/

# The fixed compile command never uses LTO, GCC plugins or sanitizers: drop
# them, which saves a third of the toolchain's size. linebuf.o is linked into
# every program (see common/linebuf.c).
RUN apk add --no-cache g++ \
 && rm -rf /usr/bin/lto-dump /usr/libexec/gcc/*/*/lto1 /usr/lib/gcc/*/*/plugin /usr/lib/lib*san* \
 && install -d /opt/lightning \
 && gcc -O2 -c -o /opt/lightning/linebuf.o /tmp/linebuf.c \
 && rm /tmp/linebuf.c \
 && addgroup -S -g 10001 runner \
 && adduser -S -D -H -u 10001 -G runner -h /tmp -s /sbin/nologin runner \
 && install -d -o runner -g runner -m 0700 /workspace

COPY --chmod=0755 common/run /opt/lightning/run
COPY --chmod=0644 cpp/lang.sh /opt/lightning/lang.sh

# Precompile the usual first includes with the build flags of lang.sh: this
# makes compiling a program that starts with <iostream> about 2.5 times
# faster, and one that starts with <bits/stdc++.h> twice as fast.
RUN . /opt/lightning/lang.sh \
 && install -d /opt/lightning/pch/bits \
 && cxx -x c++-header -o /opt/lightning/pch/iostream.gch /usr/include/c++/*/iostream \
 && cxx -x c++-header -o /opt/lightning/pch/bits/stdc++.h.gch /usr/include/c++/*/*/bits/stdc++.h

ENV HOME=/tmp LANG=C.UTF-8
WORKDIR /workspace
USER 10001:10001
CMD ["/opt/lightning/run"]
