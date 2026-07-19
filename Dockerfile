# ── Stage 1: Build ────────────────────────────────────────
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /aegisrl ./cmd/server

# ── Stage 2: Runtime ──────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /aegisrl /app/aegisrl

# Non-root user for security
RUN addgroup -S aegisrl && adduser -S aegisrl -G aegisrl
USER aegisrl

EXPOSE 8081 9100

ENTRYPOINT ["/app/aegisrl"]
