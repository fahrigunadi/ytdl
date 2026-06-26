package handler_test

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gunadi/ytdl/internal/handler"
	"github.com/gunadi/ytdl/internal/ytdlp"
)

type mockStreamService struct {
	reader io.ReadCloser
	err    error
}

func (m *mockStreamService) GetFormats(_ context.Context, _ string) (*ytdlp.VideoInfo, error) {
	return nil, nil
}

func (m *mockStreamService) Stream(_ context.Context, _, _ string) (io.ReadCloser, error) {
	return m.reader, m.err
}

func setupDownloadRouter(svc ytdlp.Downloader) *gin.Engine {
	r := gin.New()
	h := handler.NewDownloadHandler(svc)
	r.GET("/download", h.Handle)
	return r
}

func TestDownloadHandler_MissingParams(t *testing.T) {
	r := setupDownloadRouter(&mockStreamService{})
	req := httptest.NewRequest(http.MethodGet, "/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func b64(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func TestDownloadHandler_InvalidURL(t *testing.T) {
	r := setupDownloadRouter(&mockStreamService{})
	req := httptest.NewRequest(http.MethodGet,
		"/download?url="+b64("https://twitter.com/v/1")+"&format_id=18&ext=mp4&title=test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDownloadHandler_StreamsBody(t *testing.T) {
	content := "fake video bytes"
	svc := &mockStreamService{reader: io.NopCloser(strings.NewReader(content))}
	r := setupDownloadRouter(svc)

	req := httptest.NewRequest(http.MethodGet,
		"/download?url="+b64("https://youtube.com/watch?v=abc")+"&format_id=18&ext=mp4&title=My+Video", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("expected attachment header, got: %s", w.Header().Get("Content-Disposition"))
	}
	if w.Body.String() != content {
		t.Errorf("expected body %q, got %q", content, w.Body.String())
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Video", "My Video"},
		{"Video: Part 1", "Video- Part 1"},
		{"A/B\\C", "A-B-C"},
		{"", "video"},
		{"  spaces  ", "spaces"},
	}
	for _, tt := range tests {
		got := handler.SanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
