FROM golang:1.27-alpine@sha256:4cb7ac979db5fcc41cae44b2227ba5ab8a51e8807f40d9ba4dee20a0ad960b5b AS build

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
