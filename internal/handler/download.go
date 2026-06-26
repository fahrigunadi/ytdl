package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gunadi/ytdl/internal/ytdlp"
)

type DownloadHandler struct {
	svc ytdlp.Downloader
}

func NewDownloadHandler(svc ytdlp.Downloader) *DownloadHandler {
	return &DownloadHandler{svc: svc}
}

func (h *DownloadHandler) Handle(c *gin.Context) {
	rawURL := c.Query("url")
	formatID := c.Query("format_id")
	ext := c.Query("ext")
	title := c.Query("title")

	if rawURL == "" || formatID == "" || ext == "" {
		c.String(http.StatusBadRequest, "missing required parameters: url, format_id, ext")
		return
	}
	if !IsYouTubeURL(rawURL) {
		c.String(http.StatusBadRequest, "invalid URL")
		return
	}

	r, err := h.svc.Stream(c.Request.Context(), rawURL, formatID)
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
	default:
		return "application/octet-stream"
	}
}
