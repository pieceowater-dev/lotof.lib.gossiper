package mediakit

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

const (
	// PresignPutTTL is how long a browser has to start the direct upload.
	PresignPutTTL = 10 * time.Minute
	// PresignGetTTL is how long a download address stays valid.
	PresignGetTTL = 15 * time.Minute
	// sniffBytes is how much of an uploaded object is read to confirm its real type.
	sniffBytes = 512
)

// PresignedUpload is what a browser needs to put a file straight into S3: it sends Body with the headers below
// to URL, then calls ConfirmUpload with Key.
type PresignedUpload struct {
	URL         string
	Key         string
	ContentType string
	// Headers must be sent with the PUT exactly as given (they are part of the signature).
	Headers map[string]string
	Expires time.Time
}

func (s *Store) presignClient() *s3.PresignClient { return s3.NewPresignClient(s.client) }

// PresignUpload checks the plan's quota against the declared size and returns a short-lived address for a direct
// PUT. The declared size and type are signed, so S3 refuses a body of another length or type. The real type is
// checked from the stored bytes in ConfirmUpload.
func (s *Store) PresignUpload(ctx context.Context, namespace, filename, contentType string, size, limit int64) (*PresignedUpload, error) {
	if !s.ready() {
		return nil, errNotInitialized
	}
	if size <= 0 {
		return nil, ErrUploadEmpty
	}
	if size > MaxUploadBytes {
		return nil, ErrUploadTooLarge
	}
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	ext, ok := allowedMedia[ct]
	if !ok {
		return nil, ErrUnsupportedMedia
	}
	if err := s.CheckQuota(ctx, namespace, limit, size); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s/%s/%s%s", s.prefix, strings.TrimSpace(namespace), uuid.NewString(), ext)
	disposition := "inline"
	if ct == "application/pdf" {
		disposition = "attachment"
	}
	req, err := s.presignClient().PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:             aws.String(s.bucket),
		Key:                aws.String(key),
		ContentType:        aws.String(ct),
		ContentLength:      aws.Int64(size),
		ContentDisposition: aws.String(disposition),
	}, s3.WithPresignExpires(PresignPutTTL))
	if err != nil {
		return nil, err
	}
	headers := map[string]string{"Content-Type": ct, "Content-Disposition": disposition}
	return &PresignedUpload{URL: req.URL, Key: key, ContentType: ct, Headers: headers, Expires: time.Now().Add(PresignPutTTL)}, nil
}

// ConfirmUpload verifies a finished direct upload: the key must be this namespace's, the object must be within the
// size cap, and its first bytes must be an allowed type matching the key. A bad object is deleted. It returns the
// stored size and type and counts the file towards the namespace's usage.
func (s *Store) ConfirmUpload(ctx context.Context, namespace, key string) (size int64, contentType string, err error) {
	if !s.ready() {
		return 0, "", errNotInitialized
	}
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(key, fmt.Sprintf("%s/%s/", s.prefix, strings.TrimSpace(namespace))) || strings.Contains(key, "..") {
		return 0, "", fmt.Errorf("storage key is not allowed")
	}
	ctx, cancel := context.WithTimeout(ctx, s3Timeout)
	defer cancel()
	reject := func(e error) (int64, string, error) {
		_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
		return 0, "", e
	}
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return 0, "", fmt.Errorf("uploaded file not found: %w", err)
	}
	if head.ContentLength == nil || *head.ContentLength <= 0 {
		return reject(ErrUploadEmpty)
	}
	if *head.ContentLength > MaxUploadBytes {
		return reject(ErrUploadTooLarge)
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), Range: aws.String(fmt.Sprintf("bytes=0-%d", sniffBytes-1)),
	})
	if err != nil {
		return 0, "", err
	}
	defer out.Body.Close()
	head512, err := io.ReadAll(io.LimitReader(out.Body, sniffBytes))
	if err != nil {
		return 0, "", err
	}
	ct, ext, err := DetectMedia(head512)
	if err != nil {
		return reject(err)
	}
	if !strings.HasSuffix(key, ext) {
		return reject(ErrUnsupportedMedia)
	}
	s.NoteUploaded(namespace, *head.ContentLength)
	return *head.ContentLength, ct, nil
}

// PresignDownload returns a short-lived address that serves a stored object straight from S3. Only keys under
// this app's prefix are signed. Documents are forced to download and everything is served with its allowlisted type.
func (s *Store) PresignDownload(ctx context.Context, key string) (string, error) {
	if !s.ready() {
		return "", errNotInitialized
	}
	objectKey := strings.TrimSpace(key)
	if objectKey == "" || !strings.HasPrefix(objectKey, s.prefix+"/") || strings.Contains(objectKey, "..") {
		return "", fmt.Errorf("storage key is required")
	}
	ct := "application/octet-stream"
	for t, ext := range allowedMedia {
		if strings.HasSuffix(objectKey, ext) {
			ct = t
			break
		}
	}
	disposition := "attachment"
	switch ct {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		disposition = "inline"
	}
	req, err := s.presignClient().PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(objectKey),
		ResponseContentType:        aws.String(ct),
		ResponseContentDisposition: aws.String(disposition),
	}, s3.WithPresignExpires(PresignGetTTL))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}
