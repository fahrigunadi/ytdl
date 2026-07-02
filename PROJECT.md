# ytdl — YouTube Downloader

Web app: paste YouTube URL, pick format, file streams straight to browser. No disk storage, no DB, no auth.

**Live:** https://ytdl.fahrigunadi.dev
**Module:** `github.com/fahrigunadi/ytdl`
**Go:** 1.25

---

## Idea / Goals

- Fetch available formats from a YouTube URL via `yt-dlp`.
- Show video formats (merged, up to 1080p+) and audio-only formats separately.
- Stream file directly to client on download click — zero disk I/O on server.
- Kill `yt-dlp` subprocess automatically on client disconnect (no zombie processes).
- Single-container Docker deploy.
- Expose a public JSON API alongside the HTMX web UI.

**Non-goals (v1):** auth/rate limiting, non-YouTube platforms, progress bar, download history/DB.

---

## Stack

| Layer | Tech |
|---|---|
| Backend | Go 1.25 + Gin (`github.com/gin-gonic/gin` v1.10.0) |
| Downloader | `yt-dlp` binary (external process, must be in `PATH`) |
| Frontend | HTMX (vendored `web/static/htmx.min.js`) + Tailwind CSS (CDN, no build step) |
| Templates | Go `html/template`, `web/templates/*.html` |
| Deploy | Docker multi-stage (golang:1.25-alpine builder → alpine:3.20 runtime with python3/ffmpeg/yt-dlp) |

No database, no persistent storage, no JS framework/bundler.

---

## Directory Structure

```
ytdl/
├── cmd/server/main.go            # entry point: Gin setup, routes, template funcs
├── internal/
│   ├── handler/
│   │   ├── info.go               # POST /info → HTMX partial (format list)
│   │   ├── download.go           # GET /download → stream file to client
│   │   ├── api.go                # GET /api/info → JSON format list
│   │   └── funcs.go              # Go template helper funcs
│   ├── ytdlp/
│   │   └── service.go            # wraps yt-dlp process: GetFormats(), Stream()
│   └── middleware/
│       └── timeout.go            # request context timeout wrapper
├── web/
│   ├── templates/
│   │   ├── index.html            # main page: URL form + collapsible API docs
│   │   ├── formats.html          # HTMX partial: thumbnail/title/tabs/download buttons
│   │   └── error.html            # HTMX partial: error message box
│   └── static/htmx.min.js
├── docs/superpowers/             # design spec + implementation plan (SDD workflow docs)
├── .superpowers/sdd/             # per-task briefs/reports/review diffs from build process
├── Dockerfile                    # multi-stage build
├── docker-compose.yml            # single service, port 8080
└── go.mod / go.sum
```

---

## Routes

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/` | Serve `index.html` (main UI) |
| `POST` | `/info` | HTMX endpoint — form-encoded `url`, returns `formats.html` partial. 30s timeout. |
| `GET` | `/download` | Stream file to browser (query params, see below) |
| `GET` | `/api/info` | Public JSON API — `?url=<youtube-url>`. 30s timeout. |
| `GET` | `/static/*` | Static assets (htmx.min.js) |

---

## Core Component: `internal/ytdlp/service.go`

```go
type Downloader interface {
    GetFormats(ctx context.Context, url string) (*VideoInfo, error)
    Stream(ctx context.Context, url, formatID string) (io.ReadCloser, error)
}
```

- `Service.New()` — resolves `yt-dlp` path via `exec.LookPath`, fails startup if not found (fail-fast). Holds a semaphore (`maxConcurrentDownloads = 5`) capping simultaneous streams.
- `GetFormats` — runs `yt-dlp --dump-single-json --no-playlist --no-warnings <url>`, parses JSON into `rawVideoInfo`/`rawFormat`, then `parseVideoInfo()` transforms it:
  - Audio-only formats (`vcodec=none, acodec!=none`) kept as-is, resolution label = `"<abr>kbps"` or `"audio"`.
  - Video-only formats (`vcodec!=none, acodec=none`, e.g. 1080p+ DASH streams) marked `NeedsAudioMerge=true`, ext forced to `mkv` (yt-dlp merges to mkv when piping to stdout).
  - Video+audio (already-merged) formats used directly.
  - Dedup: one Format per resolution label, keeping the **largest filesize** candidate (`bestVideo` map).
  - Video formats prepended in first-seen resolution order; audio formats appended after.
- `Stream` — acquires semaphore slot (or aborts on `ctx.Done()`), runs `yt-dlp -f <formatID> -o - --no-playlist --no-warnings <url>`, returns stdout pipe wrapped in `cmdReadCloser`. `needs_audio_merge` streams request format `formatID+bestaudio` (yt-dlp handles the ffmpeg mux internally, pipes muxed mkv to stdout).
- `cmdReadCloser.Close()` — closes the pipe, `cmd.Wait()`s to reap the subprocess, releases semaphore. Client disconnect → context cancel → process killed → no leaks.

---

## Handlers

### `internal/handler/info.go` (HTMX flow)
- `POST /info`, form field `url`.
- `IsYouTubeURL()` (exported, shared validator) — accepts `youtube.com`, `www.youtube.com`, `youtu.be`.
- Renders `formats.html` on success, `error.html` (HTTP 400) on invalid URL or fetch failure.

### `internal/handler/download.go`
- `GET /download?url=<base64url>&format_id=<id>&ext=<ext>&title=<title>&needs_merge=<0|1>`.
- Validation: `url` base64url-decoded then re-checked via `IsYouTubeURL`; `ext` whitelist (`mp4, webm, m4a, opus, mkv`); `format_id` regex `^[a-zA-Z0-9_\-+]+$`.
- `needs_merge=1` → stream format string becomes `formatID+bestaudio`.
- Sets `Content-Disposition: attachment; filename="<sanitized-title>.<ext>"`, `Content-Type` (mapped per ext), `X-Content-Type-Options: nosniff`.
- `SanitizeFilename()` (exported) strips path/control chars (`/ \ : * ? " < > |`) from title, defaults to `"video"` if empty.
- `io.Copy` from yt-dlp stdout straight to `gin.ResponseWriter` — no buffering to disk.

### `internal/handler/api.go` (public JSON API)
- `GET /api/info?url=<raw-youtube-url>` — note: **raw URL here**, not base64 (unlike `/download`).
- Response: `{title, thumbnail, duration, formats: [{format_id, ext, resolution, filesize, is_audio_only, needs_audio_merge, abitrate?, download_url}]}`.
- `download_url` is prebuilt, ready-to-use `/download?...` link (base64-encodes the URL internally via `buildDownloadURL`) — API consumers don't construct it themselves.
- Errors return JSON `{"error": "..."}` (400 for bad/non-YouTube URL, 500 for yt-dlp failure).

### `internal/handler/funcs.go`
Template funcs registered via `r.SetFuncMap`: `formatDuration` (seconds → `h:mm:ss` or `m:ss`), `formatFilesize` (bytes → `~N MB`, or `-` if unknown), `b64url` (base64url encode, used by `formats.html` to build download links inline).

### `internal/middleware/timeout.go`
Wraps request context with a deadline via `context.WithTimeout`; used on `/info` and `/api/info` (30s each). `/download` has no timeout — long streams must run unbounded.

---

## Frontend Flow

### `index.html`
- Tailwind (CDN) + HTMX (`/static/htmx.min.js`), single card layout, no build step.
- Form: `hx-post="/info"`, `hx-target="#result"`, `hx-indicator="#spinner"`.
- Collapsible **API Documentation** section (vanilla JS `classList.toggle`) — documents `GET /api/info` and `GET /download` with params tables and example request/response JSON.

### `formats.html` (HTMX partial)
- Thumbnail + title + duration.
- Video/Audio tab toggle (pure inline `<script>`, `showTab()` function, Tailwind class swap — no framework).
- Each row: resolution, ext, filesize, `<a href="/download?...">` download button (browser handles download natively, no JS).
- Video tab links include `needs_merge=1` when `.NeedsAudioMerge`.

### `error.html` (HTMX partial)
Small red alert box swapped into `#result` on any failure.

---

## Data Flow Summary

**Format lookup (HTMX):**
```
paste URL → click "Check" → POST /info → validate → yt-dlp --dump-single-json
→ parse + dedup formats → render formats.html → htmx swaps into #result
```

**Format lookup (API):**
```
GET /api/info?url=<raw-url> → validate → yt-dlp --dump-single-json
→ parse + dedup formats → JSON with prebuilt download_url per format
```

**Download (both flows converge here):**
```
click download link / GET /download?url=<b64>&format_id=&ext=&title=&needs_merge=
→ decode+validate params → yt-dlp -f <id[+bestaudio]> -o - (piped stdout)
→ set headers → io.Copy to response → client disconnect kills subprocess
```

---

## Security / Validation Notes

- Only `youtube.com` / `www.youtube.com` / `youtu.be` hosts accepted — blocks SSRF via arbitrary URL passthrough to yt-dlp.
- `format_id` restricted to safe charset regex (prevents shell/arg injection into yt-dlp's `-f` flag — note `+` allowed for the merge syntax).
- `ext` whitelist prevents arbitrary `Content-Type` header injection.
- Filenames sanitized before use in `Content-Disposition`.
- `X-Content-Type-Options: nosniff` set on downloads.
- Concurrency capped at 5 simultaneous yt-dlp streams via semaphore (resource exhaustion guard).
- No auth/rate-limiting — explicitly out of scope for v1 (see Non-goals).

---

## Testing

Table-driven Go tests, `net/http/httptest`, mock `Downloader` interface for handler isolation:

| File | Lines | Covers |
|---|---|---|
| `internal/handler/info_test.go` | 116 | URL validation, format rendering, error paths |
| `internal/handler/download_test.go` | 100 | param validation, ext/format_id whitelist, headers, streaming |
| `internal/handler/api_test.go` | 92 | JSON response shape, download_url construction, error JSON |
| `internal/ytdlp/service_test.go` | 84 | format parsing/dedup logic (`parseVideoInfo`) |
| `internal/middleware/timeout_test.go` | 55 | context deadline propagation |

Run: `go test ./...`

---

## Docker

**Dockerfile** (multi-stage):
1. `golang:1.25-alpine` builder — `CGO_ENABLED=0 GOOS=linux go build -o ytdl ./cmd/server`.
2. `alpine:3.20` runtime — installs `python3 py3-pip ffmpeg`, `pip3 install yt-dlp` (`--break-system-packages`), copies binary + `web/` dir. Exposes `:8080`.

**docker-compose.yml** — single `ytdl` service, build from `.`, port `8080:8080`, `restart: unless-stopped`.

No env vars, no volumes, no secrets — fully stateless.

---

## Build History (chronological, from git log)

1. `chore: initial design spec and implementation plan` — spec-driven dev (SDD) docs in `docs/superpowers/`.
2. `chore: project scaffold`
3. `feat: ytdlp service` — format fetch + direct stream pipe
4. `fix: reap yt-dlp subprocess in Stream via cmdReadCloser`
5. `feat: timeout middleware`
6. `feat: info handler, URL validation, template funcs`
7. `feat: download handler — pipe yt-dlp stdout to ResponseWriter`
8. `feat: full UI templates`
9. `feat: main — wire Gin, routes, template funcs`
10. `feat: Docker multi-stage build`
11. `fix: ext/format_id validation, pipe leak on Start failure, go mod tidy`
12. `feat: include video-only formats (1080p+) with ffmpeg merge via +bestaudio`
13. `feat: semaphore limits concurrent yt-dlp downloads to 5`
14. `fix: deduplicate video formats per resolution, keep highest filesize`
15. `feat: base64 encode YouTube URL in download query param`
16. `fix: translate Indonesian UI text to English`
17. `refactor: rename module to github.com/fahrigunadi/ytdl`
18. `feat: add public JSON API endpoint GET /api/info`
19. `feat: API accepts raw YouTube URL, include download_url per format`
20. `feat: add collapsible API docs section to UI`

Built via a spec-driven-development (SDD) workflow: design spec (`docs/superpowers/specs/`) → task plan (`docs/superpowers/plans/`) → per-task briefs/reports/review diffs (`.superpowers/sdd/`) → final branch review before ship (see `progress.md`: status **READY TO SHIP**, minor note on title CRLF stripping left as-is since `net/http` header handling already guards it).

---

## Known Constraints / Assumptions

- `yt-dlp` must be present in container `PATH` at runtime.
- Merged (single-file) YouTube formats cap at 1080p; higher resolutions are video-only DASH streams requiring the `+bestaudio` merge path.
- `filesize` from yt-dlp is sometimes 0/unknown — UI shows `-`.
- No persistent storage anywhere in the stack.
- Concurrency limited to 5 parallel yt-dlp processes server-wide (not per-IP).
