package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/fahrigunadi/ytdl/internal/handler"
	"github.com/fahrigunadi/ytdl/internal/ytdlp"
)

func setupAPIRouter(svc ytdlp.Downloader) *gin.Engine {
	r := gin.New()
	h := handler.NewAPIHandler(svc)
	r.GET("/api/info", h.GetInfo)
	return r
}

func TestAPIGetInfo_MissingURL(t *testing.T) {
	r := setupAPIRouter(&mockService{})
	req := httptest.NewRequest(http.MethodGet, "/api/info", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAPIGetInfo_NonYouTubeURL(t *testing.T) {
	r := setupAPIRouter(&mockService{})
	req := httptest.NewRequest(http.MethodGet, "/api/info?url=https://twitter.com/video/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAPIGetInfo_Success(t *testing.T) {
	info := &ytdlp.VideoInfo{
		Title:     "My Video",
		Thumbnail: "https://img.youtube.com/thumb.jpg",
		Duration:  120,
		Formats: []ytdlp.Format{
			{FormatID: "18", Ext: "mp4", Resolution: "360p", Filesize: 50000000},
			{FormatID: "137", Ext: "mkv", Resolution: "1080p", Filesize: 200000000, NeedsAudioMerge: true},
		},
	}
	r := setupAPIRouter(&mockService{info: info})
	req := httptest.NewRequest(http.MethodGet, "/api/info?url=https://youtube.com/watch?v=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Title   string `json:"title"`
		Formats []struct {
			FormatID    string `json:"format_id"`
			DownloadURL string `json:"download_url"`
		} `json:"formats"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Title != "My Video" {
		t.Errorf("expected title 'My Video', got %q", resp.Title)
	}
	if len(resp.Formats) != 2 {
		t.Fatalf("expected 2 formats, got %d", len(resp.Formats))
	}

	// format 18: no needs_merge
	dl0 := resp.Formats[0].DownloadURL
	if !strings.Contains(dl0, "format_id=18") || !strings.Contains(dl0, "ext=mp4") {
		t.Errorf("unexpected download_url for format 18: %s", dl0)
	}
	if strings.Contains(dl0, "needs_merge") {
		t.Errorf("format 18 should not have needs_merge in download_url")
	}

	// format 137: needs_merge=1
	dl1 := resp.Formats[1].DownloadURL
	if !strings.Contains(dl1, "needs_merge=1") {
		t.Errorf("format 137 should have needs_merge=1 in download_url, got: %s", dl1)
	}
}
