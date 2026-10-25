package user

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DeletionGracePeriod is how long an account waits, once its owner asked for
// it to be deleted, before it is deleted for good: 30 days, like Instagram.
// Inside it, logging in or a Forgot password reset keeps the account; after
// it, the purge deletes the account and nothing brings it back.
//
// This is the only place the length is written. Every rule about it is SQL
// built from this constant (internal/store/postgres/account_deletion.go) and
// compares with the database's clock, never a Go clock: the spin service's
// clock moves per account in test mode, and two clocks could disagree about
// which side of the line an account is on.
const DeletionGracePeriod = 30 * 24 * time.Hour

// DeletionRequest is what asking for a deletion answers.
type DeletionRequest struct {
	// DeleteAfter is when the grace period ends: the moment of the ask plus
	// DeletionGracePeriod, on the database's clock. From then on the account
	// cannot be kept, and the purge deletes it at its next run.
	DeleteAfter time.Time
	// Zone is the person's stored time zone name, empty when they have none.
	Zone string
}

// AccountDeleter asks for an account to be deleted and cancels the ask. Like
// PasswordChanger it is deliberately not part of Repository: only the
// Postgres repository implements it, so no other Repository double has to
// grow a method.
type AccountDeleter interface {
	// RequestDeletion marks the account to be deleted after
	// DeletionGracePeriod and, in the same transaction, ends every session it
	// has, deletes its push tokens and deletes its Forgot password code. It
	// returns ErrNotFound when the account is gone, and ErrAccountDisabled
	// when an admin disabled it or its deletion was already asked for;
	// neither changes anything.
	RequestDeletion(ctx context.Context, userID uuid.UUID) (DeletionRequest, error)
	// CancelDeletion clears the ask while the grace period lasts and the
	// account is not disabled. restored reports whether this call cleared it:
	// false when there was no ask to clear, the grace period is over, or an
	// admin disabled the account.
	CancelDeletion(ctx context.Context, userID uuid.UUID) (restored bool, err error)
}
