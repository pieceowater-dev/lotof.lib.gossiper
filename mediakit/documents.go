package mediakit

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// docTypes are the office and text documents a comment may carry besides images and PDF, by extension.
// They are never rendered by a browser from our domain: they are stored as attachments and handed out as opaque bytes.
var docTypes = map[string]string{
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".txt":  "text/plain",
	".csv":  "text/csv",
}

var (
	zipMagic = []byte("PK\x03\x04")
	oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
)

// DocumentType is the content type for a document file name, "" when the extension is not an allowed document.
func DocumentType(filename string) string {
	return docTypes[strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))]
}

// IsDocumentType reports whether ct is one of the allowed document content types.
func IsDocumentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	for _, v := range docTypes {
		if v == ct {
			return true
		}
	}
	return false
}

// DetectDocument decides whether file really is the document its extension claims: Office Open XML must be a zip,
// the old Office formats an OLE container, text and CSV must be text without NUL bytes.
func DetectDocument(filename string, file []byte) (contentType, ext string, err error) {
	if len(file) == 0 {
		return "", "", ErrUploadEmpty
	}
	if len(file) > MaxUploadBytes {
		return "", "", ErrUploadTooLarge
	}
	ext = strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	contentType, ok := docTypes[ext]
	if !ok {
		return "", "", ErrUnsupportedMedia
	}
	switch ext {
	case ".docx", ".xlsx":
		if !bytes.HasPrefix(file, zipMagic) {
			return "", "", ErrUnsupportedMedia
		}
	case ".doc", ".xls":
		if !bytes.HasPrefix(file, oleMagic) {
			return "", "", ErrUnsupportedMedia
		}
	default:
		head := file
		if len(head) > 8192 {
			head = head[:8192]
		}
		if bytes.IndexByte(head, 0) >= 0 {
			return "", "", ErrUnsupportedMedia
		}
	}
	return contentType, ext, nil
}

// UploadDocumentWithQuota stores a document (see DetectDocument) as a download-only object under the namespace's
// prefix, counted against the plan's storage quota under the same per-namespace lock as images.
func (s *Store) UploadDocumentWithQuota(ctx context.Context, namespace, filename string, file []byte, limit int64) (key, contentType string, err error) {
	if !s.ready() {
		return "", "", errNotInitialized
	}
	if strings.TrimSpace(namespace) == "" {
		return "", "", fmt.Errorf("namespace is required")
	}
	contentType, ext, err := DetectDocument(filename, file)
	if err != nil {
		return "", "", err
	}
	l, _ := s.locks.LoadOrStore(namespace, &sync.Mutex{})
	mu := l.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	if err := s.CheckQuota(ctx, namespace, limit, int64(len(file))); err != nil {
		return "", "", err
	}
	key = fmt.Sprintf("%s/%s/%s%s", s.prefix, strings.TrimSpace(namespace), uuid.NewString(), ext)
	ctx, cancel := context.WithTimeout(ctx, s3Timeout)
	defer cancel()
	if _, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:             aws.String(s.bucket),
		Key:                aws.String(key),
		Body:               bytes.NewReader(file),
		ContentType:        aws.String("application/octet-stream"),
		ContentDisposition: aws.String("attachment"),
	}); err != nil {
		return "", "", err
	}
	s.NoteUploaded(namespace, int64(len(file)))
	return key, contentType, nil
}
