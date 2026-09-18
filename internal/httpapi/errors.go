// Package httpapi provides the HTTP-layer building blocks every handler
// plan shares: response helpers and the domain-error-to-HTTP-status
// mapping. The in-memory test harness lives in testsupport_test.go.
package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// ErrorCode identifies the machine-readable strings this package returns
// in an ApiErrorBody's "error" field. Every value below appears verbatim
// in the ApiErrorCode union in lib/api/types.ts, so client and server
// agree without a translation layer.
type ErrorCode string

const (
	CodeInvalidCredentials ErrorCode = "invalid_credentials"
	CodeEmailTaken         ErrorCode = "email_taken"
	CodeUsernameTaken      ErrorCode = "username_taken"
	CodeEmailNotVerified   ErrorCode = "email_not_verified"
	CodeNotFound           ErrorCode = "not_found"
	CodeTokenInvalid       ErrorCode = "token_invalid"
	CodeTokenExpired       ErrorCode = "token_expired"
	CodeTokenConsumed      ErrorCode = "token_consumed"
	CodeSubjectLinked      ErrorCode = "subject_linked"
	CodeRateLimited        ErrorCode = "rate_limited"
	CodeValidationFailed   ErrorCode = "validation_failed"
	CodeServerError        ErrorCode = "server_error"
)

// Respond writes body as JSON with status.
func Respond(c *gin.Context, status int, body any) {
	c.JSON(status, body)
}

// RespondError maps a domain error to an HTTP status and machine-readable
// code and writes it as the response body. Any error not recognized below
// maps to a fixed 500 server_error so internal error text never reaches
// the client; the underlying error is logged server-side instead.
func RespondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, user.ErrInvalidCredentials):
		Respond(c, http.StatusUnauthorized, gin.H{"error": string(CodeInvalidCredentials)})
	case errors.Is(err, user.ErrEmailTaken):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeEmailTaken)})
	case errors.Is(err, user.ErrUsernameTaken):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeUsernameTaken)})
	case errors.Is(err, user.ErrEmailNotVerified):
		Respond(c, http.StatusForbidden, gin.H{"error": string(CodeEmailNotVerified)})
	case errors.Is(err, user.ErrNotFound):
		Respond(c, http.StatusNotFound, gin.H{"error": string(CodeNotFound)})
	case errors.Is(err, user.ErrTokenInvalid):
		Respond(c, http.StatusUnauthorized, gin.H{"error": string(CodeTokenInvalid)})
	case errors.Is(err, user.ErrTokenExpired):
		Respond(c, http.StatusUnauthorized, gin.H{"error": string(CodeTokenExpired)})
	case errors.Is(err, user.ErrTokenConsumed):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeTokenConsumed)})
	case errors.Is(err, user.ErrSubjectLinkedToOtherAccount):
		Respond(c, http.StatusConflict, gin.H{"error": string(CodeSubjectLinked)})
	default:
		log.Printf("httpapi: unmapped error: %v", err)
		Respond(c, http.StatusInternalServerError, gin.H{"error": string(CodeServerError)})
	}
}

// RespondValidationError writes a 400 validation_failed response for a
// failed ShouldBindJSON call.
func RespondValidationError(c *gin.Context, err error) {
	Respond(c, http.StatusBadRequest, gin.H{"error": string(CodeValidationFailed), "message": err.Error()})
}
