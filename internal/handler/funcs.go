package handler

import (
	"fmt"
	"html/template"
	"time"
)

func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatDuration": formatDuration,
		"formatFilesize": formatFilesize,
	}
}

func formatDuration(seconds int) string {
	d := time.Duration(seconds) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func formatFilesize(bytes int64) string {
	if bytes <= 0 {
		return "-"
	}
	const mb = 1024 * 1024
	return fmt.Sprintf("~%.0f MB", float64(bytes)/mb)
}
