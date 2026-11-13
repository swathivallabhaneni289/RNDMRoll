package storage

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// testLinkConfig is a complete storage configuration shaped like Supabase's
// (an endpoint with a path, a real region). Presigning makes no network call,
// so the real presign clients sign links offline in these tests.
var testLinkConfig = Config{
	S3Endpoint:        "https://project.storage.example.test/storage/v1/s3",
	S3Region:          "ap-southeast-1",
	S3Bucket:          "photos",
	S3AccessKeyID:     "test-access-key",
	S3SecretAccessKey: "test-secret-key",
}

// newLinkStore builds the real store with its link clock reading *at, so a
// test moves the clock by setting at.
func newLinkStore(t *testing.T, at *time.Time) *s3EntryPhotoStore {
	t.Helper()
	store, err := newEntryPhotoStore(testLinkConfig, func() time.Time { return *at })
	if err != nil {
		t.Fatal(err)
	}
	return store
}

// linkQuery returns the query of link, failing the test for no link.
func linkQuery(t *testing.T, link string) url.Values {
	t.Helper()
	if link == "" {
		t.Fatal("no link")
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("link %q does not parse: %v", link, err)
	}
	return u.Query()
}

// wantFreshUpload expects an upload link signed at the current time (within
// a minute of the real clock) that works for exactly five minutes.
func wantFreshUpload(t *testing.T, link string) {
	t.Helper()
	q := linkQuery(t, link)
	signedAt, err := time.Parse("20060102T150405Z", q.Get("X-Amz-Date"))
	if err != nil {
		t.Fatalf("the upload link's X-Amz-Date %q: %v", q.Get("X-Amz-Date"), err)
	}
	if d := time.Since(signedAt); d < -time.Minute || d > time.Minute {
		t.Fatalf("the upload link was signed at %s, %s from now, want the current time", signedAt, d)
	}
	if q.Get("X-Amz-Expires") != "300" || q.Get("X-Amz-Signature") == "" {
		t.Fatalf("the upload link works for %q seconds (signature %q), want 300 and a signature", q.Get("X-Amz-Expires"), q.Get("X-Amz-Signature"))
	}
}

// One key gives one link for a whole half hour, signed at its start for an
// hour, and a new link in the next half hour.
func TestLink_OneKeyGivesOneLinkPerHalfHour(t *testing.T) {
	key := "entries/" + uuid.NewString() + "/0123456789abcdef.jpg"
	at := time.Date(2026, 11, 9, 10, 30, 0, 0, time.UTC)
	store := newLinkStore(t, &at)

	first := store.Link(key)
	q := linkQuery(t, first)
	if q.Get("X-Amz-Date") != "20261109T103000Z" || q.Get("X-Amz-Expires") != "3600" || q.Get("X-Amz-Signature") == "" {
		t.Fatalf("query = %v, want signed at 10:30 for 3600 seconds", q)
	}
	for _, later := range []time.Time{
		time.Date(2026, 11, 9, 10, 30, 0, 1, time.UTC),
		time.Date(2026, 11, 9, 10, 44, 59, 0, time.UTC),
		time.Date(2026, 11, 9, 10, 59, 59, 999999999, time.UTC),
	} {
		at = later
		if got := store.Link(key); got != first {
			t.Fatalf("at %s: link %q, want the half hour's %q", later.Format(time.TimeOnly), got, first)
		}
	}

	at = time.Date(2026, 11, 9, 11, 0, 0, 0, time.UTC)
	next := store.Link(key)
	if next == first || linkQuery(t, next).Get("X-Amz-Date") != "20261109T110000Z" {
		t.Fatalf("at 11:00: link %q, want a new one signed at 11:00", next)
	}
}

// The half hours are UTC's, and the time only ever goes down: 16:29 in a
// zone 5 hours 45 minutes ahead is 10:44 UTC, signed at 10:30 UTC, not at
// the zone's own half hour (10:15 UTC).
func TestLink_RoundsDownToTheHalfHourInUTC(t *testing.T) {
	at := time.Date(2026, 11, 9, 16, 29, 0, 0, time.FixedZone("UTC+05:45", 5*3600+45*60))
	store := newLinkStore(t, &at)
	key := "avatars/" + uuid.NewString() + "/0123456789abcdef.jpg"
	if got := linkQuery(t, store.Link(key)).Get("X-Amz-Date"); got != "20261109T103000Z" {
		t.Fatalf("X-Amz-Date = %q, want 20261109T103000Z", got)
	}
}

// Post, comment and profile photos share the bucket, and Link signs each
// kind: a GET of the key in the bucket, on the endpoint, signed for the
// region, carrying the access key's id and never its secret.
func TestLink_SignsPostAndProfilePhotoKeys(t *testing.T) {
	at := time.Date(2026, 11, 9, 10, 31, 0, 0, time.UTC)
	store := newLinkStore(t, &at)
	id := uuid.NewString()
	for _, key := range []string{
		"entries/" + id + "/0123456789abcdef.jpg",
		"entries/" + id + "/fedcba9876543210.webp",
		"avatars/" + id + "/0123456789abcdef.jpg",
		"avatars/" + id + "/fedcba9876543210.heic",
		"avatars/" + id + "/0123456789abcdef.abcdefghij",
	} {
		link := store.Link(key)
		u, err := url.Parse(link)
		if err != nil || u.Scheme != "https" || u.Host != "project.storage.example.test" || u.Path != "/storage/v1/s3/photos/"+key {
			t.Errorf("%s: link %q, want the key in the bucket on the endpoint", key, link)
			continue
		}
		q := u.Query()
		if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" || q.Get("X-Amz-Credential") != "test-access-key/20261109/ap-southeast-1/s3/aws4_request" ||
			q.Get("X-Amz-SignedHeaders") != "host" || q.Get("X-Amz-Signature") == "" {
			t.Errorf("%s: query %v, want a GET signed for ap-southeast-1", key, q)
		}
		if strings.Contains(link, "test-secret-key") {
			t.Errorf("%s: the link carries the secret: %s", key, link)
		}
	}
}

// Link signs nothing but a key of the two shapes: anything else is "", with
// nothing sent to the signer.
func TestLink_GivesNothingForAnyOtherKey(t *testing.T) {
	at := time.Date(2026, 11, 9, 10, 31, 0, 0, time.UTC)
	store := newLinkStore(t, &at)
	id := uuid.NewString()
	signed := store.Link("avatars/" + id + "/0123456789abcdef.jpg")
	for name, key := range map[string]string{
		"empty":                     "",
		"a full URL":                "https://cdn.example.test/entries/" + id + "/0123456789abcdef.jpg",
		"a signed link":             signed,
		"a dot dot segment":         "entries/" + id + "/../" + id + "/0123456789abcdef.jpg",
		"dot dot as the file":       "avatars/" + id + "/..",
		"avatars/x":                 "avatars/x",
		"the avatar folder":         "avatars/",
		"the folder of a user":      "avatars/" + id + "/",
		"another folder":            "backups/" + id + "/0123456789abcdef.jpg",
		"upper case hex":            "entries/" + id + "/0123456789ABCDEF.jpg",
		"upper case avatar hex":     "avatars/" + id + "/0123456789ABCDEF.jpg",
		"a 15 character name":       "avatars/" + id + "/0123456789abcde.jpg",
		"a 15 character entry name": "entries/" + id + "/0123456789abcde.jpg",
		"a 17 character name":       "avatars/" + id + "/0123456789abcdef0.jpg",
		"an upper case user id":     "avatars/" + strings.ToUpper(id) + "/0123456789abcdef.jpg",
		"not a user id":             "avatars/not-a-user/0123456789abcdef.jpg",
		"an extra slash":            "avatars/" + id + "/x/0123456789abcdef.jpg",
		"a leading slash":           "/avatars/" + id + "/0123456789abcdef.jpg",
		"a query":                   "avatars/" + id + "/0123456789abcdef.jpg?x=1",
		"a type entries never take": "entries/" + id + "/0123456789abcdef.gif",
		"an extension over ten":     "avatars/" + id + "/0123456789abcdef.abcdefghijk",
		"an upper case extension":   "avatars/" + id + "/0123456789abcdef.JPG",
		"two extensions":            "avatars/" + id + "/0123456789abcdef.jpg.png",
		"no extension":              "avatars/" + id + "/0123456789abcdef",
		"a trailing dot":            "avatars/" + id + "/0123456789abcdef.",
		"a trailing line break":     "avatars/" + id + "/0123456789abcdef.jpg\n",
	} {
		if got := store.Link(key); got != "" {
			t.Errorf("%s: Link(%q) = %q, want none", name, key, got)
		}
	}
}

// stubGetPresigner stands in for the link client.
type stubGetPresigner struct {
	req   *v4.PresignedHTTPRequest
	err   error
	calls int
}

func (p *stubGetPresigner) PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	p.calls++
	return p.req, p.err
}

// A signing error, no answer and an answer with no signature all give "":
// Link never hands out an unsigned link.
func TestLink_GivesNothingWhenSigningFails(t *testing.T) {
	key := "avatars/" + uuid.NewString() + "/0123456789abcdef.png"
	for name, p := range map[string]*stubGetPresigner{
		"an error":                   {err: errors.New("signer is down")},
		"no answer":                  {},
		"an unsigned link":           {req: &v4.PresignedHTTPRequest{URL: "https://project.storage.example.test/storage/v1/s3/photos/" + key, Method: "GET"}},
		"a link that does not parse": {req: &v4.PresignedHTTPRequest{URL: "https://bad host/?X-Amz-Signature=abc", Method: "GET"}},
	} {
		store := &s3EntryPhotoStore{links: p, bucket: "photos"}
		if got := store.Link(key); got != "" || p.calls != 1 {
			t.Errorf("%s: Link = %q after %d calls, want none after one", name, got, p.calls)
		}
	}
}
