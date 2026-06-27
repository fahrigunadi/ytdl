package handler

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
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
	DownloadURL     string  `json:"download_url"`
}

type apiInfoResponse struct {
	Title     string      `json:"title"`
	Thumbnail string      `json:"thumbnail"`
	Duration  int         `json:"duration"`
	Formats   []apiFormat `json:"formats"`
}

// GetInfo handles GET /api/info?url=<youtube-url>
// Returns JSON list of available formats with ready-to-use download_url per format.
func (h *APIHandler) GetInfo(c *gin.Context) {
	rawURL := strings.TrimSpace(c.Query("url"))
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing url parameter"})
		return
	}

	if !IsYouTubeURL(rawURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only YouTube URLs are supported"})
		return
	}

	info, err := h.svc.GetFormats(c.Request.Context(), rawURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch video info: " + err.Error()})
		return
	}

	encodedURL := base64.RawURLEncoding.EncodeToString([]byte(rawURL))
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
			DownloadURL:     buildDownloadURL(encodedURL, f, info.Title),
		}
	}

	c.JSON(http.StatusOK, apiInfoResponse{
		Title:     info.Title,
		Thumbnail: info.Thumbnail,
		Duration:  info.Duration,
		Formats:   formats,
	})
}

func buildDownloadURL(encodedURL string, f ytdlp.Format, title string) string {
	q := fmt.Sprintf("/download?url=%s&format_id=%s&ext=%s&title=%s",
		encodedURL,
		url.QueryEscape(f.FormatID),
		url.QueryEscape(f.Ext),
		url.QueryEscape(title),
	)
	if f.NeedsAudioMerge {
		q += "&needs_merge=1"
	}
	return q
}
