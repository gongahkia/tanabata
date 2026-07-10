FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X github.com/gongahkia/tanabata/api/internal/buildinfo.Version=${VERSION} -X github.com/gongahkia/tanabata/api/internal/buildinfo.Commit=${COMMIT} -X github.com/gongahkia/tanabata/api/internal/buildinfo.BuildDate=${BUILD_DATE}" \
    -o /tanabata .

FROM alpine:3.20

RUN adduser -D -u 10001 tanabata
WORKDIR /app
COPY --from=build /tanabata /usr/local/bin/tanabata
COPY --chown=tanabata:tanabata --from=build /src/data /app/data

EXPOSE 8080
USER tanabata
ENTRYPOINT ["tanabata"]
