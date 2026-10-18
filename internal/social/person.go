package social

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ReportReasons are the reasons a person can be reported for.
var ReportReasons = []string{"spam", "nudity", "harassment", "hate", "violence", "impersonation", "other"}

// ErrBadReason is a report reason that is not one of ReportReasons.
var ErrBadReason = errors.New("social: bad report reason")

// Profile is the top of another person's page: who they are and the
// caller's relation to them.
type Profile struct {
	Person     Person
	Relation   Relation
	FollowsYou bool
	// Muted: the caller muted them (their posts are off the caller's Board).
	Muted bool
	// Bio is shown to anyone who can open the page, like Instagram.
	Bio string
}

// Profile reads another person's page head. A miss, yourself and a block
// either way are all ErrNotFound (the app opens your own Profile tab for
// you).
func (s *Service) Profile(ctx context.Context, viewer, id uuid.UUID) (Profile, error) {
	if viewer == id {
		return Profile{}, ErrNotFound
	}
	p, err := s.Person(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	blocked, err := s.repo.HasBlock(ctx, viewer, id)
	if err != nil {
		return Profile{}, err
	}
	if blocked {
		return Profile{}, ErrNotFound
	}
	rel, followsYou, err := s.repo.RelationOf(ctx, viewer, id)
	if err != nil {
		return Profile{}, err
	}
	muted, err := s.repo.IsMuted(ctx, viewer, id)
	if err != nil {
		return Profile{}, err
	}
	bio, err := s.repo.Bio(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	return Profile{Person: p, Relation: rel, FollowsYou: followsYou, Muted: muted, Bio: bio}, nil
}

// reachable is nil when viewer may act on id: someone else, who exists, with
// no block either way. Anything else is ErrNotFound.
func (s *Service) reachable(ctx context.Context, viewer, id uuid.UUID) error {
	if viewer == id {
		return ErrNotFound
	}
	if _, err := s.Person(ctx, id); err != nil {
		return err
	}
	blocked, err := s.repo.HasBlock(ctx, viewer, id)
	if err != nil {
		return err
	}
	if blocked {
		return ErrNotFound
	}
	return nil
}

// Mute takes id's posts off the viewer's Board. Silent; muting twice is fine.
func (s *Service) Mute(ctx context.Context, viewer, id uuid.UUID) error {
	if err := s.reachable(ctx, viewer, id); err != nil {
		return err
	}
	return s.repo.Mute(ctx, viewer, id, s.now(viewer))
}

// Unmute puts id's posts back on the viewer's Board. Silent.
func (s *Service) Unmute(ctx context.Context, viewer, id uuid.UUID) error {
	if viewer == id {
		return ErrNotFound
	}
	return s.repo.Unmute(ctx, viewer, id)
}

// Report keeps the viewer's report of id. The person is never told.
func (s *Service) Report(ctx context.Context, viewer, id uuid.UUID, reason string) error {
	ok := false
	for _, r := range ReportReasons {
		if r == reason {
			ok = true
		}
	}
	if !ok {
		return ErrBadReason
	}
	if err := s.reachable(ctx, viewer, id); err != nil {
		return err
	}
	return s.repo.Report(ctx, viewer, id, reason, s.now(viewer))
}

// PeopleOf is another person's Followers (followers true) or Following list.
// Like a private Instagram account, only an accepted follower sees it; for
// anyone else, and yourself (your own lists are GET /follows), it is
// ErrNotFound.
func (s *Service) PeopleOf(ctx context.Context, viewer, owner uuid.UUID, followers bool) ([]Person, error) {
	if err := s.reachable(ctx, viewer, owner); err != nil {
		return nil, err
	}
	rel, _, err := s.repo.RelationOf(ctx, viewer, owner)
	if err != nil {
		return nil, err
	}
	if rel != RelationFollowing {
		return nil, ErrNotFound
	}
	return s.repo.PeopleOf(ctx, viewer, owner, followers)
}
