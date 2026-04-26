# ── Build stage ───────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /coppermind ./cmd/coppermind

# ── Runtime stage ────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S coppermind && \
    adduser -S -G coppermind coppermind

COPY --from=builder /coppermind /usr/local/bin/coppermind

RUN mkdir -p /data && chown coppermind:coppermind /data
VOLUME /data
WORKDIR /data

USER coppermind

EXPOSE 5000

ENV COPPERMIND_DB=/data/coppermind.db \
    COPPERMIND_DATA_DIR=/data \
    COPPERMIND_HOST=0.0.0.0 \
    COPPERMIND_PORT=5000

ENTRYPOINT ["coppermind"]
CMD ["serve"]
