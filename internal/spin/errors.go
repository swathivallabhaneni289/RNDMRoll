package spin

import (
	"errors"
	"fmt"
)

// Sentinel errors the service returns. Their text is fixed and plain; the
// HTTP layer maps each to a machine-readable code and never echoes input.
var (
	// ErrNotOpen: the spin is not open yet (before 8:00 PM, a date that has
	// not come, or less than 12 hours after the last daily spin).
	ErrNotOpen = errors.New("spin: not open yet")
	// ErrClosed: the day has closed.
	ErrClosed = errors.New("spin: that day has closed")
	// ErrNoBonus: no bonus spin is held, or one was already used that day.
	ErrNoBonus = errors.New("spin: no bonus spin available")
	// ErrDaySettled: the day is already posted or skipped.
	ErrDaySettled = errors.New("spin: that day is already settled")
	// ErrWheelInvalid is the target of errors.Is for a *WheelError.
	ErrWheelInvalid = errors.New("spin: the wheel is not valid")
	// ErrStale: the wheel was changed since the caller read it.
	ErrStale = errors.New("spin: the wheel changed since it was read")
	// ErrZoneMissing: no usable time zone is on file, so the phone must
	// report one again.
	ErrZoneMissing = errors.New("spin: no time zone on file")
	// ErrZoneInvalid: a time zone name the caller sent is not one we accept.
	ErrZoneInvalid = errors.New("spin: time zone name is not valid")
	// ErrNoOther: the wheel has no other category to pick.
	ErrNoOther = errors.New("spin: no other category on the wheel")
	// ErrBusy: another request held this person's lock too long (503).
	ErrBusy = errors.New("spin: busy, try again")
)

// Wheel problem reasons. The first seven come from ValidateWheel; the last
// two come from the service, which can see the database.
const (
	ReasonTooFew        = "too_few"
	ReasonTooMany       = "too_many"
	ReasonNameRequired  = "name_required"
	ReasonNameTooLong   = "name_too_long"
	ReasonDuplicateName = "duplicate_name"
	ReasonWeightRange   = "weight_range"
	ReasonDuplicateID   = "duplicate_id"
	ReasonUnknownID     = "unknown_id"
	ReasonTooManyTotal  = "too_many_total"
)

// WheelError says why a wheel is not valid and which row (by position in
// the sent list) is at fault. Index is -1 when the problem is the list as a
// whole. It matches ErrWheelInvalid with errors.Is.
type WheelError struct {
	Reason string
	Index  int
}

func (e *WheelError) Error() string {
	return fmt.Sprintf("spin: the wheel is not valid (%s)", e.Reason)
}

// Is makes errors.Is(err, ErrWheelInvalid) true for any *WheelError.
func (e *WheelError) Is(target error) bool { return target == ErrWheelInvalid }

// InputError names a request field that failed and the fixed reason, for
// problems found after the request was parsed (a date out of range, an
// after_seq that does not match). Field and Reason use the same names the
// HTTP layer's field errors use.
type InputError struct {
	Field  string
	Reason string
}

func (e *InputError) Error() string {
	return fmt.Sprintf("spin: %s is not valid (%s)", e.Field, e.Reason)
}
