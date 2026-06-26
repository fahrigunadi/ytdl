package handler

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/fahrigunadi/ytdl/internal/ytdlp"
)

var validExt = map[string]bool{"mp4": true, "webm": true, "m4a": true, "opus": true, "mkv": true}

var validFormatID = regexp.MustCompile(`^[a-zA-Z0-9_\-+]+$`)

type DownloadHandler struct {
	svc ytdlp.Downloader
}

func NewDownloadHandler(svc ytdlp.Downloader) *DownloadHandler {
	return &DownloadHandler{svc: svc}
}

func (h *DownloadHandler) Handle(c *gin.Context) {
	urlParam := c.Query("url")
	formatID := c.Query("format_id")
	ext := c.Query("ext")
	title := c.Query("title")
	needsMerge := c.Query("needs_merge") == "1"

	if urlParam == "" || formatID == "" || ext == "" {
		c.String(http.StatusBadRequest, "missing required parameters: url, format_id, ext")
		return
	}

	decoded, err := base64.RawURLEncoding.DecodeString(urlParam)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid url encoding")
		return
	}
	rawURL := string(decoded)
	if !validExt[ext] {
		c.String(http.StatusBadRequest, "invalid ext: must be one of mp4, webm, m4a, opus, mkv")
		return
	}
	if !validFormatID.MatchString(formatID) {
		c.String(http.StatusBadRequest, "invalid format_id")
		return
	}
	if !IsYouTubeURL(rawURL) {
		c.String(http.StatusBadRequest, "invalid URL")
		return
	}

	streamFormat := formatID
	if needsMerge {
		streamFormat = formatID + "+bestaudio"
	}

	r, err := h.svc.Stream(c.Request.Context(), rawURL, streamFormat)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to start stream: %s", err.Error())
		return
	}
	defer r.Close()

	filename := SanitizeFilename(title) + "." + ext
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Type", mimeForExt(ext))
	c.Header("X-Content-Type-Options", "nosniff")

	io.Copy(c.Writer, r) //nolint:errcheck — client disconnect is expected, yt-dlp killed by ctx
}

// SanitizeFilename exported for testing.
func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "video"
	}
	r := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-",
		"*", "-", "?", "", `"`, "",
		"<", "", ">", "", "|", "-",
	)
	return r.Replace(name)
}

func mimeForExt(ext string) string {
	switch ext {
	case "mp4":
		return "video/mp4"
	case "webm":
		return "video/webm"
	case "m4a":
		return "audio/mp4"
	case "opus":
		return "audio/ogg"
	case "mkv":
		return "video/x-matroska"
	default:
		return "application/octet-stream"
	}
}
