# Stage 1: Build Go binary
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ytdl ./cmd/server

# Stage 2: Runtime with yt-dlp
FROM alpine:3.20
RUN apk add --no-cache python3 py3-pip ffmpeg && \
    pip3 install --no-cache-dir --break-system-packages yt-dlp
WORKDIR /app
COPY --from=builder /app/ytdl .
COPY web/ web/
EXPOSE 8080
CMD ["./ytdl"]
