# Build stage
FROM golang:1.27-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/pulsewatch ./cmd/server

# Runtime stage
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
  && adduser -D -H -u 10001 pulsewatch
WORKDIR /app
COPY --from=builder /out/pulsewatch /app/pulsewatch
RUN mkdir -p /app/data && chown -R pulsewatch:pulsewatch /app
USER pulsewatch
ENV PULSEWATCH_ADDR=:8080 \
    PULSEWATCH_DB=/app/data/pulsewatch.db \
    PULSEWATCH_WORKERS=5 \
    PULSEWATCH_HTTP_TIMEOUT_SEC=5
EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["/app/pulsewatch"]
