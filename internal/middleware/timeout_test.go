package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gunadi/ytdl/internal/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestTimeout_SetsDeadline(t *testing.T) {
	var hasDeadline bool

	r := gin.New()
	r.Use(middleware.Timeout(5 * time.Second))
	r.GET("/test", func(c *gin.Context) {
		_, hasDeadline = c.Request.Context().Deadline()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !hasDeadline {
		t.Error("expected context deadline, got none")
	}
}

func TestTimeout_CancelsSlowHandler(t *testing.T) {
	r := gin.New()
	r.Use(middleware.Timeout(50 * time.Millisecond))
	r.GET("/slow", func(c *gin.Context) {
		select {
		case <-time.After(200 * time.Millisecond):
			c.Status(http.StatusOK)
		case <-c.Request.Context().Done():
			c.Status(http.StatusGatewayTimeout)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("expected 504, got %d", w.Code)
	}
}
