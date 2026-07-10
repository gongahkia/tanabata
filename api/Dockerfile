FROM golang:1.26-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build

WORKDIR /src/api
COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ ./
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X github.com/gongahkia/tanabata/api/internal/buildinfo.Version=${VERSION} -X github.com/gongahkia/tanabata/api/internal/buildinfo.Commit=${COMMIT} -X github.com/gongahkia/tanabata/api/internal/buildinfo.BuildDate=${BUILD_DATE}" \
    -o /tanabata .

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/gongahkia/tanabata" \
    org.opencontainers.image.revision="${REVISION}" \
    org.opencontainers.image.version="${VERSION}"

RUN apk add --no-cache tini && adduser -D -u 10001 tanabata
WORKDIR /app
COPY --from=build /tanabata /usr/local/bin/tanabata
COPY --chown=tanabata:tanabata --from=build /src/api/data /app/data
COPY --chown=tanabata:tanabata openapi /app/openapi

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget -qO- http://localhost:8080/readyz || exit 1
USER tanabata
ENTRYPOINT ["/sbin/tini", "--", "tanabata"]
