// Package httpapi: entry_errors.go holds the error codes of the entry routes
// (the photo ticket, post, read, edit, delete and skip) and the one function
// that maps the entry service's errors to them. errors.go and spin_errors.go
// are not edited: these are additions, and the phone adds the same three names
// to its ApiErrorCode.
package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

const (
	// CodeNoSpin: the date has no spin, so there is nothing to post or skip (a
	// date that has not come, or a day the person never spun). 409.
	CodeNoSpin ErrorCode = "no_spin"
	// CodeDayClosed: the day's late window has ended, so it can no longer be
	// posted or skipped. 409. The spin routes say spin_closed for the same
	// condition; the entry routes use their own word.
	CodeDayClosed ErrorCode = "day_closed"
	// CodeSpinChanged: the spin number the phone saw is no longer the date's
	// latest spin (a bonus spin was made since). 409.
	CodeSpinChanged ErrorCode = "spin_changed"
)

// The fixed texts of a refused photo list. The shared reason texts in
// errors.go cannot say "photo", so this field has its own, in the same body
// shape as every other field error.
const (
	photosRequiredText = "Add at least one photo."
	photosTooManyText  = "You can add up to 10 photos."
	photosInvalidText  = "One of the photos can't be used."
)

// fieldPhotos is the JSON name of the photo list field.
const fieldPhotos = "photos"

// respondPhotosProblem writes the 400 validation_failed answer for the photo
// list, with the fixed text for the reason: required, too_many, and invalid
// for everything else.
func respondPhotosProblem(c *gin.Context, reason string) {
	text := photosInvalidText
	switch reason {
	case spin.ProblemRequired:
		text = photosRequiredText
	case spin.ProblemTooMany:
		text = photosTooManyText
	}
	Respond(c, http.StatusBadRequest, gin.H{
		"error":   string(CodeValidationFailed),
		"field":   fieldPhotos,
		"message": text,
	})
}

// respondEntryError maps the entry service's errors to a response. The entry
// errors are mapped first (a day that has closed is day_closed here, not the
// spin routes' spin_closed); everything else goes to respondSpinError, so a
// busy lock stays 503, a missing zone stays the time_zone field error, an
// InputError names its field, and anything unknown is a logged, fixed 500.
func respondEntryError(c *gin.Context, err error) {
	var problem *spin.FieldProblem
	switch {
	case errors.Is(err, spin.ErrNoSpin):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeNoSpin)})
	case errors.Is(err, spin.ErrClosed):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeDayClosed)})
	case errors.Is(err, spin.ErrDaySettled):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeDaySettled)})
	case errors.Is(err, spin.ErrSpinChanged):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeSpinChanged)})
	case errors.Is(err, spin.ErrEntryNotFound):
		Respond(c, http.StatusNotFound, gin.H{"error": string(CodeNotFound)})
	case errors.As(err, &problem):
		if problem.Field == fieldPhotos {
			respondPhotosProblem(c, problem.Reason)
			return
		}
		RespondFieldError(c, problem.Field, problem.Reason)
	default:
		respondSpinError(c, err)
	}
}
