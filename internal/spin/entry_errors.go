package spin

import (
	"errors"
	"fmt"
)

// Entry errors the service returns. Their text is fixed and plain; the HTTP
// layer maps each to a machine-readable code and never echoes input. The
// closed and settled cases reuse ErrClosed and ErrDaySettled from errors.go.
var (
	// ErrNoSpin: the date has no spin, so there is nothing to post or skip (a
	// future date, or a day the person never spun).
	ErrNoSpin = errors.New("spin: no spin on that date")
	// ErrSpinChanged: the spin number the phone saw is no longer the date's
	// latest spin (a bonus spin was made since).
	ErrSpinChanged = errors.New("spin: that is not the latest spin of the day")
	// ErrEntryNotFound: no entry with that id belongs to this person.
	ErrEntryNotFound = errors.New("spin: no such entry")
)

// Reasons a FieldProblem carries. They are the words the HTTP layer's field
// errors use.
const (
	ProblemRequired = "required"
	ProblemInvalid  = "invalid"
	ProblemTooLong  = "too_long"
	ProblemTooMany  = "too_many"
)

// FieldProblem says which entry field failed and why. Field is the JSON name
// of the field (title, stars, reaction, comment or photos) and Reason is one
// of the Problem constants. The cleaning functions return it as a plain
// error; callers read it with errors.As.
type FieldProblem struct {
	Field  string
	Reason string
}

func (p *FieldProblem) Error() string {
	return fmt.Sprintf("spin: entry field %s is not valid (%s)", p.Field, p.Reason)
}

// problem returns a *FieldProblem as an error. It is never nil.
func problem(field, reason string) error {
	return &FieldProblem{Field: field, Reason: reason}
}
