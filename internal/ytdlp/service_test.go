package ytdlp

import (
	"testing"
)

func TestParseVideoInfo_FiltersVideoOnlyFormats(t *testing.T) {
	raw := &rawVideoInfo{
		Title:     "Test Video",
		Thumbnail: "https://img.youtube.com/thumb.jpg",
		Duration:  180,
		Formats: []rawFormat{
			{FormatID: "137", Ext: "mp4", Height: 1080, VCodec: "avc1", ACodec: "none"},
			{FormatID: "18", Ext: "mp4", Height: 360, VCodec: "avc1", ACodec: "mp4a"},
			{FormatID: "140", Ext: "m4a", VCodec: "none", ACodec: "mp4a", ABR: 128},
		},
	}

	info := parseVideoInfo(raw)

	if info.Title != "Test Video" {
		t.Errorf("expected title %q, got %q", "Test Video", info.Title)
	}
	if len(info.Formats) != 2 {
		t.Fatalf("expected 2 formats (video-only 137 filtered), got %d", len(info.Formats))
	}
	if info.Formats[0].FormatID != "18" {
		t.Errorf("expected format 18, got %s", info.Formats[0].FormatID)
	}
	if info.Formats[1].FormatID != "140" {
		t.Errorf("expected format 140, got %s", info.Formats[1].FormatID)
	}
	if !info.Formats[1].IsAudioOnly {
		t.Error("format 140 should be IsAudioOnly=true")
	}
	if info.Formats[0].IsAudioOnly {
		t.Error("format 18 should be IsAudioOnly=false")
	}
}

func TestResolutionLabel_Video(t *testing.T) {
	got := resolutionLabel(rawFormat{Height: 720, VCodec: "avc1"}, false)
	if got != "720p" {
		t.Errorf("expected %q, got %q", "720p", got)
	}
}

func TestResolutionLabel_AudioWithBitrate(t *testing.T) {
	got := resolutionLabel(rawFormat{ABR: 128}, true)
	if got != "128kbps" {
		t.Errorf("expected %q, got %q", "128kbps", got)
	}
}

func TestResolutionLabel_AudioNoBitrate(t *testing.T) {
	got := resolutionLabel(rawFormat{}, true)
	if got != "audio" {
		t.Errorf("expected %q, got %q", "audio", got)
	}
}

func TestResolutionLabel_UnknownVideo(t *testing.T) {
	got := resolutionLabel(rawFormat{VCodec: "avc1"}, false)
	if got != "unknown" {
		t.Errorf("expected %q, got %q", "unknown", got)
	}
}
