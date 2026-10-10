# syntax=docker/dockerfile:1

# Lightning web image: the Vite build served by Caddy, which also terminates TLS
# and reverse-proxies /api/* to the API. Build context: web/.

# The bundle is architecture independent: build it once, on the build platform.
FROM --platform=$BUILDPLATFORM node:24-alpine AS build
WORKDIR /app
# No registry round trip to check for a newer npm before every command.
ENV NPM_CONFIG_UPDATE_NOTIFIER=false

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund

COPY . .
# Same origin: the browser calls /api/v1/... and Caddy forwards it.
ENV VITE_API_BASE_URL=""
RUN npm run build

FROM caddy:2-alpine

ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="lightning-web" \
      org.opencontainers.image.description="Lightning web UI served by Caddy (automatic HTTPS, strict security headers, /api reverse proxy)" \
      org.opencontainers.image.source="https://github.com/camilbenameur/lightning" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}"

# Run unprivileged. Docker lets container processes bind ports below 1024
# (net.ipv4.ip_unprivileged_port_start=0), so the binary's file capability is
# dropped: it would make exec fail under cap_drop ALL + no-new-privileges.
RUN setcap -r /usr/bin/caddy \
    && addgroup -S -g 10000 caddy \
    && adduser -S -D -H -h /data -u 10000 -G caddy caddy \
    && chown -R caddy:caddy /data /config

COPY Caddyfile /etc/caddy/Caddyfile
COPY --from=build /app/dist /srv

USER 10000:10000
