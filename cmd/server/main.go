package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/fahrigunadi/ytdl/internal/handler"
	"github.com/fahrigunadi/ytdl/internal/middleware"
	"github.com/fahrigunadi/ytdl/internal/ytdlp"
)

func main() {
	svc, err := ytdlp.New()
	if err != nil {
		log.Fatalf("startup: %v", err)
	}

	r := gin.Default()
	r.SetFuncMap(handler.TemplateFuncs())
	r.LoadHTMLGlob("web/templates/*")
	r.Static("/static", "web/static")

	infoHandler := handler.NewInfoHandler(svc)
	downloadHandler := handler.NewDownloadHandler(svc)
	apiHandler := handler.NewAPIHandler(svc)

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
	r.POST("/info", middleware.Timeout(30*time.Second), infoHandler.Handle)
	r.GET("/download", downloadHandler.Handle)

	api := r.Group("/api")
	api.GET("/info", middleware.Timeout(30*time.Second), apiHandler.GetInfo)

	log.Println("listening on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("server: %v", err)
	}
}
