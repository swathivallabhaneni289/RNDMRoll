package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// recordingPresigner is stubPresigner plus the options it was given, so a
// test can read the expiry that was passed to the signer.
type recordingPresigner struct {
	stubPresigner
	expires time.Duration
}

func (p *recordingPresigner) PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	var opts s3.PresignOptions
	for _, fn := range optFns {
		fn(&opts)
	}
	p.expires = opts.Expires
	return p.stubPresigner.PresignPutObject(ctx, params, optFns...)
}

// stubDeleter replaces the real S3 client for deletes.
type stubDeleter struct {
	err       error
	calls     int
	lastInput *s3.DeleteObjectInput
}

func (d *stubDeleter) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	d.calls++
	d.lastInput = params
	if d.err != nil {
		return nil, d.err
	}
	return &s3.DeleteObjectOutput{}, nil
}

func newTestEntryStore(p presigner, d objectDeleter) *s3EntryPhotoStore {
	return &s3EntryPhotoStore{
		presigner:     p,
		deleter:       d,
		bucket:        "rndmroll-photos-test",
		publicBaseURL: "https://cdn.example.test",
	}
}

func TestEntryPhotoStore_EachAllowedTypeGivesATicketInTheCallersFolder(t *testing.T) {
	for contentType, ext := range map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"} {
		p := &stubPresigner{url: "https://fake-bucket.example.test/signed-put?sig=abc"}
		store := newTestEntryStore(p, &stubDeleter{})
		userID := uuid.New()

		ticket, err := store.PresignEntryPhotoUpload(context.Background(), userID, contentType, 2345678)
		if err != nil {
			t.Fatalf("%s: %v", contentType, err)
		}
		if ticket.UploadURL != "https://fake-bucket.example.test/signed-put?sig=abc" {
			t.Errorf("%s: UploadURL = %q", contentType, ticket.UploadURL)
		}
		if ticket.ExpiresIn != 300 {
			t.Errorf("%s: ExpiresIn = %d, want 300", contentType, ticket.ExpiresIn)
		}
		if !strings.HasPrefix(ticket.Key, "entries/"+userID.String()+"/") || !strings.HasSuffix(ticket.Key, "."+ext) {
			t.Errorf("%s: key = %q, want entries/<user>/<hex>.%s", contentType, ticket.Key, ext)
		}
		if !EntryPhotoKeyOwnedBy(ticket.Key, userID) {
			t.Errorf("%s: key %q is not owned by the caller", contentType, ticket.Key)
		}
		if p.calls != 1 {
			t.Errorf("%s: %d presign calls, want 1", contentType, p.calls)
		}
	}
}

func TestEntryPhotoStore_OtherTypesAreRefusedWithoutASignature(t *testing.T) {
	for _, contentType := range []string{
		"image/gif", "image/heic", "image/heif", "image/avif", "application/pdf",
		"image/jpeg; charset=x", "IMAGE/JPEG", "image/JPEG", "image/jpg", " image/jpeg", "image/jpeg ", "image/", "image", "",
	} {
		p := &stubPresigner{}
		store := newTestEntryStore(p, &stubDeleter{})
		_, err := store.PresignEntryPhotoUpload(context.Background(), uuid.New(), contentType, 1024)
		if !errors.Is(err, ErrEntryPhotoType) {
			t.Errorf("%q: error = %v, want ErrEntryPhotoType", contentType, err)
		}
		if p.calls != 0 {
			t.Errorf("%q: the presigner was called %d times", contentType, p.calls)
		}
	}
}

func TestEntryPhotoStore_LengthBounds(t *testing.T) {
	cases := []struct {
		name   string
		length int64
		ok     bool
	}{
		{"zero", 0, false},
		{"negative", -1, false},
		{"one byte", 1, true},
		{"50 MiB", 50 * 1024 * 1024, true},
		{"50 MiB plus one", 50*1024*1024 + 1, false},
	}
	if MaxEntryPhotoBytes != 50*1024*1024 {
		t.Fatalf("MaxEntryPhotoBytes = %d", MaxEntryPhotoBytes)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &stubPresigner{}
			store := newTestEntryStore(p, &stubDeleter{})
			_, err := store.PresignEntryPhotoUpload(context.Background(), uuid.New(), "image/jpeg", c.length)
			if c.ok {
				if err != nil || p.calls != 1 {
					t.Fatalf("error = %v, calls = %d, want accepted", err, p.calls)
				}
				return
			}
			if !errors.Is(err, ErrEntryPhotoTooLarge) || p.calls != 0 {
				t.Fatalf("error = %v, calls = %d, want ErrEntryPhotoTooLarge and no signature", err, p.calls)
			}
		})
	}
}

func TestEntryPhotoStore_TheSignedRequestCarriesTheTypeLengthAndExpiry(t *testing.T) {
	p := &recordingPresigner{}
	store := newTestEntryStore(p, &stubDeleter{})

	ticket, err := store.PresignEntryPhotoUpload(context.Background(), uuid.New(), "image/png", 987654)
	if err != nil {
		t.Fatal(err)
	}
	in := p.lastInput
	if in == nil || in.Bucket == nil || in.Key == nil || in.ContentType == nil || in.ContentLength == nil {
		t.Fatalf("the signed request is incomplete: %+v", in)
	}
	if *in.Bucket != "rndmroll-photos-test" || *in.Key != ticket.Key || *in.ContentType != "image/png" || *in.ContentLength != 987654 {
		t.Errorf("signed request = bucket %q key %q type %q length %d", *in.Bucket, *in.Key, *in.ContentType, *in.ContentLength)
	}
	if p.expires != 300*time.Second {
		t.Errorf("expiry passed to the signer = %v, want 5 minutes", p.expires)
	}
}

func TestEntryPhotoStore_TwoTicketsGiveTwoKeys(t *testing.T) {
	store := newTestEntryStore(&stubPresigner{}, &stubDeleter{})
	userID := uuid.New()
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		ticket, err := store.PresignEntryPhotoUpload(context.Background(), userID, "image/jpeg", 1024)
		if err != nil {
			t.Fatal(err)
		}
		if seen[ticket.Key] {
			t.Fatalf("key %q was handed out twice", ticket.Key)
		}
		seen[ticket.Key] = true
	}
}

func TestEntryPhotoStore_PresignerErrorIsReturnedWithNoTicket(t *testing.T) {
	boom := errors.New("signer is down")
	store := newTestEntryStore(&stubPresigner{err: boom}, &stubDeleter{})
	ticket, err := store.PresignEntryPhotoUpload(context.Background(), uuid.New(), "image/jpeg", 1024)
	if ticket != nil || !errors.Is(err, boom) {
		t.Fatalf("ticket %v, error %v, want none and the signer's error", ticket, err)
	}
}

func TestEntryPhotoKeyOwnedBy(t *testing.T) {
	me := uuid.MustParse("3f0c2a52-7d4e-4c4b-9c1d-6a1f2b3c4d5e")
	other := uuid.MustParse("9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d")
	const file = "0123456789abcdef"
	own := "entries/" + me.String() + "/" + file

	cases := []struct {
		name string
		key  string
		want bool
	}{
		{"own jpg", own + ".jpg", true},
		{"own png", own + ".png", true},
		{"own webp", own + ".webp", true},
		{"another user's folder", "entries/" + other.String() + "/" + file + ".jpg", false},
		{"upper case hex in the file name", "entries/" + me.String() + "/0123456789ABCDEF.jpg", false},
		{"upper case user id", "entries/" + strings.ToUpper(me.String()) + "/" + file + ".jpg", false},
		{"15 hex digits", "entries/" + me.String() + "/0123456789abcde.jpg", false},
		{"17 hex digits", "entries/" + me.String() + "/0123456789abcdef0.jpg", false},
		{"a dot dot segment", "entries/" + me.String() + "/../" + file + ".jpg", false},
		{"dot dot as the folder", "entries/../" + file + ".jpg", false},
		{"an extra slash", "entries/" + me.String() + "/x/" + file + ".jpg", false},
		{"a leading slash", "/" + own + ".jpg", false},
		{"another extension", own + ".gif", false},
		{"upper case extension", own + ".JPG", false},
		{"two extensions", own + ".jpg.png", false},
		{"no extension", own, false},
		{"a trailing dot", own + ".", false},
		{"a query", own + ".jpg?x=1", false},
		{"a trailing line break", own + ".jpg\n", false},
		{"the avatar folder", "avatars/" + me.String() + "/" + file + ".jpg", false},
		{"a full URL", "https://cdn.example.test/" + own + ".jpg", false},
		{"empty", "", false},
		{"just the folder", "entries/", false},
		{"the folder of the user", "entries/" + me.String() + "/", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := EntryPhotoKeyOwnedBy(c.key, me); got != c.want {
				t.Errorf("EntryPhotoKeyOwnedBy(%q) = %v, want %v", c.key, got, c.want)
			}
		})
	}
}

func TestEntryPhotoStore_PublicURL(t *testing.T) {
	key := "entries/3f0c2a52-7d4e-4c4b-9c1d-6a1f2b3c4d5e/0123456789abcdef.jpg"
	for _, base := range []string{"https://cdn.example.test", "https://cdn.example.test/", "https://cdn.example.test//"} {
		store := newTestEntryStore(&stubPresigner{}, &stubDeleter{})
		store.publicBaseURL = base
		if got, want := store.PublicURL(key), "https://cdn.example.test/"+key; got != want {
			t.Errorf("base %q: PublicURL = %q, want %q", base, got, want)
		}
	}
}

func TestEntryPhotoStore_DeleteCallsStorageOnceForAValidKey(t *testing.T) {
	d := &stubDeleter{}
	store := newTestEntryStore(&stubPresigner{}, d)
	key := "entries/" + uuid.NewString() + "/0123456789abcdef.webp"

	if err := store.DeleteEntryPhoto(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if d.calls != 1 || d.lastInput == nil || *d.lastInput.Key != key || *d.lastInput.Bucket != "rndmroll-photos-test" {
		t.Fatalf("calls = %d input = %+v, want one call for the key", d.calls, d.lastInput)
	}
}

func TestEntryPhotoStore_DeleteNeverCallsStorageForABadKey(t *testing.T) {
	id := uuid.NewString()
	for _, key := range []string{
		"",
		"entries/",
		"entries/" + id + "/",
		"entries/" + id + "/0123456789abcdef",
		"entries/" + id + "/0123456789abcdef.gif",
		"entries/" + id + "/0123456789ABCDEF.jpg",
		"entries/" + id + "/../0123456789abcdef.jpg",
		"entries/" + id + "/x/0123456789abcdef.jpg",
		"entries/" + strings.ToUpper(id) + "/0123456789abcdef.jpg",
		"entries/not-a-user/0123456789abcdef.jpg",
		"entries/{" + id + "}/0123456789abcdef.jpg",
		"avatars/" + id + "/0123456789abcdef.jpg",
		"/entries/" + id + "/0123456789abcdef.jpg",
		"https://cdn.example.test/entries/" + id + "/0123456789abcdef.jpg",
		"entries/" + id + "/0123456789abcdef.jpg?versionId=1",
	} {
		d := &stubDeleter{}
		store := newTestEntryStore(&stubPresigner{}, d)
		if err := store.DeleteEntryPhoto(context.Background(), key); err == nil {
			t.Errorf("DeleteEntryPhoto(%q) = nil, want a refusal", key)
		}
		if d.calls != 0 {
			t.Errorf("DeleteEntryPhoto(%q) called storage %d times", key, d.calls)
		}
	}
}

func TestEntryPhotoStore_DeleteReturnsStorageError(t *testing.T) {
	boom := errors.New("storage is down")
	store := newTestEntryStore(&stubPresigner{}, &stubDeleter{err: boom})
	key := "entries/" + uuid.NewString() + "/0123456789abcdef.jpg"
	if err := store.DeleteEntryPhoto(context.Background(), key); err != boom {
		t.Fatalf("error = %v, want the storage error", err)
	}
}

func TestNewEntryPhotoStore_MissingConfigFieldReturnsError(t *testing.T) {
	full := Config{
		S3Endpoint:        "https://s3.example.test",
		S3Region:          "auto",
		S3Bucket:          "photos",
		S3AccessKeyID:     "key",
		S3SecretAccessKey: "secret",
		S3PublicBaseURL:   "https://cdn.example.test",
	}
	if _, err := NewEntryPhotoStore(Config{}); err != ErrIncompleteConfig {
		t.Fatalf("empty config: error = %v, want ErrIncompleteConfig", err)
	}
	for name, mutate := range map[string]func(*Config){
		"endpoint":   func(c *Config) { c.S3Endpoint = "" },
		"bucket":     func(c *Config) { c.S3Bucket = "" },
		"access key": func(c *Config) { c.S3AccessKeyID = "" },
		"secret":     func(c *Config) { c.S3SecretAccessKey = "" },
		"public url": func(c *Config) { c.S3PublicBaseURL = "" },
	} {
		cfg := full
		mutate(&cfg)
		if _, err := NewEntryPhotoStore(cfg); err != ErrIncompleteConfig {
			t.Errorf("missing %s: error = %v, want ErrIncompleteConfig", name, err)
		}
	}
	// A complete configuration builds a store with no network call, and an
	// empty region is accepted.
	for _, region := range []string{"auto", ""} {
		cfg := full
		cfg.S3Region = region
		store, err := NewEntryPhotoStore(cfg)
		if err != nil || store == nil {
			t.Fatalf("region %q: %v", region, err)
		}
		if got := store.PublicURL("entries/x/y.jpg"); got != "https://cdn.example.test/entries/x/y.jpg" {
			t.Errorf("PublicURL = %q", got)
		}
	}
}
