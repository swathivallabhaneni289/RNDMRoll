// Package social holds one-way follows (a public account is followed at once,
// a private one gets a request), blocks, the notification settings and the
// ONE visibility rule every Phase 5 read goes through. A person is only ever
// described by id, name, username and photo link.
package social

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is the one answer for a person or request the caller may not
// reach: a miss, yourself, a block either way, someone else's request.
var ErrNotFound = errors.New("social: not found")

// Person is how another account is shown: nothing else ever leaves.
type Person struct {
	ID       uuid.UUID
	Name     string
	Username string
	PhotoURL *string
}

// Relation is the caller toward another person.
type Relation string

const (
	RelationNone      Relation = "none"
	RelationRequested Relation = "requested"
	RelationFollowing Relation = "following"
)

// Follow statuses and how an accepted row became accepted (see migration
// 0008): direct (a public account, at once), request (the owner accepted)
// and silent (the owner switched to public).
const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"

	ViaDirect  = "direct"
	ViaRequest = "request"
	ViaSilent  = "silent"
)

// LookupResult is the answer to a username lookup.
type LookupResult struct {
	Person     Person
	Relation   Relation
	FollowsYou bool
	Private    bool
}

// SearchHit is one person a search found, with the caller's relation.
type SearchHit struct {
	Person     Person
	Relation   Relation
	FollowsYou bool
}

// MaxSearchResults is the most people one search answers with.
const MaxSearchResults = 20

// CleanSearch reads a search value: trimmed, one leading @ removed, lower
// cased, at most 30 characters. "" means there is nothing to search for.
func CleanSearch(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "@")
	s = strings.ToLower(s)
	if r := []rune(s); len(r) > 30 {
		s = string(r[:30])
	}
	return s
}

// Request is a pending follow request: its id, the other person and when it
// was made.
type Request struct {
	ID     uuid.UUID
	Person Person
	At     time.Time
}

// Lists are the caller's People page lists.
type Lists struct {
	Followers []Person
	Following []Person
	Incoming  []Request
	Outgoing  []Request
}

// Blocked is one person the caller blocked.
type Blocked struct {
	Person Person
	At     time.Time
}

// Settings are the three notification switches; all true by default.
type Settings struct {
	Reactions  bool
	Comments   bool
	Milestones bool
}

// DefaultSettings is what a person has before they change anything.
var DefaultSettings = Settings{Reactions: true, Comments: true, Milestones: true}

// SettingsPatch changes only the switches that are not nil.
type SettingsPatch struct {
	Reactions  *bool
	Comments   *bool
	Milestones *bool
}

// Apply returns s with the patch's switches applied.
func (p SettingsPatch) Apply(s Settings) Settings {
	if p.Reactions != nil {
		s.Reactions = *p.Reactions
	}
	if p.Comments != nil {
		s.Comments = *p.Comments
	}
	if p.Milestones != nil {
		s.Milestones = *p.Milestones
	}
	return s
}

// usernamePattern is the username shape the sign-up page and the database
// check (users_username_charset, migration 0001) already enforce.
var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)

// CleanUsername reads a lookup value: trimmed, one leading @ removed, lower
// cased. The reason is "required" for an empty value and "invalid" for one
// that cannot be a username; "" when the value is good.
func CleanUsername(raw string) (string, string) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "@")
	s = strings.ToLower(s)
	if s == "" {
		return "", "required"
	}
	if !usernamePattern.MatchString(s) {
		return "", "invalid"
	}
	return s, ""
}
