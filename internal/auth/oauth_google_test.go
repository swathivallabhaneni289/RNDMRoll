package auth

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/api/idtoken"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// newGooglePayload builds a stub idtoken.Payload the way Google's own
// idtoken.Validate would return one on success -- Claims carries every raw
// JWT claim (via the library's own json.Unmarshal into a map), which is
// where Verify reads email, email_verified, and name from.
func newGooglePayload(sub, email string, emailVerified bool, name, audience string) *idtoken.Payload {
	return &idtoken.Payload{
		Issuer:   "https://accounts.google.com",
		Audience: audience,
		Subject:  sub,
		Claims: map[string]interface{}{
			"sub":            sub,
			"email":          email,
			"email_verified": emailVerified,
			"name":           name,
			"aud":            audience,
		},
	}
}

func TestGoogle_ValidAudienceAndSignatureReturnsIdentity(t *testing.T) {
	v := &googleVerifier{
		clientIDs: []string{"ios-client-id", "android-client-id", "web-client-id"},
		validate: func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error) {
			if audience != "web-client-id" {
				return nil, errors.New("idtoken: audience provided does not match aud claim in the JWT")
			}
			return newGooglePayload("google-subject-1", "person@example.com", true, "Person Name", audience), nil
		},
	}

	identity, err := v.Verify(context.Background(), "token")
	if err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}
	if identity.Subject != "google-subject-1" {
		t.Errorf("Subject = %q, want google-subject-1", identity.Subject)
	}
	if identity.Email != "person@example.com" {
		t.Errorf("Email = %q, want person@example.com", identity.Email)
	}
	if !identity.EmailVerified {
		t.Error("EmailVerified = false, want true")
	}
	if identity.Name != "Person Name" {
		t.Errorf("Name = %q, want Person Name", identity.Name)
	}
}

func TestGoogle_TriesEveryConfiguredClientIDBeforeRejecting(t *testing.T) {
	var attempted []string
	v := &googleVerifier{
		clientIDs: []string{"a", "b", "c"},
		validate: func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error) {
			attempted = append(attempted, audience)
			return nil, errors.New("idtoken: audience provided does not match aud claim in the JWT")
		},
	}

	_, err := v.Verify(context.Background(), "token")
	if !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
	if len(attempted) != 3 {
		t.Fatalf("tried %d client IDs, want 3: %v", len(attempted), attempted)
	}
}

func TestGoogle_AudienceMatchingNoneIsRejected(t *testing.T) {
	v := &googleVerifier{
		clientIDs: []string{"expected-id"},
		validate: func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error) {
			return nil, errors.New("idtoken: audience provided does not match aud claim in the JWT")
		},
	}

	_, err := v.Verify(context.Background(), "token")
	if !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
}

func TestGoogle_InvalidSignatureIsRejected(t *testing.T) {
	v := &googleVerifier{
		clientIDs: []string{"expected-id"},
		validate: func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error) {
			return nil, errors.New("idtoken: signature verification failed")
		},
	}

	_, err := v.Verify(context.Background(), "token")
	if !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
}

func TestGoogle_ExpiredTokenIsRejected(t *testing.T) {
	v := &googleVerifier{
		clientIDs: []string{"expected-id"},
		validate: func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error) {
			return nil, errors.New("idtoken: token expired: now=100, expires=1")
		},
	}

	_, err := v.Verify(context.Background(), "token")
	if !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
}

func TestGoogle_UnverifiedEmailIsRejected(t *testing.T) {
	v := &googleVerifier{
		clientIDs: []string{"expected-id"},
		validate: func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error) {
			return newGooglePayload("google-subject-2", "person@example.com", false, "Name", audience), nil
		},
	}

	_, err := v.Verify(context.Background(), "token")
	if !errors.Is(err, user.ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
}

func TestGoogle_NewGoogleVerifierWiresRealValidateFunc(t *testing.T) {
	v := NewGoogleVerifier([]string{"client-id"})
	gv, ok := v.(*googleVerifier)
	if !ok {
		t.Fatalf("NewGoogleVerifier returned %T, want *googleVerifier", v)
	}
	if gv.validate == nil {
		t.Fatal("NewGoogleVerifier did not wire a validate func")
	}
	if len(gv.clientIDs) != 1 || gv.clientIDs[0] != "client-id" {
		t.Fatalf("clientIDs = %v, want [client-id]", gv.clientIDs)
	}
}
