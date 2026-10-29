// Package adminlink makes and reads the one-time links that open the admin
// page without a password (developer, 2026-10-29: "make it easy to access the
// admin page"). `make admin` (cmd/admin-link) makes one and opens it in the
// browser; the admin page (httpapi AdminHandler.openLink) reads it.
//
// A link is signed with JWT_SECRET, the secret the admin page's cookie is
// signed with, so whoever can make a link could already make that cookie: a
// link opens nothing new, it only saves the password. It works for TTL, and
// once (the admin page remembers the nonces of the links it has opened).
package adminlink

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TTL is how long a link works.
const TTL = 2 * time.Minute

// MinSecretBytes is the shortest secret a link is signed with: JWT_SECRET's
// own minimum (internal/config).
const MinSecretBytes = 32

// ErrBad is every link that does not work: malformed, signed with another
// secret, ended, or ending further away than TTL.
var ErrBad = errors.New("adminlink: bad or expired link")

// Link is what a link says: the admin it opens the page for ("" is the first
// admin the server found when it started), its nonce, and when it ends.
type Link struct {
	Email   string
	Nonce   string
	Expires time.Time
}

// New signs a link for email ("" for the first admin) that ends TTL after now.
func New(secret []byte, email string, now time.Time) (string, error) {
	if len(secret) < MinSecretBytes {
		return "", fmt.Errorf("adminlink: the secret is shorter than %d bytes", MinSecretBytes)
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString([]byte(email)) + "." +
		strconv.FormatInt(now.Add(TTL).Unix(), 10) + "." +
		base64.RawURLEncoding.EncodeToString(nonce)
	return payload + "." + sign(secret, payload), nil
}

// Read checks a link's signature and end at now, and returns what it says. A
// link that ends more than a minute past TTL from now is refused as well: no
// link New makes does, unless the two clocks disagree by over a minute.
func Read(secret []byte, token string, now time.Time) (Link, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[2] == "" {
		return Link{}, ErrBad
	}
	payload := parts[0] + "." + parts[1] + "." + parts[2]
	if !hmac.Equal([]byte(parts[3]), []byte(sign(secret, payload))) {
		return Link{}, ErrBad
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return Link{}, ErrBad
	}
	end := time.Unix(exp, 0)
	if !now.Before(end) || end.After(now.Add(TTL+time.Minute)) {
		return Link{}, ErrBad
	}
	email, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Link{}, ErrBad
	}
	return Link{Email: string(email), Nonce: parts[2], Expires: end}, nil
}

// sign is the admin page's signature (httpapi AdminHandler.sign) for the
// purpose "link", so a link never passes as the cookie or a CSRF token.
func sign(secret []byte, payload string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("rndmroll-admin-link:" + payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
