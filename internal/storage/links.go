package storage

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Photo links (developer, 2026-11-09: photos in a private bucket). Storage
// answers nothing to an unsigned request, so every photo the app shows is a
// presigned GET link the server signs when it reads, and only for someone the
// callers have already allowed to see that photo. A link works for linkLife
// after its signing time, and the signing time is rounded down to linkWindow
// (UTC), so:
//   - one photo gets the same link for a whole window, and the app's image
//     cache sees no change;
//   - every link has more than linkLife - linkWindow (30 minutes) left when it
//     is handed out: the app reloads data older than 20 minutes, which must
//     stay under that;
//   - no link works more than linkLife after it was handed out ("within an
//     hour", the words of the Privacy Policy).
//
// Both are constants on purpose, not settings: the legal text promises them.
const (
	// linkLife is how long a photo link works after its signing time
	// (X-Amz-Expires=3600).
	linkLife = 60 * time.Minute
	// linkWindow is the step the signing time of a photo link is rounded
	// down to.
	linkWindow = 30 * time.Minute
)

// getPresigner is the seam for photo links. It is kept apart from presigner
// (the upload tickets') so the upload stubs need no GET method.
type getPresigner interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// The link client satisfies the seam, and roundedSigner the SDK's presigner.
var _ getPresigner = (*s3.PresignClient)(nil)
var _ s3.HTTPPresignerV4 = roundedSigner{}

// roundedSigner is the presigner of the link client. The SDK always passes
// the current time to PresignHTTP; roundedSigner signs with now rounded down
// to linkWindow in UTC instead, so the same key gives the same link for the
// whole window. It must never sign an upload: a 5-minute upload link signed up
// to 30 minutes in the past would be expired before it was handed out.
type roundedSigner struct {
	signer *v4.Signer
	now    func() time.Time
}

// PresignHTTP signs r with the rounded time in place of the one it is given.
func (r roundedSigner) PresignHTTP(ctx context.Context, credentials aws.Credentials, req *http.Request,
	payloadHash, service, region string, _ time.Time, optFns ...func(*v4.SignerOptions)) (string, http.Header, error) {
	return r.signer.PresignHTTP(ctx, credentials, req, payloadHash, service, region, r.now().UTC().Truncate(linkWindow), optFns...)
}

// newLinkClient builds the presign client for photo links over client. It is
// a second presign client, used only for GET links and never for uploads:
// every link it makes is signed by roundedSigner on now's clock and works for
// linkLife. Like the SDK's own S3 signer, the signer leaves the key's path
// unescaped, so the signature covers the path storage checks.
func newLinkClient(client *s3.Client, now func() time.Time) *s3.PresignClient {
	signer := v4.NewSigner(func(o *v4.SignerOptions) {
		o.DisableURIPathEscaping = true
	})
	return s3.NewPresignClient(client, func(o *s3.PresignOptions) {
		o.Presigner = roundedSigner{signer: signer, now: now}
		o.Expires = linkLife
	})
}

// Link returns the photo link of key: a presigned GET link to the file, from
// the link client. Post, comment and profile photos share one bucket, so it
// signs a key shaped like an entry photo key (isEntryPhotoKey) or a profile
// photo key (isAvatarKey), of any account, and nothing else. Who may see the
// photo is decided before it is asked. An empty or unexpected key, and a
// signing error, give "": Link never returns a bare key or an unsigned link.
// Presigning is local math with no network call, so no request context is
// needed.
func (s *s3EntryPhotoStore) Link(key string) string {
	if !isEntryPhotoKey(key) && !isAvatarKey(key) {
		return ""
	}
	req, err := s.links.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil || req == nil || !isSigned(req.URL) {
		return ""
	}
	return req.URL
}

// isSigned reports whether link carries a signature, the last check before a
// link leaves the server.
func isSigned(link string) bool {
	u, err := url.Parse(link)
	return err == nil && u.Query().Get("X-Amz-Signature") != ""
}
