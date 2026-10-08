// Package storage generates presigned upload tickets against S3-compatible
// object storage. NewAvatarStore's client is configured with path-style
// addressing and an explicit BaseEndpoint so the same code works against
// AWS S3, Cloudflare R2, and a local MinIO without branching.
package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// maxAvatarBytes caps an avatar upload at 50 MiB: a safety ceiling far above
// any phone photo, so in practice people can pick any photo (the developer
// asked for no practical limit, 2026-10-08). This bound is passed into the
// presigned request itself (ContentLength), not merely checked in Go, so the
// storage provider enforces it even if the uploading client ignores what it
// was handed.
const maxAvatarBytes = 50 * 1024 * 1024

// presignExpiry is how long a presigned avatar-upload URL remains valid.
const presignExpiry = 300 * time.Second

var (
	// ErrUnsupportedContentType is returned for any content type outside
	// allowedAvatarContentTypes.
	ErrUnsupportedContentType = errors.New("storage: unsupported avatar content type")
	// ErrContentTooLarge is returned when contentLength exceeds maxAvatarBytes.
	ErrContentTooLarge = errors.New("storage: avatar content exceeds the size limit")
	// ErrIncompleteConfig is returned by NewAvatarStore when a required S3
	// config field is empty.
	ErrIncompleteConfig = errors.New("storage: incomplete S3 configuration")
)

// allowedAvatarContentTypes maps the common image content types to the file
// extension used in the object key. Any other image type is accepted too
// (see avatarExtension); only things that are not images are refused.
var allowedAvatarContentTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/gif":  "gif",
	"image/heic": "heic",
	"image/heif": "heif",
	"image/avif": "avif",
}

// avatarExtension returns the object-key extension for an image content type.
// A type outside the map is accepted when it is image/ followed by up to ten
// lowercase letters or digits, and that subtype becomes the extension; anything
// else (a PDF, a path, an odd character) is refused so nothing unexpected ever
// lands in an object key.
func avatarExtension(contentType string) (string, bool) {
	if ext, ok := allowedAvatarContentTypes[contentType]; ok {
		return ext, true
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", false
	}
	sub := contentType[len("image/"):]
	if sub == "" || len(sub) > 10 {
		return "", false
	}
	for _, r := range sub {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return "", false
		}
	}
	return sub, true
}

// UploadTicket is what a client receives to perform a direct-to-storage
// avatar upload: a presigned PUT URL to upload to, and the public URL the
// object will have once uploaded. The client stores PublicURL back through
// PATCH /me once the upload succeeds.
type UploadTicket struct {
	UploadURL string
	PublicURL string
	ExpiresIn int
}

// AvatarStore issues presigned avatar-upload tickets.
type AvatarStore interface {
	PresignAvatarUpload(ctx context.Context, userID uuid.UUID, contentType string, contentLength int64) (*UploadTicket, error)
}

// Config holds the subset of application configuration NewAvatarStore
// needs. Field names deliberately mirror internal/config.Config's S3
// fields one-to-one so a caller can pass them through directly.
type Config struct {
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3PublicBaseURL   string
}

// presigner is the seam s3_test.go substitutes with a stub, so no test in
// this package reaches a real S3-compatible endpoint over the network.
// Real-bucket verification is deferred to a later phase-1 checkpoint (see
// SUMMARY.md) since S3 credentials are not yet provisioned in this
// environment.
type presigner interface {
	PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// s3AvatarStore is the AvatarStore implementation backed by an S3-compatible
// presign client.
type s3AvatarStore struct {
	presigner     presigner
	bucket        string
	publicBaseURL string
}

// NewAvatarStore builds an AvatarStore backed by aws-sdk-go-v2, using
// static credentials, the configured region, BaseEndpoint set to the
// configured endpoint, and path-style addressing.
func NewAvatarStore(cfg Config) (AvatarStore, error) {
	if cfg.S3Endpoint == "" || cfg.S3Bucket == "" || cfg.S3AccessKeyID == "" || cfg.S3SecretAccessKey == "" || cfg.S3PublicBaseURL == "" {
		return nil, ErrIncompleteConfig
	}
	region := cfg.S3Region
	if region == "" {
		region = "auto"
	}

	client := s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(cfg.S3Endpoint),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, ""),
	})
	presignClient := s3.NewPresignClient(client)

	return &s3AvatarStore{
		presigner:     presignClient,
		bucket:        cfg.S3Bucket,
		publicBaseURL: cfg.S3PublicBaseURL,
	}, nil
}

// PresignAvatarUpload returns a short-lived presigned PUT URL scoped to
// userID's own object-key prefix, bounded by content type and size.
func (s *s3AvatarStore) PresignAvatarUpload(ctx context.Context, userID uuid.UUID, contentType string, contentLength int64) (*UploadTicket, error) {
	ext, ok := avatarExtension(contentType)
	if !ok {
		return nil, ErrUnsupportedContentType
	}
	if contentLength <= 0 || contentLength > maxAvatarBytes {
		return nil, ErrContentTooLarge
	}

	key, err := avatarObjectKey(userID, ext)
	if err != nil {
		return nil, fmt.Errorf("storage: generate object key: %w", err)
	}

	req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(contentLength),
	}, s3.WithPresignExpires(presignExpiry))
	if err != nil {
		return nil, fmt.Errorf("storage: presign avatar upload: %w", err)
	}

	return &UploadTicket{
		UploadURL: req.URL,
		PublicURL: s.publicBaseURL + "/" + key,
		ExpiresIn: int(presignExpiry.Seconds()),
	}, nil
}

// avatarObjectKey builds an owner-scoped, collision-free object key:
// avatars/{userID}/{random16hex}.{ext}. Including the user ID scopes every
// upload to its owner; the random component means a new upload never
// collides with or silently replaces a cached older one.
func avatarObjectKey(userID uuid.UUID, ext string) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("avatars/%s/%s.%s", userID.String(), hex.EncodeToString(buf), ext), nil
}
