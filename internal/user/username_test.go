package user

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// fakeUsernameRepo is a minimal in-memory Repository fake scoped to this
// file's tests: only UsernameTaken is exercised by username.go, so every
// other method is a harmless stub.
type fakeUsernameRepo struct {
	taken map[string]bool
}

func newFakeUsernameRepo(takenUsernames ...string) *fakeUsernameRepo {
	m := make(map[string]bool, len(takenUsernames))
	for _, u := range takenUsernames {
		m[strings.ToLower(u)] = true
	}
	return &fakeUsernameRepo{taken: m}
}

func (f *fakeUsernameRepo) Create(ctx context.Context, email string, passwordHash *string, verified bool, via *VerificationSource) (*User, error) {
	return nil, ErrNotFound
}

func (f *fakeUsernameRepo) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return nil, ErrNotFound
}

func (f *fakeUsernameRepo) GetByEmailCI(ctx context.Context, email string) (*User, error) {
	return nil, ErrNotFound
}

func (f *fakeUsernameRepo) GetByProviderSubject(ctx context.Context, provider VerificationSource, subject string) (*User, error) {
	return nil, ErrNotFound
}

func (f *fakeUsernameRepo) LinkProviderSubject(ctx context.Context, id uuid.UUID, provider VerificationSource, subject string) error {
	return ErrNotFound
}

func (f *fakeUsernameRepo) UsernameTaken(ctx context.Context, username string) (bool, error) {
	return f.taken[strings.ToLower(username)], nil
}

func (f *fakeUsernameRepo) MarkEmailVerified(ctx context.Context, id uuid.UUID, via VerificationSource) error {
	return ErrNotFound
}

func (f *fakeUsernameRepo) UpdateProfile(ctx context.Context, id uuid.UUID, p ProfilePatch) (*User, error) {
	return nil, ErrNotFound
}

func (f *fakeUsernameRepo) CreateComplete(ctx context.Context, in NewAccount) (*User, error) {
	return nil, ErrNotFound
}

func (f *fakeUsernameRepo) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	return false, ErrNotFound
}

func (f *fakeUsernameRepo) ClaimAndRevoke(ctx context.Context, id uuid.UUID, via VerificationSource) (bool, error) {
	return false, ErrNotFound
}

var _ Repository = (*fakeUsernameRepo)(nil)

func TestUsername_NormalizeStripsSpacesCapsAndPunctuation(t *testing.T) {
	got := NormalizeUsername("Swathi V. Rao!!")
	if got != usernamePattern.FindString(got) {
		t.Fatalf("normalized username %q does not match the charset pattern", got)
	}
	if got != strings.ToLower(got) {
		t.Fatalf("expected lowercase, got %q", got)
	}
	if strings.ContainsAny(got, " .!") {
		t.Fatalf("expected punctuation and spaces stripped, got %q", got)
	}
}

func TestUsername_NormalizeFallsBackToUsableBaseWhenEmpty(t *testing.T) {
	got := NormalizeUsername("!!!???")
	if got != fallbackUsernameBase {
		t.Fatalf("expected fallback base %q for a name with no valid characters, got %q", fallbackUsernameBase, got)
	}

	// A name written entirely in a non-Latin script strips to nothing
	// under the [a-z0-9_] charset and must still yield a usable handle.
	nonLatin := NormalizeUsername("田中太郎")
	if nonLatin != fallbackUsernameBase {
		t.Fatalf("expected fallback base %q for a non-Latin-only name, got %q", fallbackUsernameBase, nonLatin)
	}
	if err := ValidateUsername(nonLatin); err != nil {
		t.Fatalf("fallback base must itself be a valid handle: %v", err)
	}
}

func TestUsername_NormalizeTruncatesLongNamesRatherThanFailing(t *testing.T) {
	longName := strings.Repeat("a", 100)
	got := NormalizeUsername(longName)
	if len(got) != usernameMaxLength {
		t.Fatalf("expected truncation to %d characters, got length %d (%q)", usernameMaxLength, len(got), got)
	}
	if err := ValidateUsername(got); err != nil {
		t.Fatalf("truncated username must be valid: %v", err)
	}
}

func TestUsername_ValidateRejectsUppercasePunctuationAndBadLengths(t *testing.T) {
	cases := []string{
		"Swathi",                // uppercase
		"sw-athi",               // punctuation
		"ab",                    // too short
		strings.Repeat("a", 21), // too long
		"",                      // empty
	}
	for _, c := range cases {
		if err := ValidateUsername(c); err == nil {
			t.Errorf("expected ValidateUsername(%q) to reject, got nil error", c)
		}
	}
	if err := ValidateUsername("swathi_v1"); err != nil {
		t.Errorf("expected a valid username to pass, got %v", err)
	}
}

func TestUsername_SuggestReturnsPlainBaseWhenFree(t *testing.T) {
	repo := newFakeUsernameRepo()
	got, err := SuggestUsername(context.Background(), repo, "Swathi Vallabhaneni")
	if err != nil {
		t.Fatalf("SuggestUsername returned error: %v", err)
	}
	want := NormalizeUsername("Swathi Vallabhaneni")
	if got != want {
		t.Fatalf("expected the free base %q, got %q", want, got)
	}
}

func TestUsername_SuggestReturnsSuffixedVariantWhenBaseTakenAndSuffixIsNotPredictable(t *testing.T) {
	base := NormalizeUsername("Swathi")
	repo := newFakeUsernameRepo(base)

	first, err := SuggestUsername(context.Background(), repo, "Swathi")
	if err != nil {
		t.Fatalf("SuggestUsername returned error: %v", err)
	}
	if first == base {
		t.Fatalf("expected a suffixed variant since %q is taken, got the bare base", base)
	}
	if !strings.HasPrefix(first, base) {
		t.Fatalf("expected suffixed variant to retain the base %q as a prefix, got %q", base, first)
	}

	second, err := SuggestUsername(context.Background(), repo, "Swathi")
	if err != nil {
		t.Fatalf("SuggestUsername returned error: %v", err)
	}
	if first == second {
		t.Fatalf("expected two suggestions for the same taken base to differ (non-predictable suffix), got %q both times", first)
	}
}

func TestUsername_SuggestAlternatesReturnsThreeDistinctFreeCandidates(t *testing.T) {
	base := NormalizeUsername("Swathi")
	repo := newFakeUsernameRepo(base)

	alternates, err := SuggestAlternates(context.Background(), repo, base, 3)
	if err != nil {
		t.Fatalf("SuggestAlternates returned error: %v", err)
	}
	if len(alternates) != 3 {
		t.Fatalf("expected 3 alternates, got %d (%v)", len(alternates), alternates)
	}

	seen := make(map[string]bool, 3)
	for _, a := range alternates {
		if seen[a] {
			t.Fatalf("expected distinct alternates, got a duplicate: %q in %v", a, alternates)
		}
		seen[a] = true
		if err := ValidateUsername(a); err != nil {
			t.Errorf("alternate %q is not a valid username: %v", a, err)
		}
		if taken, _ := repo.UsernameTaken(context.Background(), a); taken {
			t.Errorf("alternate %q must be free, but the fake repo reports it taken", a)
		}
	}
}

func TestUsername_ValidateErrorNeverEchoesInput(t *testing.T) {
	input := "Not-A-Valid-Handle!"
	err := ValidateUsername(input)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), input) {
		t.Errorf("error text %q echoes the input", err.Error())
	}
}
