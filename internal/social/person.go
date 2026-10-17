package social

import (
	"context"

	"github.com/google/uuid"
)

// Profile is the top of another person's page: who they are and the
// caller's relation to them.
type Profile struct {
	Person     Person
	Relation   Relation
	FollowsYou bool
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
	return Profile{Person: p, Relation: rel, FollowsYou: followsYou}, nil
}
