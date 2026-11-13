package storage

import (
	"context"
	"errors"
	"regexp"
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
		presigner: p,
		bucket:    "rndmroll-avatars-test",
	}
}

func TestS3_PresignAvatarUpload_AllowedImageTypeReturnsPresignedURLAndKey(t *testing.T) {
	for contentType, ext := range allowedAvatarContentTypes {
		p := &stubPresigner{url: "https://fake-bucket.example.test/signed-put?sig=abc"}
		store := newTestStore(p)
		userID := uuid.New()

		ticket, err := store.PresignAvatarUpload(context.Background(), userID, contentType, 1024)
		if err != nil {
			t.Fatalf("PresignAvatarUpload(%s) returned error: %v", contentType, err)
		}
		if ticket.UploadURL == "" {
			t.Errorf("expected a non-empty UploadURL for %s", contentType)
		}
		want := regexp.MustCompile(`^avatars/` + userID.String() + `/[0-9a-f]{16}\.` + ext + `$`)
		if !want.MatchString(ticket.Key) || *p.lastInput.Key != ticket.Key {
			t.Errorf("%s: Key = %q (signed %q), want avatars/<user>/<16 hex>.%s and the key that was signed", contentType, ticket.Key, *p.lastInput.Key, ext)
		}
		if !AvatarKeyOwnedBy(ticket.Key, userID) || !isAvatarKey(ticket.Key) {
			t.Errorf("%s: key %q is not a profile photo key of the caller", contentType, ticket.Key)
		}
		if p.calls != 1 {
			t.Errorf("expected exactly one presign call, got %d", p.calls)
		}
	}
}

// Every image type the store accepts gives a key PATCH /me saves: the
// extension rule of the key is the one of the content type.
func TestS3_PresignAvatarUpload_EveryAcceptedTypeGivesASavableKey(t *testing.T) {
	userID := uuid.New()
	for _, contentType := range []string{"image/jpeg", "image/heic", "image/jxl", "image/bmp", "image/x3f", "image/abcdefghij"} {
		ticket, err := newTestStore(&stubPresigner{}).PresignAvatarUpload(context.Background(), userID, contentType, 1024)
		if err != nil {
			t.Fatalf("%s: %v", contentType, err)
		}
		if !AvatarKeyOwnedBy(ticket.Key, userID) {
			t.Errorf("%s: key %q would be refused by PATCH /me", contentType, ticket.Key)
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

// The real avatar store signs its upload ticket at the current time for five
// minutes: an upload is never rounded down like a photo link.
func TestNewAvatarStore_TheUploadTicketIsNeverRounded(t *testing.T) {
	store, err := NewAvatarStore(testLinkConfig)
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := store.PresignAvatarUpload(context.Background(), uuid.New(), "image/png", 4096)
	if err != nil {
		t.Fatal(err)
	}
	wantFreshUpload(t, ticket.UploadURL)
}

func TestAvatarKeyOwnedBy(t *testing.T) {
	me := uuid.MustParse("3f0c2a52-7d4e-4c4b-9c1d-6a1f2b3c4d5e")
	other := uuid.MustParse("9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d")
	const file = "0123456789abcdef"
	own := "avatars/" + me.String() + "/" + file

	cases := []struct {
		name string
		key  string
		want bool
	}{
		{"own jpg", own + ".jpg", true},
		{"own heic", own + ".heic", true},
		{"own ten character extension", own + ".abcdefghij", true},
		{"own extension with digits", own + ".x3f", true},
		{"another user's folder", "avatars/" + other.String() + "/" + file + ".jpg", false},
		{"the entry folder", "entries/" + me.String() + "/" + file + ".jpg", false},
		{"another folder", "uploads/" + me.String() + "/" + file + ".jpg", false},
		{"a dot dot segment", "avatars/" + me.String() + "/../" + other.String() + "/" + file + ".jpg", false},
		{"dot dot as the folder", "avatars/../" + file + ".jpg", false},
		{"an extra slash", "avatars/" + me.String() + "/x/" + file + ".jpg", false},
		{"a query", own + ".jpg?x=1", false},
		{"a fragment", own + ".jpg#x", false},
		{"a full URL", "https://cdn.example.test/" + own + ".jpg", false},
		{"a leading slash", "/" + own + ".jpg", false},
		{"upper case hex in the file name", "avatars/" + me.String() + "/0123456789ABCDEF.jpg", false},
		{"upper case user id", "avatars/" + strings.ToUpper(me.String()) + "/" + file + ".jpg", false},
		{"15 hex digits", "avatars/" + me.String() + "/0123456789abcde.jpg", false},
		{"17 hex digits", "avatars/" + me.String() + "/0123456789abcdef0.jpg", false},
		{"an extension over ten", own + ".abcdefghijk", false},
		{"upper case extension", own + ".JPG", false},
		{"two extensions", own + ".jpg.png", false},
		{"no extension", own, false},
		{"a trailing dot", own + ".", false},
		{"a percent escape", own + ".j%70g", false},
		{"a trailing line break", own + ".jpg\n", false},
		{"empty", "", false},
		{"just the folder", "avatars/", false},
		{"the folder of the user", "avatars/" + me.String() + "/", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AvatarKeyOwnedBy(c.key, me); got != c.want {
				t.Errorf("AvatarKeyOwnedBy(%q) = %v, want %v", c.key, got, c.want)
			}
		})
	}
}

// DeleteAvatar sends one delete for the owner's own profile photo key, in
// the bucket.
func TestS3_DeleteAvatar_CallsStorageOnceForTheOwnersKey(t *testing.T) {
	d := &stubDeleter{}
	store := &s3AvatarStore{presigner: &stubPresigner{}, deleter: d, bucket: "rndmroll-avatars-test"}
	owner := uuid.New()
	key := "avatars/" + owner.String() + "/0123456789abcdef.heic"

	if err := store.DeleteAvatar(context.Background(), owner, key); err != nil {
		t.Fatal(err)
	}
	if d.calls != 1 || d.lastInput == nil || *d.lastInput.Key != key || *d.lastInput.Bucket != "rndmroll-avatars-test" {
		t.Fatalf("calls = %d input = %+v, want one call for the key", d.calls, d.lastInput)
	}
}

// DeleteAvatar sends nothing to storage for any key but a profile photo key
// in the owner's own folder: not another person's, not an entry photo, not a
// folder, a link or a malformed name, and nothing for the zero id.
func TestS3_DeleteAvatar_NeverCallsStorageForAnyOtherKey(t *testing.T) {
	me := uuid.MustParse("3f0c2a52-7d4e-4c4b-9c1d-6a1f2b3c4d5e")
	other := uuid.MustParse("9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d")
	mine := "avatars/" + me.String() + "/"
	own := mine + "0123456789abcdef.jpg"
	cases := map[string]struct {
		owner uuid.UUID
		key   string
	}{
		"another person's key":    {me, "avatars/" + other.String() + "/0123456789abcdef.jpg"},
		"someone else's own key":  {other, own},
		"an entry photo key":      {me, "entries/" + me.String() + "/0123456789abcdef.jpg"},
		"the owner's folder":      {me, mine},
		"the avatar folder":       {me, "avatars/"},
		"a dot dot segment":       {me, mine + "../" + other.String() + "/0123456789abcdef.jpg"},
		"a full URL":              {me, "https://cdn.example.test/" + own},
		"a signed link":           {me, "https://cdn.example.test/" + own + "?X-Amz-Signature=abc"},
		"a query":                 {me, own + "?versionId=1"},
		"upper case hex":          {me, mine + "0123456789ABCDEF.jpg"},
		"a 15 hex name":           {me, mine + "0123456789abcde.jpg"},
		"an extra slash":          {me, mine + "x/0123456789abcdef.jpg"},
		"a leading slash":         {me, "/" + own},
		"empty":                   {me, ""},
		"the zero id's own shape": {uuid.Nil, "avatars/" + uuid.Nil.String() + "/0123456789abcdef.jpg"},
	}
	for name, c := range cases {
		d := &stubDeleter{}
		store := &s3AvatarStore{presigner: &stubPresigner{}, deleter: d, bucket: "rndmroll-avatars-test"}
		if err := store.DeleteAvatar(context.Background(), c.owner, c.key); !errors.Is(err, errNotAvatarKey) {
			t.Errorf("%s: DeleteAvatar(%q) = %v, want errNotAvatarKey", name, c.key, err)
		}
		if d.calls != 0 {
			t.Errorf("%s: DeleteAvatar(%q) called storage %d times", name, c.key, d.calls)
		}
	}
}

func TestS3_DeleteAvatar_ReturnsStorageError(t *testing.T) {
	boom := errors.New("storage is down")
	store := &s3AvatarStore{presigner: &stubPresigner{}, deleter: &stubDeleter{err: boom}, bucket: "rndmroll-avatars-test"}
	owner := uuid.New()
	if err := store.DeleteAvatar(context.Background(), owner, "avatars/"+owner.String()+"/0123456789abcdef.png"); err != boom {
		t.Fatalf("error = %v, want the storage error", err)
	}
}
