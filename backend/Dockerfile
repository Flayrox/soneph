# Stage 1: Build Go Application
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod* go.sum* ./
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o soneph-server main.go

# Stage 2: Final runtime with Python, FFmpeg, and spotdl
FROM python:3.11-alpine

# Install FFmpeg & build essentials needed by spotdl
RUN apk add --no-cache ffmpeg git gcc musl-dev libffi-dev

# Install spotdl CLI
RUN pip install --no-cache-dir spotdl

WORKDIR /app

# Copy binary and post-processing scripts
COPY --from=builder /app/soneph-server /app/soneph-server
COPY embed_lyrics.py /app/embed_lyrics.py
COPY fast_filter.py /app/fast_filter.py
COPY lyrics_retry.py /app/lyrics_retry.py
COPY patch_lyrics_timeout.py /app/patch_lyrics_timeout.py

# Patch spotdl's synced lyrics provider to add a 6-second timeout
# so Musixmatch rate-limiting never stalls ALL downloads in the queue.
RUN python3 /app/patch_lyrics_timeout.py

# Create downloads directory
RUN mkdir -p /app/downloads

ENV DOWNLOAD_DIR=/app/downloads
ENV PORT=8080

EXPOSE 8080

CMD ["/app/soneph-server"]
