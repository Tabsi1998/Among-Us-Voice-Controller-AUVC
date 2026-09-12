# The AUVC bot image.
#
# One process, one SQLite file, no other services. Everything it needs is
# built here from the repository, so there is no nested checkout and no
# upstream release tag involved.
FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY bot/go.mod bot/go.sum ./
RUN go mod download
COPY bot/ ./
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/auvc .

FROM alpine:3.24
RUN apk add --no-cache ca-certificates && \
    addgroup -g 10001 auvc && adduser -D -u 10001 -G auvc auvc && \
    mkdir -p /app/logs /data && chown auvc:auvc /app/logs /data
WORKDIR /app
COPY --from=build /out/auvc /app/auvc
COPY bot/locales/ /app/locales/
COPY LICENSE THIRD_PARTY_NOTICES.md /app/
COPY LICENSES/ /app/LICENSES/
ENV LOCALE_PATH=/app/locales LOG_PATH=/app/logs AUVC_DATABASE_PATH=/data/amongus.db
# The capture connection also carries /healthz. Binding it here rather than
# leaving it unset means a container is reachable and checkable by default;
# publishing the port stays the operator's decision.
ENV AUVC_CAPTURE_ADDR=:8123
EXPOSE 8123
VOLUME ["/data"]
USER auvc

# A health check that only proves the process is running is worth little: a
# bot that lost Discord or its database is as useless as one that crashed,
# and only the crash restarts itself. /healthz checks both.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget --quiet --spider http://127.0.0.1:8123/healthz || exit 1

ENTRYPOINT ["/app/auvc"]
