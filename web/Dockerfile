# syntax=docker/dockerfile:1.7

FROM node:22.16.0-alpine3.22 AS build

WORKDIR /app

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY tsconfig.json vite.config.ts eslint.config.js index.html ./
COPY src ./src

ARG VITE_API_BASE_URL=""
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}

RUN npm run build

FROM busybox:1.37.0-uclibc AS health

FROM joseluisq/static-web-server:2.43.0 AS runtime

COPY --from=health /bin/busybox /bin/busybox
COPY --from=build --chown=10001:10001 /app/dist /public

ENV SERVER_HOST=0.0.0.0 \
    SERVER_PORT=8080 \
    SERVER_ROOT=/public \
    SERVER_COMPRESSION=true \
    SERVER_CACHE_CONTROL_HEADERS=true \
    SERVER_DIRECTORY_LISTING=false \
    SERVER_LOG_LEVEL=info \
    SERVER_GRACE_PERIOD=1

USER 10001:10001

EXPOSE 8080

HEALTHCHECK --interval=5s --timeout=3s --start-period=3s --retries=12 \
    CMD ["/bin/busybox", "wget", "-q", "-T", "2", "-O", "/dev/null", "http://127.0.0.1:8080/"]

ENTRYPOINT ["/static-web-server"]