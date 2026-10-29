package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// fakeAccounts is the user store in memory. set keeps the hash SetPassword
// stored, and setErr is what SetPassword answers instead.
type fakeAccounts struct {
	users  []*user.User
	set    map[uuid.UUID]string
	setErr error
}

func (f *fakeAccounts) GetByEmailCI(_ context.Context, email string) (*user.User, error) {
	for _, u := range f.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return nil, user.ErrNotFound
}

func (f *fakeAccounts) GetByUsernameCI(_ context.Context, username string) (*user.User, error) {
	for _, u := range f.users {
		if u.Username != nil && strings.EqualFold(*u.Username, username) {
			return u, nil
		}
	}
	return nil, user.ErrNotFound
}

func (f *fakeAccounts) SetPassword(_ context.Context, id uuid.UUID, hash string) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.set == nil {
		f.set = map[uuid.UUID]string{}
	}
	f.set[id] = hash
	return nil
}

// script answers the prompts in order and keeps them; past its answers it
// answers as Ctrl+D does.
type script struct {
	lines, secrets []string
	asked          []string
}

func (s *script) Line(prompt string) (string, error) {
	s.asked = append(s.asked, prompt)
	if len(s.lines) == 0 {
		return "", io.EOF
	}
	line := s.lines[0]
	s.lines = s.lines[1:]
	return line, nil
}

func (s *script) Secret(prompt string) (string, error) {
	s.asked = append(s.asked, prompt)
	if len(s.secrets) == 0 {
		return "", io.EOF
	}
	secret := s.secrets[0]
	s.secrets = s.secrets[1:]
	return secret, nil
}

const goodPassword = "Fresh-start9"

func newStore() (*fakeAccounts, *user.User) {
	name := "pat_rolls"
	u := &user.User{ID: uuid.New(), Email: "pat@example.com", Username: &name}
	return &fakeAccounts{users: []*user.User{u}}, u
}

// The account is found by its email in any case, its username or @username,
// and the stored hash is one log in accepts for the typed password, which is
// never printed.
func TestRun_SetsAPasswordLogInAccepts(t *testing.T) {
	for _, account := range []string{"pat@example.com", "PAT@Example.COM", "pat_rolls", "@Pat_Rolls", "  pat_rolls  "} {
		store, u := newStore()
		ask := &script{secrets: []string{goodPassword, goodPassword}}
		var out strings.Builder
		if err := run(context.Background(), store, ask, &out, account); err != nil {
			t.Fatalf("%q: %v", account, err)
		}
		hash, ok := store.set[u.ID]
		if !ok {
			t.Fatalf("%q: no password was set", account)
		}
		if err := auth.ComparePassword(hash, goodPassword); err != nil {
			t.Fatalf("%q: log in would refuse the new password: %v", account, err)
		}
		got := out.String()
		if !strings.Contains(got, "A new password for @pat_rolls.\n") || !strings.Contains(got, "Done. @pat_rolls has a new password.") {
			t.Fatalf("%q: printed %q", account, got)
		}
		if strings.Contains(got, goodPassword) {
			t.Fatalf("%q: the password was printed", account)
		}
	}
}

// With no account given, it asks for one, then for the password twice.
func TestRun_AsksForTheAccountWhenNoneIsGiven(t *testing.T) {
	store, u := newStore()
	ask := &script{lines: []string{"pat_rolls"}, secrets: []string{goodPassword, goodPassword}}
	if err := run(context.Background(), store, ask, io.Discard, ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.set[u.ID]; !ok {
		t.Fatal("no password was set")
	}
	want := []string{"Email or username: ", "New password: ", "Type it again: "}
	if strings.Join(ask.asked, "|") != strings.Join(want, "|") {
		t.Fatalf("asked %q, want %q", ask.asked, want)
	}
}

// An account without a username yet is named by its email.
func TestRun_NamesAnAccountWithoutAUsernameByItsEmail(t *testing.T) {
	store, u := newStore()
	u.Username = nil
	ask := &script{secrets: []string{goodPassword, goodPassword}}
	var out strings.Builder
	if err := run(context.Background(), store, ask, &out, "pat@example.com"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "A new password for pat@example.com.") {
		t.Fatalf("printed %q", out.String())
	}
}

// Every refusal says why, ends with "Nothing changed" and stores nothing. A
// refused account is refused before any password is asked for, and a
// password the rule refuses before it is asked for again.
func TestRun_RefusalsChangeNothing(t *testing.T) {
	tooManyBytes := strings.Repeat("é", 40) + "A1!" // 43 characters, 83 bytes
	cases := []struct {
		name    string
		account string
		secrets []string
		edit    func(*fakeAccounts, *user.User)
		want    string
		asked   int
	}{
		{name: "no account given", want: "no account given", asked: 1},
		{name: "unknown email", account: "nobody@example.com", want: "no account has that email or username"},
		{name: "unknown username", account: "nobody_here", want: "no account has that email or username"},
		{name: "not a username", account: "no spaces!", want: "no account has that email or username"},
		{name: "disabled", account: "pat_rolls", edit: func(_ *fakeAccounts, u *user.User) { u.Disabled = true },
			want: "@pat_rolls is disabled or set to be deleted"},
		{name: "deletion asked for", account: "pat_rolls", edit: func(_ *fakeAccounts, u *user.User) { u.DeletionRequested = true },
			want: "@pat_rolls is disabled or set to be deleted"},
		{name: "empty", account: "pat_rolls", secrets: []string{""}, want: "the password is empty", asked: 1},
		{name: "short", account: "pat_rolls", secrets: []string{"Ab1!"}, want: "at least 8 characters", asked: 1},
		{name: "no symbol", account: "pat_rolls", secrets: []string{"Freshstart9"}, want: "a capital letter, a number and a symbol", asked: 1},
		{name: "too many characters", account: "pat_rolls", secrets: []string{strings.Repeat("a", 70) + "A1!"}, want: "too long", asked: 1},
		{name: "too many bytes", account: "pat_rolls", secrets: []string{tooManyBytes, tooManyBytes}, want: "too long", asked: 1},
		{name: "mismatch", account: "pat_rolls", secrets: []string{goodPassword, goodPassword + "x"}, want: "the two passwords don't match", asked: 2},
		{name: "disabled meanwhile", account: "pat_rolls", secrets: []string{goodPassword, goodPassword},
			edit: func(f *fakeAccounts, _ *user.User) { f.setErr = user.ErrAccountDisabled },
			want: "@pat_rolls is disabled or set to be deleted", asked: 2},
		{name: "gone meanwhile", account: "pat_rolls", secrets: []string{goodPassword, goodPassword},
			edit: func(f *fakeAccounts, _ *user.User) { f.setErr = user.ErrNotFound },
			want: "@pat_rolls is gone", asked: 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store, u := newStore()
			if c.edit != nil {
				c.edit(store, u)
			}
			ask := &script{secrets: c.secrets}
			err := run(context.Background(), store, ask, io.Discard, c.account)
			if err == nil || !strings.Contains(err.Error(), c.want) || !strings.HasSuffix(err.Error(), "Nothing changed") {
				t.Fatalf("got %v, want %q ending in Nothing changed", err, c.want)
			}
			if len(store.set) != 0 {
				t.Fatal("a password was stored")
			}
			if len(ask.asked) != c.asked {
				t.Fatalf("asked %q, want %d prompts", ask.asked, c.asked)
			}
		})
	}
}
