package social

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

// Repository is the storage of follows, blocks and settings. Every pair
// write runs in one transaction under a lock on the pair, so a follow, an
// accept and a block between the same two people never interleave.
type Repository interface {
	// FindByUsername finds a finished account by exact username (already
	// cleaned, lower case). ErrNotFound for a miss.
	FindByUsername(ctx context.Context, username string) (Person, bool, error)
	// Search finds finished accounts whose username or name starts with
	// prefix (already cleaned, lower case), the viewer and blocks either way
	// left out, at most limit, the closest first.
	Search(ctx context.Context, viewer uuid.UUID, prefix string, limit int) ([]SearchHit, error)
	// RelationOf is viewer toward other, and whether other follows viewer
	// (accepted).
	RelationOf(ctx context.Context, viewer, other uuid.UUID) (Relation, bool, error)
	// People reads persons by id (missing ids are left out).
	People(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Person, error)
	// HasBlock reports a block either way.
	HasBlock(ctx context.Context, a, b uuid.UUID) (bool, error)

	// Follow makes or keeps follower's row toward followee: accepted at once
	// when followee is public, else pending. ErrNotFound for an unknown or
	// unfinished account, yourself, or a block either way.
	Follow(ctx context.Context, follower, followee uuid.UUID, at time.Time) (Relation, error)
	// Unfollow deletes follower's row toward followee (a follow or a
	// request). Absent is fine.
	Unfollow(ctx context.Context, follower, followee uuid.UUID) error
	// RemoveFollower deletes follower's row toward owner. Absent is fine.
	RemoveFollower(ctx context.Context, owner, follower uuid.UUID) error
	// Accept turns a pending request to owner into a follow (via request).
	Accept(ctx context.Context, owner, requestID uuid.UUID, at time.Time) error
	// Decline deletes a pending request to owner.
	Decline(ctx context.Context, owner, requestID uuid.UUID) error
	// AcceptAllPending accepts every pending request to owner (via silent).
	AcceptAllPending(ctx context.Context, owner uuid.UUID, at time.Time) error
	// Lists returns the People page lists, blocks left out.
	Lists(ctx context.Context, viewer uuid.UUID) (Lists, error)
	// FollowerCount counts accepted followers.
	FollowerCount(ctx context.Context, owner uuid.UUID) (int, error)
	// FollowingCount counts accepted follows the viewer has.
	FollowingCount(ctx context.Context, viewer uuid.UUID) (int, error)

	// Block records blocker's block of blocked and deletes the follows and
	// requests between the two. ErrNotFound for an unknown account or
	// yourself. A repeat is fine.
	Block(ctx context.Context, blocker, blocked uuid.UUID, at time.Time) error
	// Unblock deletes the block. Absent is fine.
	Unblock(ctx context.Context, blocker, blocked uuid.UUID) error
	// Blocks lists blocker's blocks, newest first.
	Blocks(ctx context.Context, blocker uuid.UUID) ([]Blocked, error)

	// Settings reads the switches (DefaultSettings when no row).
	Settings(ctx context.Context, userID uuid.UUID) (Settings, error)
	// SaveSettings stores the switches.
	SaveSettings(ctx context.Context, userID uuid.UUID, s Settings, at time.Time) error

	// ViewFacts reads Follows, Blocked and ViewerPostedDay for viewer and
	// owner on localDate (YYYY-MM-DD). IsOwner and Archived are the
	// caller's.
	ViewFacts(ctx context.Context, viewer, owner uuid.UUID, localDate string) (Facts, error)
	// Mute and Unmute hide or show muted's posts on muter's Board; both are
	// silent and repeatable. IsMuted reads it.
	Mute(ctx context.Context, muter, muted uuid.UUID, at time.Time) error
	Unmute(ctx context.Context, muter, muted uuid.UUID) error
	IsMuted(ctx context.Context, muter, muted uuid.UUID) (bool, error)
	// PeopleOf lists owner's accepted followers (followers true) or the
	// people owner follows (accepted), newest first, without anyone who has
	// a block either way with viewer.
	PeopleOf(ctx context.Context, viewer, owner uuid.UUID, followers bool) ([]Person, error)
	// Suggestions are people the viewer may know (Find your friends): people
	// that the people they follow follow, and people who follow them, minus
	// anyone they follow or asked, themselves and any block either way;
	// the most likely first, at most limit.
	Suggestions(ctx context.Context, viewer uuid.UUID, limit int) ([]Suggestion, error)
	// Bio reads a person's bio ("" when none).
	Bio(ctx context.Context, id uuid.UUID) (string, error)
	// Report keeps a report of reported by reporter (one per pair; a new
	// one replaces the reason).
	Report(ctx context.Context, reporter, reported uuid.UUID, reason string, at time.Time) error
}

// Service is the follows, blocks and settings use cases. Every write reads
// now once from the spin slice's clock, for the person acting.
type Service struct {
	repo Repository
	clk  clock.Clock
}

// NewService builds a Service.
func NewService(repo Repository, clk clock.Clock) *Service {
	return &Service{repo: repo, clk: clk}
}

func (s *Service) now(actor uuid.UUID) time.Time {
	return s.clk.NowFor(actor.String()).UTC()
}

// Lookup finds a person by exact username. A miss, yourself and a block
// either way are all ErrNotFound.
func (s *Service) Lookup(ctx context.Context, viewer uuid.UUID, username string) (LookupResult, error) {
	p, private, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		return LookupResult{}, err
	}
	if p.ID == viewer {
		return LookupResult{}, ErrNotFound
	}
	blocked, err := s.repo.HasBlock(ctx, viewer, p.ID)
	if err != nil {
		return LookupResult{}, err
	}
	if blocked {
		return LookupResult{}, ErrNotFound
	}
	rel, followsYou, err := s.repo.RelationOf(ctx, viewer, p.ID)
	if err != nil {
		return LookupResult{}, err
	}
	return LookupResult{Person: p, Relation: rel, FollowsYou: followsYou, Private: private}, nil
}

// Search finds people by the start of their username or name. An empty
// value answers no one.
func (s *Service) Search(ctx context.Context, viewer uuid.UUID, prefix string) ([]SearchHit, error) {
	if prefix == "" {
		return []SearchHit{}, nil
	}
	return s.repo.Search(ctx, viewer, prefix, MaxSearchResults)
}

// Person reads one person. ErrNotFound when missing.
func (s *Service) Person(ctx context.Context, id uuid.UUID) (Person, error) {
	m, err := s.repo.People(ctx, []uuid.UUID{id})
	if err != nil {
		return Person{}, err
	}
	p, ok := m[id]
	if !ok {
		return Person{}, ErrNotFound
	}
	return p, nil
}

// People reads persons by id; missing ids are left out.
func (s *Service) People(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Person, error) {
	return s.repo.People(ctx, ids)
}

// Follow sends target a follow request (every account acts as private). A
// repeat answers the same relation.
func (s *Service) Follow(ctx context.Context, viewer, target uuid.UUID) (Relation, error) {
	if viewer == target {
		return "", ErrNotFound
	}
	return s.repo.Follow(ctx, viewer, target, s.now(viewer))
}

// Unfollow ends a follow or cancels a request. Silent.
func (s *Service) Unfollow(ctx context.Context, viewer, target uuid.UUID) error {
	return s.repo.Unfollow(ctx, viewer, target)
}

// RemoveFollower removes follower from the viewer's followers. Silent.
func (s *Service) RemoveFollower(ctx context.Context, viewer, follower uuid.UUID) error {
	return s.repo.RemoveFollower(ctx, viewer, follower)
}

// Accept accepts a request addressed to the viewer.
func (s *Service) Accept(ctx context.Context, viewer, requestID uuid.UUID) error {
	return s.repo.Accept(ctx, viewer, requestID, s.now(viewer))
}

// Decline deletes a request addressed to the viewer. Silent.
func (s *Service) Decline(ctx context.Context, viewer, requestID uuid.UUID) error {
	return s.repo.Decline(ctx, viewer, requestID)
}

// BecamePublic accepts every pending request to the viewer, with no
// notification. The profile route calls it after a switch to public.
func (s *Service) BecamePublic(ctx context.Context, viewer uuid.UUID) error {
	return s.repo.AcceptAllPending(ctx, viewer, s.now(viewer))
}

// Lists returns the People page lists.
func (s *Service) Lists(ctx context.Context, viewer uuid.UUID) (Lists, error) {
	return s.repo.Lists(ctx, viewer)
}

// CountFollowers counts accepted followers (the Profile's Followers number).
func (s *Service) CountFollowers(ctx context.Context, userID string) (int, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return 0, err
	}
	return s.repo.FollowerCount(ctx, id)
}

// CountFollowing counts accepted follows the person has (the Profile's
// Following number).
func (s *Service) CountFollowing(ctx context.Context, userID string) (int, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return 0, err
	}
	return s.repo.FollowingCount(ctx, id)
}

// FollowsAnyone reports whether the viewer follows at least one person.
func (s *Service) FollowsAnyone(ctx context.Context, viewer uuid.UUID) (bool, error) {
	n, err := s.repo.FollowingCount(ctx, viewer)
	return n > 0, err
}

// Block blocks another person: the follows and requests both ways go, and
// each is hidden from the other. Unknown and yourself are ErrNotFound.
func (s *Service) Block(ctx context.Context, viewer, other uuid.UUID) error {
	if viewer == other {
		return ErrNotFound
	}
	return s.repo.Block(ctx, viewer, other, s.now(viewer))
}

// Unblock removes a block. Nothing comes back.
func (s *Service) Unblock(ctx context.Context, viewer, other uuid.UUID) error {
	return s.repo.Unblock(ctx, viewer, other)
}

// Blocks lists the viewer's blocks, newest first.
func (s *Service) Blocks(ctx context.Context, viewer uuid.UUID) ([]Blocked, error) {
	return s.repo.Blocks(ctx, viewer)
}

// Settings reads the viewer's notification switches.
func (s *Service) Settings(ctx context.Context, viewer uuid.UUID) (Settings, error) {
	return s.repo.Settings(ctx, viewer)
}

// UpdateSettings applies a patch and returns the switches as stored.
func (s *Service) UpdateSettings(ctx context.Context, viewer uuid.UUID, p SettingsPatch) (Settings, error) {
	cur, err := s.repo.Settings(ctx, viewer)
	if err != nil {
		return Settings{}, err
	}
	next := p.Apply(cur)
	if err := s.repo.SaveSettings(ctx, viewer, next, s.now(viewer)); err != nil {
		return Settings{}, err
	}
	return next, nil
}

// ViewFacts reads the facts for one entry of owner on localDate. isOwner
// and archived are filled by the caller from the entry.
func (s *Service) ViewFacts(ctx context.Context, viewer, owner uuid.UUID, localDate string, archived bool) (Facts, error) {
	if viewer == owner {
		return Facts{IsOwner: true, Archived: archived}, nil
	}
	f, err := s.repo.ViewFacts(ctx, viewer, owner, localDate)
	if err != nil {
		return Facts{}, err
	}
	f.Archived = archived
	return f, nil
}
