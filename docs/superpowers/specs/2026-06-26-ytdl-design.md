# YouTube Downloader — Design Spec

**Date:** 2026-06-26  
**Stack:** Go + Gin + HTMX + go-ytdlp  
**Deployment:** Docker  

---

## Overview

Web app YouTube downloader yang memungkinkan user paste URL YouTube, pilih format (video/audio), lalu file di-stream langsung ke browser — tanpa menyimpan file di server.

---

## Goals

- Fetch format list dari YouTube URL via yt-dlp
- Tampilkan format video (merged mp4/webm) dan audio-only (m4a/opus)
- Stream file langsung ke browser saat user klik download (zero disk I/O)
- Kill yt-dlp process otomatis saat client disconnect
- Deploy via Docker (single container)

## Non-Goals

- Auth / rate limiting
- Support platform selain YouTube
- Merge video+audio stream terpisah (4K, dll) — butuh ffmpeg pipeline, skip untuk v1
- Progress bar download (browser native progress cukup)
- Riwayat download / database

---

## Architecture

### Directory Structure

```
ytdl/
├── cmd/
│   └── server/
│       └── main.go              # entry point, Gin setup, routes
├── internal/
│   ├── handler/
│   │   ├── info.go              # POST /info → fetch & render format list
│   │   └── download.go          # GET /download → stream file to client
│   ├── ytdlp/
│   │   └── service.go           # wrap go-ytdlp: GetFormats(), Stream()
│   └── middleware/
│       └── timeout.go           # request timeout middleware
├── web/
│   ├── templates/
│   │   ├── index.html           # main page (full layout)
│   │   └── formats.html         # HTMX partial: format list + thumbnail
│   └── static/
│       └── htmx.min.js
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

### Component Responsibilities

| Component | Responsibility |
|---|---|
| `cmd/server/main.go` | Init Gin, register routes, load templates, start server |
| `internal/handler/info.go` | Validate URL, call ytdlp.GetFormats(), render formats.html partial |
| `internal/handler/download.go` | Validate params, set response headers, call ytdlp.Stream(), io.Copy to ResponseWriter |
| `internal/ytdlp/service.go` | Wrap go-ytdlp: run yt-dlp process, parse JSON output, expose typed structs |
| `internal/middleware/timeout.go` | Wrap request context with deadline (30s info, 0 for download) |
| `web/templates/index.html` | Main UI: URL input, HTMX attributes, result container |
| `web/templates/formats.html` | Partial: thumbnail, title, format tabs, download buttons |

---

## Routes

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Serve index.html |
| `POST` | `/info` | Fetch format list (HTMX endpoint, returns HTML partial) |
| `GET` | `/download` | Stream file download |
| `GET` | `/static/*` | Static assets |

---

## Data Flow

### Flow 1: Fetch Format List

```
User paste URL → klik "Cek"
  → HTMX: POST /info (hx-post="/info", hx-target="#result")
  → handler/info.go:
      1. Validate URL (must be youtube.com or youtu.be)
      2. Call ytdlp.GetFormats(ctx, url)
      3. ytdlp spawns: yt-dlp --dump-json <url>
      4. Parse JSON → VideoInfo{Title, Thumbnail, Duration, Formats[]}
      5. Filter formats (see Format Filtering)
      6. Render formats.html partial
  → HTMX swap #result div
```

### Flow 2: Stream Download

```
User klik tombol [↓]
  → Browser navigate: GET /download?url=...&format_id=...&ext=...&title=...
  → handler/download.go:
      1. Validate params
      2. Set headers:
           Content-Disposition: attachment; filename="<title>.<ext>"
           Content-Type: <mime based on ext>
           Transfer-Encoding: chunked
           X-Content-Type-Options: nosniff
      3. Call ytdlp.Stream(ctx, url, format_id) → io.Reader
      4. io.Copy(gin.ResponseWriter, reader)
  → Client disconnect → ctx.Done() → yt-dlp process killed via cmd.Cancel()
```

### Format Filtering Logic

**Video tab** — formats dengan video stream (sudah merged, bukan dash):
- Extension: mp4 atau webm
- Has both video codec + audio codec (not `none`)
- Group by resolution label: 1080p, 720p, 480p, 360p, 240p
- Sort: highest resolution first

**Audio tab** — audio-only formats:
- Extension: m4a, webm, opus
- Video codec = "none"
- Sort: highest bitrate first

Formats yang butuh separate merge (vcodec != none && acodec == none) di-skip untuk v1.

---

## ytdlp.Service Interface

```go
type VideoInfo struct {
    Title     string
    Thumbnail string
    Duration  int // seconds
    Formats   []Format
}

type Format struct {
    FormatID   string
    Ext        string
    Resolution string // "1080p", "audio only", etc.
    Filesize   int64  // bytes, may be 0 if unknown
    VCodec     string
    ACodec     string
    ABitrate   float64
    IsAudioOnly bool
}

type Service interface {
    GetFormats(ctx context.Context, url string) (*VideoInfo, error)
    Stream(ctx context.Context, url, formatID string) (io.ReadCloser, error)
}
```

---

## UI Design

Mirip savefrom.net — minimal, single-page, no sidebar.

```
┌─────────────────────────────────────────────────────┐
│  🎬 YT Downloader                                   │
│                                                     │
│  ┌───────────────────────────────────┐ [  Cek  ]   │
│  │ https://youtube.com/watch?v=...   │             │
│  └───────────────────────────────────┘             │
│                                    [spinner saat loading]
│  ── hasil ──────────────────────────────────────── │
│  [thumbnail]  Judul Video                          │
│               Durasi: 3:45                         │
│                                                     │
│  [ Video ] [ Audio ]                               │
│  ┌─────────────────────────────────────────────┐   │
│  │ 1080p   MP4   ~120 MB   [ ↓ Download ]      │   │
│  │ 720p    MP4   ~80 MB    [ ↓ Download ]      │   │
│  │ 480p    MP4   ~40 MB    [ ↓ Download ]      │   │
│  └─────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────┘
```

**Tech:**
- Tailwind CSS via CDN (no build step)
- HTMX via CDN
- Tab Video/Audio: Tailwind class toggle, pure HTML (no JS framework)
- Tombol download = `<a href="/download?...">` — browser handle natively
- Loading: `hx-indicator` Tailwind spinner

---

## Error Handling

| Scenario | Behavior |
|---|---|
| URL bukan YouTube | HTMX swap error partial: "URL tidak valid. Hanya YouTube yang didukung." |
| Video private / unavailable | HTMX swap error partial: pesan dari yt-dlp stderr |
| yt-dlp binary tidak ditemukan | Server panic saat startup (fail fast) |
| Timeout fetch info (>30s) | 408, HTMX swap error partial: "Timeout. Coba lagi." |
| Client disconnect saat stream | ctx cancel → `cmd.Wait()` returns → process killed, no leak |
| Format ID tidak valid | 400 Bad Request |

Error partial adalah fragment HTML kecil yang di-swap ke `#result` div, sama seperti formats partial.

---

## Docker

### Dockerfile (multi-stage)

```dockerfile
# Stage 1: Build Go binary
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o ytdl ./cmd/server

# Stage 2: Runtime
FROM alpine:3.20
RUN apk add --no-cache python3 py3-pip ffmpeg && \
    pip3 install --no-cache-dir yt-dlp
WORKDIR /app
COPY --from=builder /app/ytdl .
COPY web/ web/
EXPOSE 8080
CMD ["./ytdl"]
```

### docker-compose.yml

```yaml
services:
  ytdl:
    build: .
    ports:
      - "8080:8080"
    restart: unless-stopped
```

---

## Constraints & Assumptions

- yt-dlp harus ada di PATH di runtime container
- Merged formats (video+audio) di YouTube max resolusi 1080p
- Filesize dari yt-dlp kadang 0/unknown — tampilkan "-" di UI
- Tidak ada persistent storage apapun
- Concurrency: tidak ada limit explicit di v1 — tiap request spawn 1 yt-dlp process
