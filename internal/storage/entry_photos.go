package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// MaxEntryPhotoBytes caps one entry photo at 50 MiB, the same ceiling as the
// avatar. The limit is per photo and one ticket is made per photo. Like the
// avatar's, the bound is signed into the upload request itself.
const MaxEntryPhotoBytes = 50 * 1024 * 1024

var (
	// ErrEntryPhotoType is returned for a content type that is not exactly
	// image/jpeg, image/png or image/webp.
	ErrEntryPhotoType = errors.New("storage: unsupported entry photo content type")
	// ErrEntryPhotoTooLarge is returned when the length is above
	// MaxEntryPhotoBytes, and also for a length of zero or less (as the
	// avatar store does).
	ErrEntryPhotoTooLarge = errors.New("storage: entry photo is empty or exceeds the size limit")

	// errNotEntryPhotoKey is what DeleteEntryPhoto returns for a string that
	// is not shaped like an entry photo key. Nothing is sent to storage.
	errNotEntryPhotoKey = errors.New("storage: not an entry photo key")
)

// entryPhotoExtensions maps the allowed content types, exactly, to the file
// extension used in the key.
var entryPhotoExtensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

const (
	// entryKeyFolder starts every entry photo key.
	entryKeyFolder = "entries/"
	// entryKeyHexLen is the length of the random file name.
	entryKeyHexLen = 16
)

// EntryPhotoTicket is what a client receives to upload one entry photo
// straight to storage: a presigned PUT URL, and the key the file will have.
// The key is sent back when the entry is saved. It carries no public link:
// the server builds links from stored keys when it reads.
type EntryPhotoTicket struct {
	UploadURL string
	Key       string
	ExpiresIn int
}

// EntryPhotoStore issues upload tickets for entry photos, builds the public
// link of a stored key and removes a file.
type EntryPhotoStore interface {
	PresignEntryPhotoUpload(ctx context.Context, userID uuid.UUID, contentType string, contentLength int64) (*EntryPhotoTicket, error)
	PublicURL(key string) string
	DeleteEntryPhoto(ctx context.Context, key string) error
}

// objectDeleter is the seam entry_photos_test.go substitutes with a stub, as
// presigner is for uploads, so no test reaches a real endpoint.
type objectDeleter interface {
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// The real client satisfies both seams.
var _ objectDeleter = (*s3.Client)(nil)
var _ presigner = (*s3.PresignClient)(nil)

type s3EntryPhotoStore struct {
	presigner     presigner
	deleter       objectDeleter
	bucket        string
	publicBaseURL string
}

// NewEntryPhotoStore builds an EntryPhotoStore with the same client options
// as NewAvatarStore: static credentials, the configured region (auto when
// empty), BaseEndpoint set to the endpoint and path-style addressing. A
// missing required field is ErrIncompleteConfig.
func NewEntryPhotoStore(cfg Config) (EntryPhotoStore, error) {
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

	return &s3EntryPhotoStore{
		presigner:     s3.NewPresignClient(client),
		deleter:       client,
		bucket:        cfg.S3Bucket,
		publicBaseURL: cfg.S3PublicBaseURL,
	}, nil
}

// PresignEntryPhotoUpload returns a short-lived presigned PUT URL for one
// photo, with the content type and length signed into the request, and the
// key it will have: entries/{userID}/{16 lowercase hex}.{ext}, new for every
// ticket. The type is checked first, then the length.
func (s *s3EntryPhotoStore) PresignEntryPhotoUpload(ctx context.Context, userID uuid.UUID, contentType string, contentLength int64) (*EntryPhotoTicket, error) {
	ext, ok := entryPhotoExtensions[contentType]
	if !ok {
		return nil, ErrEntryPhotoType
	}
	if contentLength <= 0 || contentLength > MaxEntryPhotoBytes {
		return nil, ErrEntryPhotoTooLarge
	}

	key, err := entryPhotoKey(userID, ext)
	if err != nil {
		return nil, fmt.Errorf("storage: generate entry photo key: %w", err)
	}

	req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(contentLength),
	}, s3.WithPresignExpires(presignExpiry))
	if err != nil {
		return nil, fmt.Errorf("storage: presign entry photo upload: %w", err)
	}

	return &EntryPhotoTicket{
		UploadURL: req.URL,
		Key:       key,
		ExpiresIn: int(presignExpiry.Seconds()),
	}, nil
}

// PublicURL joins the public base (trailing slashes trimmed) and the key with
// one slash.
func (s *s3EntryPhotoStore) PublicURL(key string) string {
	return strings.TrimRight(s.publicBaseURL, "/") + "/" + strings.TrimLeft(key, "/")
}

// DeleteEntryPhoto removes one file. It calls storage once, and only for a
// string shaped like an entry photo key (of any user); anything else is
// refused with no call, so a bad value can never name some other object. The
// storage error, if any, is returned as it came.
func (s *s3EntryPhotoStore) DeleteEntryPhoto(ctx context.Context, key string) error {
	if !isEntryPhotoKey(key) {
		return errNotEntryPhotoKey
	}
	_, err := s.deleter.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

// EntryPhotoKeyOwnedBy reports whether key is exactly
// entries/{userID}/{16 lowercase hex}.{jpg|png|webp} and nothing else: no
// dot segments, no query, no second slash, no capitals.
func EntryPhotoKeyOwnedBy(key string, userID uuid.UUID) bool {
	prefix := entryKeyFolder + userID.String() + "/"
	if !strings.HasPrefix(key, prefix) {
		return false
	}
	return isEntryPhotoFileName(key[len(prefix):])
}

// isEntryPhotoKey reports whether key has the shape of an entry photo key for
// some user: the folder, a canonical lowercase UUID and a server made file
// name.
func isEntryPhotoKey(key string) bool {
	rest, ok := strings.CutPrefix(key, entryKeyFolder)
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
	return isEntryPhotoFileName(name)
}

// isEntryPhotoFileName reports whether name is exactly 16 lowercase hex
// digits, a dot and one of the allowed extensions.
func isEntryPhotoFileName(name string) bool {
	if len(name) <= entryKeyHexLen+1 || name[entryKeyHexLen] != '.' {
		return false
	}
	for i := 0; i < entryKeyHexLen; i++ {
		c := name[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	switch name[entryKeyHexLen+1:] {
	case "jpg", "png", "webp":
		return true
	}
	return false
}

// entryPhotoKey builds an owner-scoped, collision-free key:
// entries/{userID}/{random16hex}.{ext}.
func entryPhotoKey(userID uuid.UUID, ext string) (string, error) {
	buf := make([]byte, entryKeyHexLen/2)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s/%s.%s", entryKeyFolder, userID.String(), hex.EncodeToString(buf), ext), nil
}
