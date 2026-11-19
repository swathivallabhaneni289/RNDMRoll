package secondlook

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeRepo records each Ask and answers made and err.
type fakeRepo struct {
	made  bool
	err   error
	calls []fakeAsk
}

type fakeAsk struct {
	owner, removal uuid.UUID
	body           string
	keys           []string
	at             time.Time
}

func (f *fakeRepo) Ask(_ context.Context, owner, removal uuid.UUID, body string, keys []string, at time.Time) (bool, error) {
	f.calls = append(f.calls, fakeAsk{owner, removal, body, keys, at})
	return f.made, f.err
}

// heard counts the admin emails asked for.
type heard struct{ n int }

func (h *heard) SecondLookAsked() { h.n++ }

// ownerClock is the owner's own clock: now, or err.
type ownerClock struct {
	now time.Time
	err error
}

func (c ownerClock) ViewerClock(context.Context, string) (time.Time, *time.Location, error) {
	return c.now, time.UTC, c.err
}

var (
	owner   = uuid.MustParse("6f1c2c4e-8a43-4bb1-9d55-5a0f0e3b2c11")
	other   = uuid.MustParse("0b9d1e2f-3c4a-4d5e-8f6a-7b8c9d0e1f2a")
	removal = uuid.MustParse("9a8b7c6d-5e4f-4a3b-9c2d-1e0f9a8b7c6d")
)

func key(of uuid.UUID, name string) string { return "entries/" + of.String() + "/" + name }

// The words are 1 to 500 characters after trimming, with line breaks as \n
// and no other control characters, and the photo keys at most 3, each the
// caller's own entry photo key, none twice. A refusal never reaches the
// store.
func TestAsk_RefusesWordsAndPhotosByTheRules(t *testing.T) {
	ok := key(owner, "0123456789abcdef.jpg")
	for name, tc := range map[string]struct {
		text   string
		keys   []string
		field  string
		reason string
	}{
		"no words":                   {"", nil, "text", "required"},
		"only spaces and line feeds": {" \r\n\t ", nil, "text", "required"},
		"501 characters":             {strings.Repeat("é", 501), nil, "text", "too_long"},
		"a control character":        {"It was\x07 a joke", nil, "text", "invalid"},
		"4 photos": {"Why", []string{key(owner, "0000000000000001.jpg"), key(owner, "0000000000000002.jpg"),
			key(owner, "0000000000000003.jpg"), key(owner, "0000000000000004.jpg")}, "photo_keys", "invalid"},
		"someone else's photo":   {"Why", []string{key(other, "0123456789abcdef.jpg")}, "photo_keys", "invalid"},
		"a profile photo":        {"Why", []string{"avatars/" + owner.String() + "/0123456789abcdef.jpg"}, "photo_keys", "invalid"},
		"the same photo twice":   {"Why", []string{ok, ok}, "photo_keys", "invalid"},
		"a key of another shape": {"Why", []string{key(owner, "../x.jpg")}, "photo_keys", "invalid"},
		"an empty key":           {"Why", []string{""}, "photo_keys", "invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			repo, h := &fakeRepo{made: true}, &heard{}
			svc := NewService(repo, ownerClock{now: time.Now()})
			svc.SetNotifier(h)
			_, err := svc.Ask(context.Background(), owner, removal, tc.text, tc.keys)
			var problem *FieldProblem
			if !errors.As(err, &problem) || problem.Field != tc.field || problem.Reason != tc.reason {
				t.Fatalf("Ask: %v, want %s %s", err, tc.field, tc.reason)
			}
			if len(repo.calls) != 0 || h.n != 0 {
				t.Fatalf("a refusal reached the store (%d) or the admins (%d)", len(repo.calls), h.n)
			}
		})
	}
}

// A request is stored trimmed, with its line breaks as \n, its photos in
// order (none as an empty list), and the owner's own now; the admins hear of
// it once, after the save, and never of a retry or of a refusal.
func TestAsk_StoresTheRequestAndTellsTheAdminsOnce(t *testing.T) {
	ownersNow := time.Date(2026, 11, 19, 21, 5, 0, 0, time.UTC)
	keys := []string{key(owner, "0000000000000002.png"), key(owner, "0000000000000001.webp"), key(owner, "0000000000000003.jpg")}
	repo, h := &fakeRepo{made: true}, &heard{}
	svc := NewService(repo, ownerClock{now: ownersNow})
	svc.SetNotifier(h)
	made, err := svc.Ask(context.Background(), owner, removal, "  It was a joke.\r\nAsk them.  ", keys)
	if err != nil || !made || h.n != 1 {
		t.Fatalf("Ask: made %v, %v, %d emails, want made, nil and 1", made, err, h.n)
	}
	got := repo.calls[0]
	if got.owner != owner || got.removal != removal || got.body != "It was a joke.\nAsk them." || !got.at.Equal(ownersNow) ||
		strings.Join(got.keys, ",") != strings.Join(keys, ",") {
		t.Fatalf("stored %+v", got)
	}
	if _, err := svc.Ask(context.Background(), owner, removal, strings.Repeat("é", 500), nil); err != nil {
		t.Fatalf("500 characters and no photos: %v", err)
	}
	if got := repo.calls[1].keys; got == nil || len(got) != 0 {
		t.Fatalf("no photos are stored as %#v, want an empty list", got)
	}

	repo.made = false
	if made, err := svc.Ask(context.Background(), owner, removal, "It was a joke.", nil); err != nil || made || h.n != 2 {
		t.Fatalf("a retry: made %v, %v, %d emails, want not made, nil and still 2", made, err, h.n)
	}
	repo.err = ErrAsked
	if _, err := svc.Ask(context.Background(), owner, removal, "Again", nil); !errors.Is(err, ErrAsked) || h.n != 2 {
		t.Fatalf("a second request: %v, %d emails, want ErrAsked and still 2", err, h.n)
	}

	// With no clock for the owner (no zone stored yet) the server's now is
	// kept, and with no notifier nobody is told.
	repo.made, repo.err = true, nil
	quiet := NewService(repo, ownerClock{err: errors.New("no zone")})
	before := time.Now()
	if made, err := quiet.Ask(context.Background(), owner, removal, "Why", nil); err != nil || !made {
		t.Fatalf("Ask with no zone: %v, %v", made, err)
	}
	if at := repo.calls[len(repo.calls)-1].at; at.Before(before) {
		t.Fatalf("with no zone the request is kept at %v, want the server's now", at)
	}
}
