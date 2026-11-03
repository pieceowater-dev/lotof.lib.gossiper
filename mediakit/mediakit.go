// Package mediakit is the image/PDF store every tenant service copied: a
// content-sniffing validator, an S3 uploader with namespace-scoped UUID keys,
// and a per-namespace storage quota. goods, issues, menu and plans carried
// identical copies that differed only in the key prefix (audit 2).
package mediakit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

const (
	// MaxUploadBytes caps one uploaded file.
	MaxUploadBytes = 10 << 20
	// s3Timeout bounds one S3 request, on top of the caller's own deadline.
	s3Timeout = 30 * time.Second
	// DefaultRegion is used when Config.Region is empty.
	DefaultRegion = "eu-north-1"
	usageTTL      = 2 * time.Minute
)

var (
	ErrUploadTooLarge   = errors.New("file is too large (limit 10 MB)")
	ErrUploadEmpty      = errors.New("file is empty")
	ErrUnsupportedMedia = errors.New("unsupported file type: only PNG, JPEG, GIF, WebP images and PDF are accepted")
	// ErrStorageQuotaExceeded is returned when an upload would take a
	// namespace past the storage quota of its plan.
	ErrStorageQuotaExceeded = errors.New("storage limit of the plan is reached")
	errNotInitialized       = errors.New("s3 uploader is not initialized")
)

// allowedMedia maps the sniffed content type to the extension the object is stored under.
// SVG and HTML are deliberately absent: they can carry scripts.
var allowedMedia = map[string]string{
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"application/pdf": ".pdf",
}

// DetectMedia decides what a file is from its bytes; the type and the name the client sent are ignored.
func DetectMedia(file []byte) (contentType, ext string, err error) {
	if len(file) == 0 {
		return "", "", ErrUploadEmpty
	}
	if len(file) > MaxUploadBytes {
		return "", "", ErrUploadTooLarge
	}
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(file), ";")[0]))
	ext, ok := allowedMedia[contentType]
	if !ok {
		return "", "", ErrUnsupportedMedia
	}
	return contentType, ext, nil
}

// ServableType is what is reported for a stored object: anything outside the allowlist (an older upload)
// is handed out as opaque bytes, never as something a browser would render.
func ServableType(stored string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(stored, ";")[0]))
	if _, ok := allowedMedia[ct]; ok {
		return ct
	}
	return "application/octet-stream"
}

var fileNameSafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// SanitizeFilename reduces a client-supplied name to a safe base name; "" when nothing usable is left.
func SanitizeFilename(filename string) string {
	base := filepath.Base(strings.TrimSpace(filename))
	if base == "." || base == "/" || base == "" {
		return ""
	}

	base = fileNameSafeChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-._")
	base = strings.ReplaceAll(base, "..", ".")
	if base == "" {
		return ""
	}

	if len(base) > 160 {
		base = base[:160]
	}
	return base
}

// Config selects the bucket and this app's slice of it.
type Config struct {
	Bucket string
	Region string
	// AccessKeyID/SecretAccessKey are optional; without them the default AWS credential chain is used.
	AccessKeyID     string
	SecretAccessKey string
	// Prefix is where this app keeps its files in the shared bucket ("goods"); keys are
	// <Prefix>/<namespace>/<uuid><ext>, and only keys under it are ever served.
	Prefix string
}

type usageEntry struct {
	bytes, files int64
	at           time.Time
}

// Store uploads, serves and measures one app's files. A nil *Store (no bucket configured) is safe:
// every operation returns an "not initialized" error.
type Store struct {
	bucket string
	prefix string
	client *s3.Client

	usageMu sync.Mutex
	usage   map[string]usageEntry
	locks   sync.Map
}

// UploadedImage is the result of a stored upload.
type UploadedImage struct {
	Filename    string
	Key         string
	ContentType string
}

// New opens the S3 client; it returns nil when no bucket is configured or the AWS config cannot be loaded.
func New(cfg Config) *Store {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil
	}

	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = DefaultRegion
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if strings.TrimSpace(cfg.AccessKeyID) != "" && strings.TrimSpace(cfg.SecretAccessKey) != "" {
		provider := credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(cfg.AccessKeyID), strings.TrimSpace(cfg.SecretAccessKey), "")
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(provider))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
	if err != nil {
		return nil
	}

	return &Store{
		bucket: strings.TrimSpace(cfg.Bucket),
		prefix: strings.Trim(strings.TrimSpace(cfg.Prefix), "/"),
		client: s3.NewFromConfig(awsCfg),
		usage:  map[string]usageEntry{},
	}
}

func (s *Store) ready() bool { return s != nil && s.client != nil }

// UploadImage validates the file by its content and stores it under a namespace-scoped, UUID-based key.
func (s *Store) UploadImage(ctx context.Context, namespace, filename string, file []byte) (*UploadedImage, error) {
	if !s.ready() {
		return nil, errNotInitialized
	}
	if strings.TrimSpace(namespace) == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	// The type comes from the bytes, never from the client.
	contentType, ext, err := DetectMedia(file)
	if err != nil {
		return nil, err
	}

	safeFilename := SanitizeFilename(filename)
	if safeFilename == "" {
		safeFilename = fmt.Sprintf("image-%d.bin", time.Now().Unix())
	}

	// Namespace-scoped, UUID-based key: the original filename doesn't leak into the key and different
	// tenants' uploads can never collide or be confused with one another.
	key := fmt.Sprintf("%s/%s/%s%s", s.prefix, strings.TrimSpace(namespace), uuid.NewString(), ext)

	disposition := "inline"
	if contentType == "application/pdf" {
		disposition = "attachment"
	}
	ctx, cancel := context.WithTimeout(ctx, s3Timeout)
	defer cancel()
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:             aws.String(s.bucket),
		Key:                aws.String(key),
		Body:               bytes.NewReader(file),
		ContentType:        aws.String(contentType),
		ContentDisposition: aws.String(disposition),
	})
	if err != nil {
		return nil, err
	}

	return &UploadedImage{Filename: safeFilename, Key: key, ContentType: contentType}, nil
}

// GetImage reads a stored object back. Only keys under this app's own prefix are served, even if a caller bypasses
// the gateway's own check: the bucket is shared with other domains.
func (s *Store) GetImage(ctx context.Context, key string) ([]byte, string, error) {
	if !s.ready() {
		return nil, "", errNotInitialized
	}

	objectKey := strings.TrimSpace(key)
	if objectKey == "" || !strings.HasPrefix(objectKey, s.prefix+"/") || strings.Contains(objectKey, "..") {
		return nil, "", fmt.Errorf("storage key is required")
	}

	ctx, cancel := context.WithTimeout(ctx, s3Timeout)
	defer cancel()
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.ContentLength != nil && *resp.ContentLength > MaxUploadBytes {
		return nil, "", ErrUploadTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxUploadBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > MaxUploadBytes {
		return nil, "", ErrUploadTooLarge
	}

	contentType := ""
	if resp.ContentType != nil {
		contentType = *resp.ContentType
	}
	return data, ServableType(contentType), nil
}

// listUsage sums the sizes of everything stored for a namespace. A listing walks the prefix page by page.
func (s *Store) listUsage(ctx context.Context, namespace string) (int64, int64, error) {
	if !s.ready() {
		return 0, 0, errNotInitialized
	}
	prefix := fmt.Sprintf("%s/%s/", s.prefix, strings.TrimSpace(namespace))
	var bytes, files int64
	var token *string
	for {
		out, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(s.bucket), Prefix: aws.String(prefix), ContinuationToken: token})
		if err != nil {
			return 0, 0, err
		}
		for _, o := range out.Contents {
			if o.Size != nil {
				bytes += *o.Size
			}
			files++
		}
		if out.IsTruncated == nil || !*out.IsTruncated || out.NextContinuationToken == nil {
			return bytes, files, nil
		}
		token = out.NextContinuationToken
	}
}

// Usage reports the bytes and files a namespace currently stores. The listing is cached for a short time and bumped
// locally on every upload.
func (s *Store) Usage(ctx context.Context, namespace string) (int64, int64, error) {
	if s == nil {
		return 0, 0, errNotInitialized
	}
	s.usageMu.Lock()
	if e, ok := s.usage[namespace]; ok && time.Since(e.at) < usageTTL {
		s.usageMu.Unlock()
		return e.bytes, e.files, nil
	}
	s.usageMu.Unlock()
	b, f, err := s.listUsage(ctx, namespace)
	if err != nil {
		return 0, 0, err
	}
	s.usageMu.Lock()
	s.usage[namespace] = usageEntry{bytes: b, files: f, at: time.Now()}
	s.usageMu.Unlock()
	return b, f, nil
}

// CheckQuota rejects an upload of `add` bytes that would pass `limit` (0 = unlimited). A failure to measure never
// blocks an upload.
func (s *Store) CheckQuota(ctx context.Context, namespace string, limit, add int64) error {
	if limit <= 0 {
		return nil
	}
	used, _, err := s.Usage(ctx, namespace)
	if err != nil {
		return nil
	}
	if used+add > limit {
		return fmt.Errorf("%w (%d of %d MB used)", ErrStorageQuotaExceeded, used/1048576, limit/1048576)
	}
	return nil
}

// NoteUploaded keeps the cached usage honest between listings.
func (s *Store) NoteUploaded(namespace string, size int64) {
	if s == nil {
		return
	}
	s.usageMu.Lock()
	defer s.usageMu.Unlock()
	if e, ok := s.usage[namespace]; ok {
		e.bytes += size
		e.files++
		s.usage[namespace] = e
	}
}

// UploadWithQuota checks the plan's storage quota and stores the file as one step per namespace, so parallel
// uploads cannot all pass the check against the same used size. It returns the key and the detected type.
func (s *Store) UploadWithQuota(ctx context.Context, namespace, filename string, file []byte, limit int64) (key, contentType string, err error) {
	if !s.ready() {
		return "", "", errNotInitialized
	}
	l, _ := s.locks.LoadOrStore(namespace, &sync.Mutex{})
	mu := l.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	if err := s.CheckQuota(ctx, namespace, limit, int64(len(file))); err != nil {
		return "", "", err
	}
	img, err := s.UploadImage(ctx, namespace, filename, file)
	if err != nil {
		return "", "", err
	}
	s.NoteUploaded(namespace, int64(len(file)))
	return img.Key, img.ContentType, nil
}
