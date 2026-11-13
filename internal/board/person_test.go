package board

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestPersonCursorRoundTripAndRefusals(t *testing.T) {
	for _, d := range []string{"2026-10-08", "2028-02-29"} {
		got, err := DecodePersonCursor(EncodePersonCursor(d))
		if err != nil || got != d {
			t.Fatalf("round trip %s: %q %v", d, got, err)
		}
	}
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	for _, bad := range []string{
		// Empty, not base64, and the old cursor (an entry id).
		"", "nope", "0d3f1c2e-8a4b-4c6d-9e0f-1a2b3c4d5e6f",
		// The diary's cursor is not this list's.
		b64("v1:2026-10-08"),
		// Not a real date, a short one, padded base64, too long.
		b64("person1:2026-13-01"), b64("person1:2026-02-30"), b64("person1:2026-10-0"),
		base64.URLEncoding.EncodeToString([]byte("person1:2026-10-0")), strings.Repeat("a", 200),
	} {
		if _, err := DecodePersonCursor(bad); !errors.Is(err, ErrBadCursor) {
			t.Errorf("cursor %q: %v, want ErrBadCursor", bad, err)
		}
	}
}

// gridRepo is a Repository whose PersonEntries hands back the rows that
// match and keeps what it was asked; PersonPosts reads nothing else.
type gridRepo struct {
	Repository
	rows   []EntryRow
	before *string
	limit  int
}

func (r *gridRepo) PersonEntries(_ context.Context, _, _ uuid.UUID, before *string, limit int) ([]EntryRow, error) {
	r.before, r.limit = before, limit
	out := []EntryRow{}
	for _, e := range r.rows {
		if (before == nil || e.LocalDate < *before) && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

// TestPersonPostsCursorIsTheLastTilesDate: the next page starts from a date,
// never from an entry, so it needs nothing of the page before to still exist.
func TestPersonPostsCursorIsTheLastTilesDate(t *testing.T) {
	repo := &gridRepo{rows: []EntryRow{
		{ID: uuid.New(), LocalDate: "2026-10-08"},
		{ID: uuid.New(), LocalDate: "2026-10-07"},
		{ID: uuid.New(), LocalDate: "2026-10-06"},
	}}
	s := NewService(repo, nil, nil, nil, nil)
	viewer, owner := uuid.New(), uuid.New()

	posts, next, err := s.PersonPosts(context.Background(), viewer, owner, nil, 2)
	if err != nil || len(posts) != 2 || posts[1].LocalDate != "2026-10-07" || next == nil || *next != "2026-10-07" {
		t.Fatalf("page 1: %+v next %v err %v", posts, next, err)
	}
	if repo.before != nil || repo.limit != 3 {
		t.Fatalf("page 1 asked before %v limit %d, want nil and one more than the page", repo.before, repo.limit)
	}

	// The tile the cursor came from is gone: the date still places the page.
	repo.rows = append(repo.rows[:1], repo.rows[2:]...)
	posts, next, err = s.PersonPosts(context.Background(), viewer, owner, next, 2)
	if err != nil || len(posts) != 1 || posts[0].LocalDate != "2026-10-06" || next != nil {
		t.Fatalf("page 2: %+v next %v err %v", posts, next, err)
	}
	if repo.before == nil || *repo.before != "2026-10-07" {
		t.Fatalf("page 2 asked before %v, want 2026-10-07", repo.before)
	}
}
