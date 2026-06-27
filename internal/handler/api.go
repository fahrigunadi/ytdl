package handler

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/fahrigunadi/ytdl/internal/ytdlp"
)

type APIHandler struct {
	svc ytdlp.Downloader
}

func NewAPIHandler(svc ytdlp.Downloader) *APIHandler {
	return &APIHandler{svc: svc}
}

type apiFormat struct {
	FormatID        string  `json:"format_id"`
	Ext             string  `json:"ext"`
	Resolution      string  `json:"resolution"`
	Filesize        int64   `json:"filesize"`
	IsAudioOnly     bool    `json:"is_audio_only"`
	NeedsAudioMerge bool    `json:"needs_audio_merge"`
	ABitrate        float64 `json:"abitrate,omitempty"`
}

type apiInfoResponse struct {
	Title     string      `json:"title"`
	Thumbnail string      `json:"thumbnail"`
	Duration  int         `json:"duration"`
	Formats   []apiFormat `json:"formats"`
}

// GetInfo handles GET /api/info?url=<base64url>
// Returns JSON list of available formats for a YouTube URL.
// Download: GET /download?url=<base64url>&format_id=<id>&ext=<ext>[&needs_merge=1]
func (h *APIHandler) GetInfo(c *gin.Context) {
	urlParam := strings.TrimSpace(c.Query("url"))
	if urlParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing url parameter (base64url-encoded YouTube URL)"})
		return
	}

	decoded, err := base64.RawURLEncoding.DecodeString(urlParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url must be base64url-encoded"})
		return
	}
	rawURL := string(decoded)

	if !IsYouTubeURL(rawURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only YouTube URLs are supported"})
		return
	}

	info, err := h.svc.GetFormats(c.Request.Context(), rawURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch video info: " + err.Error()})
		return
	}

	formats := make([]apiFormat, len(info.Formats))
	for i, f := range info.Formats {
		formats[i] = apiFormat{
			FormatID:        f.FormatID,
			Ext:             f.Ext,
			Resolution:      f.Resolution,
			Filesize:        f.Filesize,
			IsAudioOnly:     f.IsAudioOnly,
			NeedsAudioMerge: f.NeedsAudioMerge,
			ABitrate:        f.ABitrate,
		}
	}

	c.JSON(http.StatusOK, apiInfoResponse{
		Title:     info.Title,
		Thumbnail: info.Thumbnail,
		Duration:  info.Duration,
		Formats:   formats,
	})
}
