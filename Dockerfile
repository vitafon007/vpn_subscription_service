# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src

ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -H -u 10001 appuser

COPY --from=builder /out/server /app/server

USER appuser
EXPOSE 23452

ENTRYPOINT ["/app/server"]
