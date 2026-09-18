// Package mail defines the outbound email abstraction: a provider-agnostic
// Sender, the concrete Mailer/Service that issues single-use verification
// tokens, and the Resend and log driver implementations. This package must
// not import internal/httpapi; it has no knowledge of HTTP request or
// response handling.
package mail

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// verificationTokenBytes is the amount of entropy read from crypto/rand for
// each verification token: 256 bits, matching internal/auth's refresh-token
// entropy.
const verificationTokenBytes = 32

// Mailer sends the account-verification email for u. internal/httpapi codes
// against this interface, matching the shape Service below satisfies, so a
// handler never depends on token-issuing mechanics directly.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, u *user.User) error
}

// Sender is the narrow transport contract a mail provider satisfies: given a
// fully composed message, deliver it. Keeping this separate from Mailer is
// what makes the provider swappable (RESEARCH.md Open Question 2): Service
// owns token issuance and message composition, while a Sender only knows how
// to hand a finished message to a transport.
type Sender interface {
	Send(ctx context.Context, to, subject, html, text string) error
}

// Service implements Mailer: it issues a single-use verification token,
// stores only its SHA-256 digest, and hands the composed message to a
// Sender.
type Service struct {
	repo    user.EmailVerificationRepository
	sender  Sender
	baseURL string
	ttl     time.Duration
}

// NewService constructs a Service. baseURL is the API's own public base URL
// (not the mobile deep link scheme): the emailed link always opens in a
// browser first, via the callback route the httpapi handler mounts.
func NewService(repo user.EmailVerificationRepository, sender Sender, baseURL string, ttl time.Duration) *Service {
	return &Service{repo: repo, sender: sender, baseURL: baseURL, ttl: ttl}
}

// SendVerificationEmail supersedes any outstanding token for u, issues a
// fresh one, and sends it. Only the token's SHA-256 digest is ever
// persisted; the plaintext token exists solely in the outgoing message, not
// in any stored field, so a leaked database cannot be used to verify
// accounts.
func (s *Service) SendVerificationEmail(ctx context.Context, u *user.User) error {
	if err := s.repo.DeleteForUser(ctx, u.ID); err != nil {
		return err
	}

	buf := make([]byte, verificationTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256(buf)

	if err := s.repo.Insert(ctx, u.ID, sum[:], time.Now().Add(s.ttl)); err != nil {
		return err
	}

	link := fmt.Sprintf("%s/v1/auth/verify-email/callback?token=%s", s.baseURL, token)
	subject, html, text := verificationMessage(link)
	return s.sender.Send(ctx, u.Email, subject, html, text)
}

// verificationMessage composes the verification email's subject, HTML body,
// and plain-text body around link. Copy is held to PROJECT.md's no-em-dash,
// no-AI-slop constraints, and matches plan 01-09's specified wording
// verbatim.
func verificationMessage(link string) (subject, html, text string) {
	subject = "Verify your email for RNDMRoll"
	body := "Tap the link below to verify your email and finish setting up your account."
	footer := "This link expires in 24 hours. If you did not sign up, you can ignore this message."
	text = fmt.Sprintf("%s\n\n%s\n\n%s", body, link, footer)
	html = fmt.Sprintf("<p>%s</p><p><a href=\"%s\">%s</a></p><p>%s</p>", body, link, link, footer)
	return subject, html, text
}
