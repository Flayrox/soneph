# ============================================================
# soneph — Single-stage image: Vite frontend embedded in the
# Go binary, served on the same port as the API.
# Build context must be the repository root.
# ============================================================

# Stage 1: Build the Vite frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build the Go binary (embeds frontend/dist via go:embed)
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend-builder /app/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o soneph-server main.go

# Stage 3: Final runtime with Python, FFmpeg, and spotdl
FROM python:3.11-alpine

# Install FFmpeg & build essentials needed by spotdl
RUN apk add --no-cache ffmpeg git gcc musl-dev libffi-dev

# Install spotdl CLI
RUN pip install --no-cache-dir spotdl

WORKDIR /app

# Copy binary (contains the embedded frontend) and post-processing scripts
COPY --from=builder /app/soneph-server /app/soneph-server
COPY backend/embed_lyrics.py /app/embed_lyrics.py
COPY backend/fast_filter.py /app/fast_filter.py
COPY backend/lyrics_retry.py /app/lyrics_retry.py
COPY backend/patch_lyrics_timeout.py /app/patch_lyrics_timeout.py

# Patch spotdl's synced lyrics provider to add a 6-second timeout
# so Musixmatch rate-limiting never stalls ALL downloads in the queue.
RUN python3 /app/patch_lyrics_timeout.py

# Create downloads directory
RUN mkdir -p /app/downloads

ENV DOWNLOAD_DIR=/app/downloads
ENV PORT=8080

EXPOSE 8080

CMD ["/app/soneph-server"]
