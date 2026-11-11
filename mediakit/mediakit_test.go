package mediakit

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// nonNilClient is a real, non-network-touching *s3.Client used only to get past the nil-guard so the
// argument-validation branches below it can be exercised. Constructing one never dials out.
func nonNilClient() *s3.Client { return s3.New(s3.Options{}) }

func newStore(prefix string) *Store {
	return &Store{bucket: "b", prefix: prefix, client: nonNilClient(), usage: map[string]usageEntry{}}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"simple filename unchanged", "photo.jpg", "photo.jpg"},
		{"empty string", "", ""},
		{"dot only", ".", ""},
		{"slash only", "/", ""},
		{"path traversal reduced to base name", "../../etc/passwd", "passwd"},
		{"nested path reduced to base name", "a/b/c/photo.jpg", "photo.jpg"},
		{"special chars replaced with dash", "my photo!@#.jpg", "my-photo-.jpg"},
		{"leading/trailing dashes dots underscores trimmed", "--photo--", "photo"},
		{"consecutive dots partially collapsed (single non-recursive pass)", "photo....jpg", "photo..jpg"},
		{"very long filename truncated to 160 chars", strings.Repeat("a", 300) + ".jpg", strings.Repeat("a", 160)},
		{"whitespace-only trims to empty", "   ", ""},
		{"non-ASCII letters replaced with dash, not stripped", "café.jpg", "caf-.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeFilename(tt.input); got != tt.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestUploadImage_GuardClauses(t *testing.T) {
	var nilStore *Store
	if _, err := nilStore.UploadImage(context.Background(), "ns", "f.jpg", []byte("x")); err == nil {
		t.Fatal("expected error for nil store")
	}
	if _, err := (&Store{bucket: "b"}).UploadImage(context.Background(), "ns", "f.jpg", []byte("x")); err == nil {
		t.Fatal("expected error for nil client")
	}
	if _, err := newStore("goods").UploadImage(context.Background(), "   ", "f.jpg", []byte("x")); err == nil {
		t.Fatal("expected error for blank namespace")
	}
	if _, err := newStore("goods").UploadImage(context.Background(), "ns", "f.svg", []byte("<svg/>")); !errors.Is(err, ErrUnsupportedMedia) {
		t.Fatalf("content must be sniffed before anything is sent, got %v", err)
	}
}

func TestGetImage_GuardClauses(t *testing.T) {
	var nilStore *Store
	tests := []struct {
		name string
		s    *Store
		key  string
	}{
		{"nil store rejected", nilStore, "goods/ns/x.jpg"},
		{"nil client rejected", &Store{bucket: "b", prefix: "goods"}, "goods/ns/x.jpg"},
		{"blank key rejected", newStore("goods"), ""},
		{"whitespace-only key rejected", newStore("goods"), "   "},
		{"foreign prefix rejected", newStore("goods"), "other/ns/x.jpg"},
		{"prefix must end at a slash", newStore("goods"), "goodsx/ns/x.jpg"},
		{"path traversal rejected", newStore("goods"), "goods/../secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := tt.s.GetImage(context.Background(), tt.key); err == nil {
				t.Errorf("GetImage(%q) must fail", tt.key)
			}
		})
	}
}

func TestDetectMedia(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	jpg := []byte("\xff\xd8\xff\xe0\x00\x10JFIF")
	gif := []byte("GIF89a\x01\x00\x01\x00")
	webp := append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), make([]byte, 16)...)
	pdf := []byte("%PDF-1.7\n")
	tests := []struct {
		name    string
		file    []byte
		wantCT  string
		wantExt string
		wantErr error
	}{
		{"png", png, "image/png", ".png", nil},
		{"jpeg", jpg, "image/jpeg", ".jpg", nil},
		{"gif", gif, "image/gif", ".gif", nil},
		{"webp", webp, "image/webp", ".webp", nil},
		{"pdf", pdf, "application/pdf", ".pdf", nil},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), "", "", ErrUnsupportedMedia},
		{"svg with xml header", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`), "", "", ErrUnsupportedMedia},
		{"html", []byte("<!DOCTYPE html><html><script>alert(1)</script></html>"), "", "", ErrUnsupportedMedia},
		{"script text", []byte("alert(1)"), "", "", ErrUnsupportedMedia},
		{"empty", nil, "", "", ErrUploadEmpty},
		{"too large", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxUploadBytes)...), "", "", ErrUploadTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, ext, err := DetectMedia(tt.file)
			if !errors.Is(err, tt.wantErr) || ct != tt.wantCT || ext != tt.wantExt {
				t.Fatalf("got (%q, %q, %v), want (%q, %q, %v)", ct, ext, err, tt.wantCT, tt.wantExt, tt.wantErr)
			}
		})
	}
	if _, _, err := DetectMedia(bytes.Repeat([]byte("a"), 10)); err == nil {
		t.Fatal("plain text must be refused")
	}
}

func TestServableType(t *testing.T) {
	for in, want := range map[string]string{
		"image/png":             "image/png",
		"IMAGE/JPEG; charset=x": "image/jpeg",
		"application/pdf":       "application/pdf",
		"image/svg+xml":         "application/octet-stream",
		"text/html":             "application/octet-stream",
		"":                      "application/octet-stream",
	} {
		if got := ServableType(in); got != want {
			t.Errorf("ServableType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheckQuota(t *testing.T) {
	s := newStore("goods")
	ctx := context.Background()
	s.usage["acme"] = usageEntry{bytes: 90 << 20, files: 3, at: time.Now()}

	if err := s.CheckQuota(ctx, "acme", 0, 1<<30); err != nil {
		t.Errorf("no limit means no check: %v", err)
	}
	if err := s.CheckQuota(ctx, "acme", 100<<20, 5<<20); err != nil {
		t.Errorf("within the limit: %v", err)
	}
	if err := s.CheckQuota(ctx, "acme", 100<<20, 20<<20); !errors.Is(err, ErrStorageQuotaExceeded) {
		t.Errorf("over the limit must be rejected, got %v", err)
	}
	s.NoteUploaded("acme", 8<<20)
	if err := s.CheckQuota(ctx, "acme", 100<<20, 5<<20); !errors.Is(err, ErrStorageQuotaExceeded) {
		t.Errorf("an upload counted since the listing must count, got %v", err)
	}
}

func TestNewWithoutBucketIsNil(t *testing.T) {
	if New(Config{Prefix: "goods"}) != nil {
		t.Fatal("no bucket means no store")
	}
}

func TestVideoIsAcceptedOnlyWhenAsked(t *testing.T) {
	mp4 := append([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'}, make([]byte, 64)...)
	if _, _, err := DetectMedia(mp4); err == nil {
		t.Fatal("a clip must be refused unless the app opts in")
	}
	ct, ext, err := detectMedia(mp4, true)
	if err != nil || ct != "video/mp4" || ext != ".mp4" {
		t.Fatalf("mp4: %q %q %v", ct, ext, err)
	}
	webm := append([]byte{0x1A, 0x45, 0xDF, 0xA3}, make([]byte, 64)...)
	if ct, _, err := detectMedia(webm, true); err != nil || ct != "video/webm" {
		t.Fatalf("webm: %q %v", ct, err)
	}
	if !IsVideoType("video/mp4; codecs=avc1") || IsVideoType("image/png") {
		t.Error("IsVideoType")
	}
}
