package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestAPIGetInfo_InvalidBase64(t *testing.T) {
	r := setupAPIRouter(&mockService{})
	req := httptest.NewRequest(http.MethodGet, "/api/info?url=!!!invalid!!!", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAPIGetInfo_NonYouTubeURL(t *testing.T) {
	r := setupAPIRouter(&mockService{})
	req := httptest.NewRequest(http.MethodGet, "/api/info?url="+b64("https://twitter.com/video/1"), nil)
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
		},
	}
	r := setupAPIRouter(&mockService{info: info})
	req := httptest.NewRequest(http.MethodGet, "/api/info?url="+b64("https://youtube.com/watch?v=abc"), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Title   string `json:"title"`
		Formats []struct {
			FormatID string `json:"format_id"`
		} `json:"formats"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Title != "My Video" {
		t.Errorf("expected title 'My Video', got %q", resp.Title)
	}
	if len(resp.Formats) != 1 || resp.Formats[0].FormatID != "18" {
		t.Errorf("unexpected formats: %+v", resp.Formats)
	}
}
