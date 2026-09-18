package storage

import (
	"context"
	"strings"
	"testing"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// stubPresigner replaces the real S3 presign client in every test in this
// file, so no test reaches a real S3-compatible endpoint over the network.
type stubPresigner struct {
	url       string
	err       error
	lastInput *s3.PutObjectInput
	calls     int
}

func (p *stubPresigner) PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	p.calls++
	p.lastInput = params
	if p.err != nil {
		return nil, p.err
	}
	url := p.url
	if url == "" {
		url = "https://fake-bucket.example.test/signed-put"
	}
	return &v4.PresignedHTTPRequest{URL: url, Method: "PUT"}, nil
}

func newTestStore(p *stubPresigner) *s3AvatarStore {
	return &s3AvatarStore{
		presigner:     p,
		bucket:        "rndmroll-avatars-test",
		publicBaseURL: "https://cdn.example.test",
	}
}

func TestS3_PresignAvatarUpload_AllowedImageTypeReturnsPresignedURLAndPublicURL(t *testing.T) {
	for contentType := range allowedAvatarContentTypes {
		p := &stubPresigner{url: "https://fake-bucket.example.test/signed-put?sig=abc"}
		store := newTestStore(p)

		ticket, err := store.PresignAvatarUpload(context.Background(), uuid.New(), contentType, 1024)
		if err != nil {
			t.Fatalf("PresignAvatarUpload(%s) returned error: %v", contentType, err)
		}
		if ticket.UploadURL == "" {
			t.Errorf("expected a non-empty UploadURL for %s", contentType)
		}
		if !strings.HasPrefix(ticket.PublicURL, "https://cdn.example.test/avatars/") {
			t.Errorf("expected PublicURL to be rooted at the configured public base, got %q", ticket.PublicURL)
		}
		if p.calls != 1 {
			t.Errorf("expected exactly one presign call, got %d", p.calls)
		}
	}
}

func TestS3_PresignAvatarUpload_DisallowedContentTypeRejected(t *testing.T) {
	p := &stubPresigner{}
	store := newTestStore(p)

	_, err := store.PresignAvatarUpload(context.Background(), uuid.New(), "application/pdf", 1024)
	if err != ErrUnsupportedContentType {
		t.Fatalf("expected ErrUnsupportedContentType, got %v", err)
	}
	if p.calls != 0 {
		t.Errorf("expected the presigner never to be called for a rejected content type, got %d calls", p.calls)
	}
}

func TestS3_PresignAvatarUpload_ContentLengthAboveCapRejected(t *testing.T) {
	p := &stubPresigner{}
	store := newTestStore(p)

	_, err := store.PresignAvatarUpload(context.Background(), uuid.New(), "image/png", maxAvatarBytes+1)
	if err != ErrContentTooLarge {
		t.Fatalf("expected ErrContentTooLarge, got %v", err)
	}
	if p.calls != 0 {
		t.Errorf("expected the presigner never to be called for an oversized upload, got %d calls", p.calls)
	}
}

func TestS3_PresignAvatarUpload_ObjectKeyIncludesOwningUserID(t *testing.T) {
	p := &stubPresigner{}
	store := newTestStore(p)
	userID := uuid.New()

	if _, err := store.PresignAvatarUpload(context.Background(), userID, "image/jpeg", 2048); err != nil {
		t.Fatalf("PresignAvatarUpload returned error: %v", err)
	}
	if p.lastInput == nil || p.lastInput.Key == nil {
		t.Fatal("expected the presigner to receive an object key")
	}
	wantPrefix := "avatars/" + userID.String() + "/"
	if !strings.HasPrefix(*p.lastInput.Key, wantPrefix) {
		t.Errorf("expected object key to start with %q, got %q", wantPrefix, *p.lastInput.Key)
	}
}

func TestS3_PresignAvatarUpload_ExpiryIsSetExplicitly(t *testing.T) {
	p := &stubPresigner{}
	store := newTestStore(p)

	ticket, err := store.PresignAvatarUpload(context.Background(), uuid.New(), "image/webp", 512)
	if err != nil {
		t.Fatalf("PresignAvatarUpload returned error: %v", err)
	}
	if ticket.ExpiresIn != 300 {
		t.Errorf("expected an explicit 300s expiry, got %d", ticket.ExpiresIn)
	}
}

func TestS3_PresignAvatarUpload_DifferentUploadsGetDistinctObjectKeys(t *testing.T) {
	p := &stubPresigner{}
	store := newTestStore(p)
	userID := uuid.New()

	if _, err := store.PresignAvatarUpload(context.Background(), userID, "image/png", 1024); err != nil {
		t.Fatalf("PresignAvatarUpload returned error: %v", err)
	}
	firstKey := *p.lastInput.Key

	if _, err := store.PresignAvatarUpload(context.Background(), userID, "image/png", 1024); err != nil {
		t.Fatalf("PresignAvatarUpload returned error: %v", err)
	}
	secondKey := *p.lastInput.Key

	if firstKey == secondKey {
		t.Errorf("expected distinct object keys across uploads for the same user, got the same key twice: %q", firstKey)
	}
}

func TestNewAvatarStore_MissingConfigFieldReturnsError(t *testing.T) {
	_, err := NewAvatarStore(Config{})
	if err != ErrIncompleteConfig {
		t.Fatalf("expected ErrIncompleteConfig for an empty config, got %v", err)
	}
}
