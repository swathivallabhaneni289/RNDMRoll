package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxBodyBytes is the largest request body any /v1 route accepts. Every
// request body here is a small JSON object, so the cap is generous and
// exists only so a caller with no session cannot make the server buffer a
// huge one.
const MaxBodyBytes int64 = 16 << 10

// BodyLimit returns Gin middleware that caps the request body at max bytes.
// A declared Content-Length over the cap is refused at once with 413 and
// {"error":"payload_too_large"}. Otherwise the body is wrapped in
// http.MaxBytesReader, so a chunked or understated body fails when it is
// read; the handler's bind error then maps to the same 413 (see
// httpapi.RespondValidationError).
//
// Gin applies Use only to routes added AFTER the call, so it must be the
// first thing registered on the group.
func BodyLimit(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > max {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "payload_too_large"})
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		}
		c.Next()
	}
}
