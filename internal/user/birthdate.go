package user

import (
	"errors"
	"time"
)

// MinimumAge is the youngest age that may hold an account. It is judged on
// the server clock, never on anything the client says about the date.
const MinimumAge = 13

// birthdateLayout is the only accepted wire format.
const birthdateLayout = "2006-01-02"

// earliestBirthYear matches the users_birthday_min check constraint.
const earliestBirthYear = 1900

// futureGrace is how far past the UTC clock a calendar date may be and still
// count as "today somewhere": UTC+14 is the furthest-ahead time zone, so a
// person there typing their own local date must not be told it is the future.
const futureGrace = 14 * time.Hour

// Fixed sentinels. Their text never contains the input, so an error can be
// returned or logged without leaking a birthday.
var (
	ErrBirthdateInvalid = errors.New("user: birthday is not a valid date")
	ErrBirthdateFuture  = errors.New("user: birthday is in the future")
)

// ValidateBirthdate parses s as a strict YYYY-MM-DD calendar date (so 31
// April and 29 February in a non-leap year are rejected), requires a year of
// 1900 or later, and rejects a date after the date of now plus 14 hours in
// UTC. It returns the date at midnight UTC. It does not judge the age; use
// AgeOn and MinimumAge for that.
func ValidateBirthdate(s string, now time.Time) (time.Time, error) {
	if len(s) != len(birthdateLayout) {
		return time.Time{}, ErrBirthdateInvalid
	}
	t, err := time.Parse(birthdateLayout, s)
	if err != nil || t.Format(birthdateLayout) != s {
		return time.Time{}, ErrBirthdateInvalid
	}
	if t.Year() < earliestBirthYear {
		return time.Time{}, ErrBirthdateInvalid
	}
	limit := now.UTC().Add(futureGrace)
	limitDate := time.Date(limit.Year(), limit.Month(), limit.Day(), 0, 0, 0, 0, time.UTC)
	if t.After(limitDate) {
		return time.Time{}, ErrBirthdateFuture
	}
	return t, nil
}

// AgeOn returns the age in whole years of someone born on birth, on the UTC
// calendar date of now. It compares the (year, month, day) tuples, not a day
// count, so a 29 February birthday turns a year older on 1 March in a
// non-leap year.
func AgeOn(birth, now time.Time) int {
	b := birth.UTC()
	n := now.UTC()
	age := n.Year() - b.Year()
	if n.Month() < b.Month() || (n.Month() == b.Month() && n.Day() < b.Day()) {
		age--
	}
	return age
}
