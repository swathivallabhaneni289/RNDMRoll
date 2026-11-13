package storage

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// ObjectAPI is the part of the S3 client that deleting an account's files
// needs: listing a folder and deleting one object. It is an interface so the
// purge can be tested against an in-memory bucket (storagetest.Bucket), and
// no test reaches a real endpoint.
type ObjectAPI interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// The real client satisfies ObjectAPI.
var _ ObjectAPI = (*s3.Client)(nil)

var (
	// ErrNotAccountFolder is returned when an account's folder cannot be
	// named safely: the id is uuid.Nil, the folder is not avatars or
	// entries, or the prefix built from them does not have the one allowed
	// shape. Nothing is sent to storage.
	ErrNotAccountFolder = errors.New("storage: not an account folder")

	// errBadListing is wrapped by the error for a listing page that cannot
	// be trusted: a key outside the folder that was listed, a key with a dot
	// segment, or a page that says more follow without a usable token for
	// the next one. Nothing on such a page is deleted.
	errBadListing = errors.New("storage: the listing cannot be trusted")
)

// accountFolderNames are the folders that hold an account's files, in the
// order they are deleted: avatars/{id}/ (s3.go) and entries/{id}/
// (entry_photos.go). Comment photos are entry photos, so they are in
// entries/ too.
var accountFolderNames = [...]string{"avatars", "entries"}

// accountFolderPattern is the only shape an account folder prefix may have:
// one of the two folders, a canonical lowercase UUID and a closing slash.
var accountFolderPattern = regexp.MustCompile(`^(avatars|entries)/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/$`)

// AccountFiles deletes the files of an account whose deletion grace period
// is over, and the photos of other people's comments that go with it. This
// is the first code here that deletes in bulk, and nothing it deletes can be
// brought back, so every delete is fenced: a folder is named only from a real
// account id and checked against one exact pattern before it is listed, a
// listed key is deleted only when it is inside that folder, and a single
// photo is deleted only when its key has the shape of an entry photo key.
type AccountFiles struct {
	api    ObjectAPI
	bucket string
}

// NewAccountFiles builds AccountFiles with the same checks and client options
// as NewEntryPhotoStore: static credentials, the configured region (auto when
// empty), BaseEndpoint set to the endpoint and path-style addressing.
// Avatars and entry photos share this bucket configuration, so a
// configuration that the upload stores refuse is refused here as well instead
// of half working. A missing required field is ErrIncompleteConfig.
func NewAccountFiles(cfg Config) (*AccountFiles, error) {
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
	return NewAccountFilesOver(client, cfg.S3Bucket), nil
}

// NewAccountFilesOver builds AccountFiles over any ObjectAPI, such as
// storagetest.Bucket in tests.
func NewAccountFilesOver(api ObjectAPI, bucket string) *AccountFiles {
	return &AccountFiles{api: api, bucket: bucket}
}

// accountFolder returns the prefix of one of an account's folders,
// folder/{id}/, built only from a non-zero id. The zero id is refused first:
// no account has it, it is what a value that was never set looks like, and
// it would pass the pattern. The prefix is then checked against
// accountFolderPattern as the last guard before anything is listed, because
// the purge cannot be undone and a prefix of any other shape could name the
// whole folder ("entries/") or another person's files. A refusal is
// ErrNotAccountFolder.
func accountFolder(folder string, id uuid.UUID) (string, error) {
	if id == uuid.Nil {
		return "", ErrNotAccountFolder
	}
	if folder != "avatars" && folder != "entries" {
		return "", ErrNotAccountFolder
	}
	prefix := folder + "/" + id.String() + "/"
	if !accountFolderPattern.MatchString(prefix) {
		return "", ErrNotAccountFolder
	}
	return prefix, nil
}

// DeleteAccountFolders deletes every file in the account's two folders,
// avatars/{id}/ and then entries/{id}/, and returns how many it deleted. Both
// prefixes are built and checked before the first listing, so a bad id sends
// nothing to storage at all.
//
// Each folder is listed page by page, and a page is checked whole before
// anything on it is deleted. A key outside the folder, or a key with a "."
// or ".." segment, means storage is not answering what was asked, and the
// delete stops with an error instead of reaching past the folder. Keys are
// deleted one DeleteObject at a time, not with the batch DeleteObjects: a
// batch answers success even when some of its keys failed and reports those
// only inside its answer, where they are easy to miss, while one call per key
// makes every failure an error.
//
// The count deleted so far is returned with any error, and a done ctx stops
// the work with ctx.Err(). Running it again after an error is safe: what was
// deleted stays deleted, and the rest is listed afresh.
func (f *AccountFiles) DeleteAccountFolders(ctx context.Context, id uuid.UUID) (deleted int, err error) {
	var prefixes [len(accountFolderNames)]string
	for i, folder := range accountFolderNames {
		prefix, err := accountFolder(folder, id)
		if err != nil {
			return 0, err
		}
		prefixes[i] = prefix
	}
	for _, prefix := range prefixes {
		n, err := f.deleteFolder(ctx, prefix)
		deleted += n
		if err != nil {
			return deleted, err
		}
	}
	return deleted, nil
}

// deleteFolder lists prefix one page at a time and deletes what each page
// names, until a page says nothing more follows.
func (f *AccountFiles) deleteFolder(ctx context.Context, prefix string) (int, error) {
	deleted := 0
	var token *string
	for {
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		out, err := f.api.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(f.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return deleted, fmt.Errorf("storage: list %s: %w", prefix, err)
		}
		keys, next, err := checkPage(prefix, token, out)
		if err != nil {
			return deleted, err
		}
		for _, key := range keys {
			if err := ctx.Err(); err != nil {
				return deleted, err
			}
			if _, err := f.api.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(f.bucket),
				Key:    aws.String(key),
			}); err != nil {
				return deleted, fmt.Errorf("storage: delete %s: %w", key, err)
			}
			deleted++
		}
		if next == nil {
			return deleted, nil
		}
		token = next
	}
}

// checkPage returns the keys of one listing page of prefix and the token for
// the next page, nil on the last one. sent is the token the page was asked
// for with. The whole page is refused, with an error wrapping errBadListing,
// when any key is outside prefix or has a dot segment, or when the page says
// more follow but gives no token, or gives back the token it was sent: the
// next listing would then ask for the same page again and go round forever.
func checkPage(prefix string, sent *string, out *s3.ListObjectsV2Output) ([]string, *string, error) {
	if out == nil {
		return nil, nil, fmt.Errorf("%w: listing %s gave no page", errBadListing, prefix)
	}
	keys := make([]string, 0, len(out.Contents))
	for _, obj := range out.Contents {
		key := aws.ToString(obj.Key)
		if !strings.HasPrefix(key, prefix) {
			return nil, nil, fmt.Errorf("%w: listing %s named %q, which is outside it", errBadListing, prefix, key)
		}
		if hasDotSegment(key[len(prefix):]) {
			return nil, nil, fmt.Errorf("%w: listing %s named %q, which has a dot segment", errBadListing, prefix, key)
		}
		keys = append(keys, key)
	}
	if !aws.ToBool(out.IsTruncated) {
		return keys, nil, nil
	}
	next := aws.ToString(out.NextContinuationToken)
	if next == "" {
		return nil, nil, fmt.Errorf("%w: listing %s says more pages follow but gives no token for the next one", errBadListing, prefix)
	}
	if next == aws.ToString(sent) {
		return nil, nil, fmt.Errorf("%w: listing %s gave back the token it was sent", errBadListing, prefix)
	}
	return keys, &next, nil
}

// hasDotSegment reports whether any slash separated segment of rest is "."
// or "..". The server never makes such a key, and some HTTP layers resolve
// these segments before storage sees the path, so a delete of
// entries/{id}/../{other id}/x.jpg could land in another person's folder.
func hasDotSegment(rest string) bool {
	for _, segment := range strings.Split(rest, "/") {
		if segment == "." || segment == ".." {
			return true
		}
	}
	return false
}

// DeletePhoto deletes one file by key: the photo of another person's comment
// that is deleted with the purged account. Like DeleteEntryPhoto it calls
// storage once, and only for a key shaped like an entry photo key
// (entries/{canonical lowercase UUID}/{16 lowercase hex}.{jpg|png|webp});
// anything else is refused with errNotEntryPhotoKey and no call, so a bad
// value can never name a folder or some other object. S3 answers success for
// a key that is already gone, so deleting the same photo again is harmless.
func (f *AccountFiles) DeletePhoto(ctx context.Context, key string) error {
	if !isEntryPhotoKey(key) {
		return errNotEntryPhotoKey
	}
	if _, err := f.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("storage: delete %s: %w", key, err)
	}
	return nil
}
