FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /tanabata .

FROM alpine:3.20

RUN adduser -D -u 10001 tanabata
WORKDIR /app
COPY --from=build /tanabata /usr/local/bin/tanabata
COPY --from=build /src/data /app/data

EXPOSE 8080
USER tanabata
CMD ["tanabata"]
