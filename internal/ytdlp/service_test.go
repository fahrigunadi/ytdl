package ytdlp

import (
	"testing"
)

func TestParseVideoInfo_IncludesVideoOnlyFormats(t *testing.T) {
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
	if len(info.Formats) != 3 {
		t.Fatalf("expected 3 formats (137 now included), got %d", len(info.Formats))
	}

	f137 := info.Formats[0]
	if f137.FormatID != "137" {
		t.Errorf("expected format 137, got %s", f137.FormatID)
	}
	if !f137.NeedsAudioMerge {
		t.Error("format 137 should be NeedsAudioMerge=true")
	}
	if f137.Ext != "mkv" {
		t.Errorf("video-only format ext should be mkv, got %s", f137.Ext)
	}

	f18 := info.Formats[1]
	if f18.FormatID != "18" {
		t.Errorf("expected format 18, got %s", f18.FormatID)
	}
	if f18.IsAudioOnly || f18.NeedsAudioMerge {
		t.Error("format 18 should be regular video+audio")
	}

	f140 := info.Formats[2]
	if f140.FormatID != "140" {
		t.Errorf("expected format 140, got %s", f140.FormatID)
	}
	if !f140.IsAudioOnly {
		t.Error("format 140 should be IsAudioOnly=true")
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
