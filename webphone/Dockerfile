# Self-hosted WebRTC softphone UI: Browser-Phone (InnovateAsterisk) served by
# nginx over HTTPS with our own CA-signed cert (mounted at /tls by compose).
#
# Browser-Phone is a static SPA built on SIP.js (MIT); the app itself is
# AGPL-3.0 — fine for internal/lab use and modification; do not ship a closed
# fork to external users.

FROM alpine:3.20 AS fetch
RUN apk add --no-cache curl tar
# Pin a ref for reproducibility (branch/tag/commit). Override at build time:
#   docker compose build --build-arg PHONE_REF=<commit>
ARG PHONE_REF=master
RUN curl -fsSL "https://github.com/InnovateAsterisk/Browser-Phone/archive/${PHONE_REF}.tar.gz" \
      | tar -xz \
 && mv Browser-Phone-*/Phone /phone

FROM nginx:1.27-alpine
COPY --from=fetch /phone /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 443
