package ytdlp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

type VideoInfo struct {
	Title     string
	Thumbnail string
	Duration  int
	Formats   []Format
}

type Format struct {
	FormatID    string
	Ext         string
	Resolution  string
	Filesize    int64
	VCodec      string
	ACodec      string
	ABitrate    float64
	IsAudioOnly bool
}

type rawFormat struct {
	FormatID string  `json:"format_id"`
	Ext      string  `json:"ext"`
	Height   int     `json:"height"`
	Filesize int64   `json:"filesize"`
	VCodec   string  `json:"vcodec"`
	ACodec   string  `json:"acodec"`
	ABR      float64 `json:"abr"`
}

type rawVideoInfo struct {
	Title     string      `json:"title"`
	Thumbnail string      `json:"thumbnail"`
	Duration  int         `json:"duration"`
	Formats   []rawFormat `json:"formats"`
}

// Downloader is the interface handlers depend on — enables mock injection in tests.
type Downloader interface {
	GetFormats(ctx context.Context, url string) (*VideoInfo, error)
	Stream(ctx context.Context, url, formatID string) (io.ReadCloser, error)
}

type Service struct {
	ytdlpPath string
}

func New() (*Service, error) {
	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		return nil, fmt.Errorf("yt-dlp not found in PATH: %w", err)
	}
	return &Service{ytdlpPath: path}, nil
}

func (s *Service) GetFormats(ctx context.Context, url string) (*VideoInfo, error) {
	cmd := exec.CommandContext(ctx, s.ytdlpPath,
		"--dump-single-json",
		"--no-playlist",
		"--no-warnings",
		url,
	)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp: %s", string(exitErr.Stderr))
		}
		return nil, err
	}
	var raw rawVideoInfo
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return parseVideoInfo(&raw), nil
}

func (s *Service) Stream(ctx context.Context, url, formatID string) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, s.ytdlpPath,
		"-f", formatID,
		"-o", "-",
		"--no-playlist",
		"--no-warnings",
		url,
	)
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return r, nil
}

func parseVideoInfo(raw *rawVideoInfo) *VideoInfo {
	info := &VideoInfo{
		Title:     raw.Title,
		Thumbnail: raw.Thumbnail,
		Duration:  raw.Duration,
	}
	for _, f := range raw.Formats {
		isAudioOnly := f.VCodec == "none" && f.ACodec != "none"
		isVideoAudio := f.VCodec != "none" && f.ACodec != "none"
		if !isAudioOnly && !isVideoAudio {
			continue // skip video-only streams (require ffmpeg merge — out of scope)
		}
		info.Formats = append(info.Formats, Format{
			FormatID:    f.FormatID,
			Ext:         f.Ext,
			Resolution:  resolutionLabel(f, isAudioOnly),
			Filesize:    f.Filesize,
			VCodec:      f.VCodec,
			ACodec:      f.ACodec,
			ABitrate:    f.ABR,
			IsAudioOnly: isAudioOnly,
		})
	}
	return info
}

func resolutionLabel(f rawFormat, isAudioOnly bool) string {
	if isAudioOnly {
		if f.ABR > 0 {
			return fmt.Sprintf("%.0fkbps", f.ABR)
		}
		return "audio"
	}
	if f.Height > 0 {
		return fmt.Sprintf("%dp", f.Height)
	}
	return "unknown"
}
