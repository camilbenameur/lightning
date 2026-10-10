FROM eclipse-temurin:25-jdk-alpine-3.24 AS jdk

# A trimmed runtime: the Java SE API, the compiler behind the source launcher,
# locale data and sun.misc.Unsafe, without debug info, man pages or headers.
# Resources stay uncompressed so that classes load fast.
RUN jlink --add-modules java.se,jdk.compiler,jdk.localedata,jdk.unsupported \
      --strip-debug --no-man-pages --no-header-files --compress=zip-0 --output /jre

FROM alpine:3.24

LABEL org.opencontainers.image.title="lightning-runtime-java" \
      org.opencontainers.image.description="Lightning sandbox runtime: Java 25 (source launcher)" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT"

COPY --from=jdk /jre /opt/java
COPY --chmod=0755 common/run /opt/lightning/run
COPY --chmod=0644 java/lang.sh /opt/lightning/lang.sh

ENV PATH=/opt/java/bin:$PATH JAVA_HOME=/opt/java

# Build the AOT cache (JEP 483/514/515): a training run compiles and runs a
# typical program (reading its own source as input) with the sandbox's JVM
# flags, then the JVM stores the classes it loaded and linked, with their
# profiles, so that every later run starts from there.
WORKDIR /tmp
RUN --mount=type=tmpfs,target=/tmp \
    --mount=type=bind,source=java/Training.java,target=/mnt/Training.java \
    addgroup -S -g 10001 runner \
 && adduser -S -D -H -u 10001 -G runner -h /tmp -s /sbin/nologin runner \
 && install -d -o runner -g runner -m 0700 /workspace \
 && install -d /opt/lightning/classes \
 && cp /mnt/Training.java Main.java \
 && sh -c '. /opt/lightning/lang.sh && java_main -XX:AOTCacheOutput=/opt/lightning/java.aot' <Main.java >/dev/null

ENV HOME=/tmp LANG=C.UTF-8
WORKDIR /workspace
USER 10001:10001
CMD ["/opt/lightning/run"]
