# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy module files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY coordinator/ ./coordinator/
COPY gen/         ./gen/
COPY protos/      ./protos/

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /coordinator \
    ./coordinator

# ── Stage 2: Minimal runtime image ────────────────────────────────────────────
FROM alpine:3.19

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /coordinator /usr/local/bin/coordinator

USER appuser
EXPOSE 8090

CMD ["coordinator"]
