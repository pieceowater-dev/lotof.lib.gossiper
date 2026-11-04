package mediakit

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func testStore() *Store {
	return New(Config{Bucket: "b", Region: "eu-north-1", AccessKeyID: "AKIATEST", SecretAccessKey: "secret", Prefix: "goods"})
}

func TestPresignUpload(t *testing.T) {
	s := testStore()
	p, err := s.PresignUpload(context.Background(), "ns1", "a.png", "image/png", 1000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Key, "goods/ns1/") || !strings.HasSuffix(p.Key, ".png") {
		t.Fatalf("key %q", p.Key)
	}
	if !strings.Contains(p.URL, "X-Amz-Signature") || p.Headers["Content-Type"] != "image/png" {
		t.Fatalf("bad presign: %+v", p)
	}
	for _, c := range []struct {
		ct   string
		size int64
		want error
	}{
		{"image/svg+xml", 10, ErrUnsupportedMedia},
		{"image/png", 0, ErrUploadEmpty},
		{"image/png", MaxUploadBytes + 1, ErrUploadTooLarge},
	} {
		if _, err := s.PresignUpload(context.Background(), "ns1", "a", c.ct, c.size, 0); !errors.Is(err, c.want) {
			t.Fatalf("%s/%d: got %v want %v", c.ct, c.size, err, c.want)
		}
	}
}

func TestPresignDownload(t *testing.T) {
	s := testStore()
	u, err := s.PresignDownload(context.Background(), "goods/ns1/x.png")
	if err != nil || !strings.Contains(u, "X-Amz-Signature") || !strings.Contains(u, "inline") {
		t.Fatalf("%v %q", err, u)
	}
	u, _ = s.PresignDownload(context.Background(), "goods/ns1/x.pdf")
	if !strings.Contains(u, "attachment") {
		t.Fatalf("pdf must download: %q", u)
	}
	for _, k := range []string{"", "menu/ns1/x.png", "goods/../menu/x.png"} {
		if _, err := s.PresignDownload(context.Background(), k); err == nil {
			t.Fatalf("key %q must be refused", k)
		}
	}
}
