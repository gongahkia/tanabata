FROM golang:1.25-alpine@sha256:56961d79ea8129efddcc0b8643fd8a5416b4e6228cfd477e3fd61deb2672c587 AS build

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

FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc

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
