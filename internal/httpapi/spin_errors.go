// Package httpapi: spin_errors.go holds the error codes of the daily spin
// routes and the one function that maps the spin service's errors to them.
// errors.go is not edited: these are additions, and the phone adds the same
// seven names to its ApiErrorCode.
package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/middleware"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

const (
	// CodeSpinNotOpen: the spin is not open yet (before 8:00 PM, a date that
	// has not come, or under 12 hours after the last daily spin). 409.
	CodeSpinNotOpen ErrorCode = "spin_not_open"
	// CodeSpinClosed: the day the phone asked for has closed. 409.
	CodeSpinClosed ErrorCode = "spin_closed"
	// CodeNoBonus: no bonus spin is held, or the day already used its one. 409.
	CodeNoBonus ErrorCode = "no_bonus"
	// CodeDaySettled: the day is already posted or skipped. 409.
	CodeDaySettled ErrorCode = "day_settled"
	// CodeWheelStale: the wheel changed since the phone read it. 409.
	CodeWheelStale ErrorCode = "wheel_stale"
	// CodeWheelInvalid: the wheel breaks a rule; the body names the reason and
	// the row. 400.
	CodeWheelInvalid ErrorCode = "wheel_invalid"
	// CodeProfileIncomplete: the account has no name or username yet. 403.
	CodeProfileIncomplete ErrorCode = "profile_incomplete"
)

// respondSpinError maps the spin service's errors to a response. Anything it
// does not recognise goes to RespondError, which logs it and answers a fixed
// 500, so internal text never reaches the client.
func respondSpinError(c *gin.Context, err error) {
	var wheelErr *spin.WheelError
	var inputErr *spin.InputError
	switch {
	case errors.Is(err, spin.ErrNotOpen):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeSpinNotOpen)})
	case errors.Is(err, spin.ErrClosed):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeSpinClosed)})
	case errors.Is(err, spin.ErrNoBonus), errors.Is(err, spin.ErrNoOther):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeNoBonus)})
	case errors.Is(err, spin.ErrDaySettled):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeDaySettled)})
	case errors.Is(err, spin.ErrStale):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeWheelStale)})
	case errors.As(err, &wheelErr):
		Respond(c, http.StatusBadRequest, gin.H{
			"error":  string(CodeWheelInvalid),
			"reason": wheelErr.Reason,
			"index":  wheelErr.Index,
		})
	case errors.Is(err, spin.ErrZoneMissing):
		RespondFieldError(c, "time_zone", reasonRequired)
	case errors.Is(err, spin.ErrZoneInvalid):
		RespondFieldError(c, "time_zone", reasonInvalid)
	case errors.As(err, &inputErr):
		RespondFieldError(c, inputErr.Field, inputErr.Reason)
	case errors.Is(err, spin.ErrBusy):
		// Another request held this person's lock for too long.
		Respond(c, http.StatusServiceUnavailable, gin.H{"error": string(CodeServerError)})
	default:
		RespondError(c, err)
	}
}

// finishedUser returns the signed-in account the way every spin route needs
// it: the row RequireUser loaded, with a name and a username. A missing row
// is 401 token_invalid (RequireUser normally answers first) and an
// unfinished account is 403 profile_incomplete. The person's id comes only
// from the token, never from the request.
func finishedUser(c *gin.Context) (*user.User, bool) {
	u, err := middleware.UserFromContext(c)
	if err != nil {
		RespondError(c, user.ErrTokenInvalid)
		return nil, false
	}
	if !u.OnboardingComplete() {
		Respond(c, http.StatusForbidden, gin.H{"error": string(CodeProfileIncomplete)})
		return nil, false
	}
	return u, true
}
