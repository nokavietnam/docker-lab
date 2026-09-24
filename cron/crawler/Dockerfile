# Build stage: Compile static Go binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compile with zero cgo and stripped symbol tables (-s -w) for minimal size and fast execution
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/crawler ./cmd/crawler

# Final stage: Ultra-lightweight secure runtime
FROM alpine:3.20

WORKDIR /app

# Install root TLS certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/crawler /app/crawler

USER appuser

# Go runtime resource tuning:
# GOMEMLIMIT prevents OOM by triggering GC before hitting container limits
# GOGC controls aggressive collection for low-RAM footprints
ENV GOMEMLIMIT=48MiB \
    GOGC=80

ENTRYPOINT ["/app/crawler"]
