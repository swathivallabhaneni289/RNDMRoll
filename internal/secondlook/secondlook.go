// Package secondlook is asking for a second look at a post or comment an
// admin removed (developer, 2026-11-19: "you can request another ... they
// can upload their file or something, and the proofs"). Its owner writes why
// it should come back and may add up to 3 photos, screenshots too: once per
// removal, and only inside admin.RemovalGrace. The admin answers on the
// admin page with Bring it back or Keep it removed, and the bell tells the
// owner. A request emails the admins at once (internal/adminalert), never
// with the words or the photos.
package secondlook

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
)

// Limits (developer, 2026-11-19).
const (
	// MaxRunes is the most characters the words may have, after trimming.
	MaxRunes = 500
	// MaxPhotos is the most photos a request may carry.
	MaxPhotos = 3
)

// The refusals, each one the app words in a few words.
var (
	// ErrNotFound is a removal that is not the caller's, not a removal of a
	// post or comment, or not there, and one whose post or comment was
	// deleted another way before the 30 days were over (its post, say).
	// The same answer for all, so nothing leaks.
	ErrNotFound = errors.New("secondlook: not found")
	// ErrAsked is a removal with a request already: one each.
	ErrAsked = errors.New("secondlook: asked already")
	// ErrTooLate is a removal past admin.RemovalGrace, purged or not yet.
	ErrTooLate = errors.New("secondlook: too late")
	// ErrBack is a removal a Bring it back answered already.
	ErrBack = errors.New("secondlook: brought back already")
)

// FieldProblem is a request body field the rules refuse: Field is "text" or
// "photo_keys", Reason "required", "too_long" or "invalid".
type FieldProblem struct {
	Field, Reason string
}

func (p *FieldProblem) Error() string { return "secondlook: " + p.Field + " " + p.Reason }

// Repository stores a request (postgres.SecondLookRepo).
type Repository interface {
	// Ask stores owner's request about the removal (a decision log row) in
	// one transaction, with words body and the photo files keys, asked at
	// at on the owner's own clock. It takes turns with the owner's other
	// requests and holds the post or comment for share, so a Bring it back
	// or the purge waits for it or goes first, and then refuses as the
	// errors above say. A key that a post's photo, a comment's or another
	// request uses already is FieldProblem photo_keys invalid. made is false when this same request (the same words and
	// photos) is stored already: the phone's retry after its 10 seconds.
	Ask(ctx context.Context, owner, removal uuid.UUID, body string, keys []string, at time.Time) (made bool, err error)
}

// Notifier hears of each new request, after it is saved
// (*adminalert.Alerts).
type Notifier interface {
	SecondLookAsked()
}

// Clock is the owner's own clock (spin.Service), which the request keeps as
// the decision log keeps its rows'.
type Clock interface {
	ViewerClock(ctx context.Context, userID string) (time.Time, *time.Location, error)
}

// Service checks and stores requests.
type Service struct {
	repo     Repository
	clk      Clock
	notifier Notifier
}

// NewService builds a Service.
func NewService(repo Repository, clk Clock) *Service {
	return &Service{repo: repo, clk: clk}
}

// SetNotifier sets who hears of each new request (the admin email). Call it
// before the service is used; nil hears nothing.
func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

// Ask checks owner's words and photo keys and stores the request about the
// removal. made is false for a retry of a request stored already, which
// changes nothing and tells no one again.
func (s *Service) Ask(ctx context.Context, owner, removal uuid.UUID, text string, keys []string) (made bool, err error) {
	body, problem := CleanText(text)
	if problem != nil {
		return false, problem
	}
	if problem := checkKeys(keys, owner); problem != nil {
		return false, problem
	}
	if keys == nil {
		keys = []string{}
	}
	at := time.Now()
	if s.clk != nil {
		if now, _, err := s.clk.ViewerClock(ctx, owner.String()); err == nil {
			at = now
		}
	}
	made, err = s.repo.Ask(ctx, owner, removal, body, keys, at)
	if err != nil {
		return false, err
	}
	if made && s.notifier != nil {
		s.notifier.SecondLookAsked()
	}
	return made, nil
}

// CleanText is the words as they are kept, by the comment's rules but up to
// MaxRunes: line breaks as \n, trimmed, required, and no other control
// characters.
func CleanText(s string) (string, *FieldProblem) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if s == "" {
		return "", &FieldProblem{Field: "text", Reason: "required"}
	}
	if utf8.RuneCountInString(s) > MaxRunes {
		return "", &FieldProblem{Field: "text", Reason: "too_long"}
	}
	for _, r := range s {
		if r != '\n' && (unicode.IsControl(r) || r == utf8.RuneError) {
			return "", &FieldProblem{Field: "text", Reason: "invalid"}
		}
	}
	return s, nil
}

// checkKeys refuses more than MaxPhotos keys, one twice, and any that is not
// an entry photo key in owner's own folder (POST /entry-photos/upload-url
// makes those): another person's file, a profile photo, or anything else.
func checkKeys(keys []string, owner uuid.UUID) *FieldProblem {
	if len(keys) > MaxPhotos {
		return &FieldProblem{Field: "photo_keys", Reason: "invalid"}
	}
	for i, k := range keys {
		if !storage.EntryPhotoKeyOwnedBy(k, owner) {
			return &FieldProblem{Field: "photo_keys", Reason: "invalid"}
		}
		for _, other := range keys[:i] {
			if other == k {
				return &FieldProblem{Field: "photo_keys", Reason: "invalid"}
			}
		}
	}
	return nil
}
