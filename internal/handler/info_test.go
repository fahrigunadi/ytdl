package handler_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/fahrigunadi/ytdl/internal/handler"
	"github.com/fahrigunadi/ytdl/internal/ytdlp"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type mockService struct {
	info *ytdlp.VideoInfo
	err  error
}

func (m *mockService) GetFormats(_ context.Context, _ string) (*ytdlp.VideoInfo, error) {
	return m.info, m.err
}

func (m *mockService) Stream(_ context.Context, _, _ string) (io.ReadCloser, error) {
	return nil, nil
}

func setupInfoRouter(svc ytdlp.Downloader) *gin.Engine {
	r := gin.New()
	r.SetFuncMap(handler.TemplateFuncs())
	r.LoadHTMLGlob("../../web/templates/*")
	h := handler.NewInfoHandler(svc)
	r.POST("/info", h.Handle)
	return r
}

func TestInfoHandler_InvalidURL(t *testing.T) {
	r := setupInfoRouter(&mockService{})

	form := url.Values{"url": {"https://twitter.com/video/123"}}
	req := httptest.NewRequest(http.MethodPost, "/info", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Invalid URL") {
		t.Errorf("expected 'Invalid URL' in body, got: %s", w.Body.String())
	}
}

func TestInfoHandler_YtdlpError(t *testing.T) {
	r := setupInfoRouter(&mockService{err: errors.New("video unavailable")})

	form := url.Values{"url": {"https://youtube.com/watch?v=test"}}
	req := httptest.NewRequest(http.MethodPost, "/info", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestInfoHandler_Success(t *testing.T) {
	info := &ytdlp.VideoInfo{
		Title:     "My Video",
		Thumbnail: "https://img.youtube.com/thumb.jpg",
		Duration:  120,
		Formats:   []ytdlp.Format{{FormatID: "18", Ext: "mp4", Resolution: "360p"}},
	}
	r := setupInfoRouter(&mockService{info: info})

	form := url.Values{"url": {"https://youtube.com/watch?v=abc123"}}
	req := httptest.NewRequest(http.MethodPost, "/info", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "My Video") {
		t.Errorf("expected title in body, got: %s", w.Body.String())
	}
}

func TestIsYouTubeURL(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{"https://youtube.com/watch?v=abc", true},
		{"https://www.youtube.com/watch?v=abc", true},
		{"https://youtu.be/abc", true},
		{"https://twitter.com/video", false},
		{"not-a-url", false},
		{"", false},
	}
	for _, tt := range tests {
		got := handler.IsYouTubeURL(tt.raw)
		if got != tt.want {
			t.Errorf("IsYouTubeURL(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}
