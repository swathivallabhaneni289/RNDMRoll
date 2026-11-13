// Package storage generates presigned upload tickets and photo links
// against S3-compatible object storage. NewAvatarStore's client is
// configured with path-style addressing and an explicit BaseEndpoint so the
// same code works against AWS S3, Cloudflare R2, Supabase Storage and a local
// MinIO without branching.
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

// maxAvatarBytes caps an avatar upload at 50 MB (50,000,000 bytes): a safety
// ceiling far above any phone photo, so in practice people can pick any photo
// (the developer asked for no practical limit, 2026-10-08), and no higher
// than Supabase's free plan takes for one file, so a bigger photo gets the
// app's own message and never a storage error. This bound is passed into the
// presigned request itself (ContentLength), not merely checked in Go, so the
// storage provider enforces it even if the uploading client ignores what it
// was handed.
const maxAvatarBytes = 50_000_000

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

	// errNotAvatarKey is what DeleteAvatar returns for a key that is not a
	// profile photo key in the owner's own folder. Nothing is sent to
	// storage.
	errNotAvatarKey = errors.New("storage: not a profile photo key of that account")
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

// avatarKeyFolder starts every avatar key.
const avatarKeyFolder = "avatars/"

// avatarExtension returns the object-key extension for an image content type.
// A type outside the map is accepted when it is image/ followed by up to ten
// lowercase letters or digits (isAvatarExtension), and that subtype becomes
// the extension; anything else (a PDF, a path, an odd character) is refused so
// nothing unexpected ever lands in an object key.
func avatarExtension(contentType string) (string, bool) {
	if ext, ok := allowedAvatarContentTypes[contentType]; ok {
		return ext, true
	}
	sub, ok := strings.CutPrefix(contentType, "image/")
	if !ok || !isAvatarExtension(sub) {
		return "", false
	}
	return sub, true
}

// isAvatarExtension reports whether ext is one to ten lowercase letters or
// digits: every extension avatarExtension gives, those of the map included.
func isAvatarExtension(ext string) bool {
	if ext == "" || len(ext) > 10 {
		return false
	}
	for _, r := range ext {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// UploadTicket is what a client receives to perform a direct-to-storage
// avatar upload: a presigned PUT URL to upload to, and the key the object
// will have once uploaded. The client stores Key back through PATCH /me once
// the upload succeeds. It carries no link: the server signs one from the
// stored key whenever it sends the profile photo (Link).
type UploadTicket struct {
	UploadURL string
	Key       string
	ExpiresIn int
}

// AvatarStore issues presigned avatar-upload tickets and removes the file of
// a profile photo a new one replaced.
type AvatarStore interface {
	PresignAvatarUpload(ctx context.Context, userID uuid.UUID, contentType string, contentLength int64) (*UploadTicket, error)
	DeleteAvatar(ctx context.Context, userID uuid.UUID, key string) error
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
}

// presigner is the seam s3_test.go substitutes with a stub, so no test in
// this package reaches a real S3-compatible endpoint over the network. The
// real bucket (a private Supabase Storage bucket, or MinIO when run
// locally) is checked by running the app against it, not by these tests.
type presigner interface {
	PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// s3AvatarStore is the AvatarStore implementation backed by an S3-compatible
// presign client, and the client itself for deletes.
type s3AvatarStore struct {
	presigner presigner
	deleter   objectDeleter
	bucket    string
}

// NewAvatarStore builds an AvatarStore backed by aws-sdk-go-v2, using
// static credentials, the configured region, BaseEndpoint set to the
// configured endpoint, and path-style addressing.
func NewAvatarStore(cfg Config) (AvatarStore, error) {
	if cfg.S3Endpoint == "" || cfg.S3Bucket == "" || cfg.S3AccessKeyID == "" || cfg.S3SecretAccessKey == "" {
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
		presigner: presignClient,
		deleter:   client,
		bucket:    cfg.S3Bucket,
	}, nil
}

// DeleteAvatar removes the file of one of userID's profile photos, the one a
// new photo replaced. It calls storage once, and only for a key that is
// exactly a profile photo key in the folder of userID, a real account
// (AvatarKeyOwnedBy): anything else is refused with errNotAvatarKey and no
// call, so a bad value can never name a folder, another person's photo or
// some other object. Nothing is ever deleted by prefix. S3 answers success
// for a key that is already gone, so deleting the same photo again is
// harmless. The storage error, if any, is returned as it came.
func (s *s3AvatarStore) DeleteAvatar(ctx context.Context, userID uuid.UUID, key string) error {
	if userID == uuid.Nil || !AvatarKeyOwnedBy(key, userID) {
		return errNotAvatarKey
	}
	_, err := s.deleter.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
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
		Key:       key,
		ExpiresIn: int(presignExpiry.Seconds()),
	}, nil
}

// avatarObjectKey builds an owner-scoped, collision-free object key:
// avatars/{userID}/{random16hex}.{ext}. Including the user ID scopes every
// upload to its owner; the random component means a new upload never
// collides with or silently replaces a cached older one.
func avatarObjectKey(userID uuid.UUID, ext string) (string, error) {
	buf := make([]byte, entryKeyHexLen/2)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s/%s.%s", avatarKeyFolder, userID.String(), hex.EncodeToString(buf), ext), nil
}

// AvatarKeyOwnedBy reports whether key is exactly avatars/{userID}/{16
// lowercase hex}.{extension} and nothing else, the extension being one
// avatarExtension gives: no dot segments, no query, no second slash, no
// capitals, no link. It is the only profile photo PATCH /me saves, and the
// shape migration 0019 checks.
func AvatarKeyOwnedBy(key string, userID uuid.UUID) bool {
	prefix := avatarKeyFolder + userID.String() + "/"
	if !strings.HasPrefix(key, prefix) {
		return false
	}
	return isAvatarFileName(key[len(prefix):])
}

// isAvatarKey reports whether key has the shape of an avatar key for some
// user: the folder, a canonical lowercase UUID and a server made file name.
func isAvatarKey(key string) bool {
	rest, ok := strings.CutPrefix(key, avatarKeyFolder)
	if !ok {
		return false
	}
	folder, name, ok := strings.Cut(rest, "/")
	if !ok || len(folder) != 36 {
		return false
	}
	id, err := uuid.Parse(folder)
	if err != nil || id.String() != folder {
		return false
	}
	return isAvatarFileName(name)
}

// isAvatarFileName reports whether name is exactly 16 lowercase hex digits, a
// dot and an extension isAvatarExtension accepts.
func isAvatarFileName(name string) bool {
	if len(name) <= entryKeyHexLen+1 || name[entryKeyHexLen] != '.' {
		return false
	}
	for i := 0; i < entryKeyHexLen; i++ {
		c := name[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return isAvatarExtension(name[entryKeyHexLen+1:])
}
