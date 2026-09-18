package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery returns Gin middleware that recovers a panic in any downstream
// handler, logs the panic value and stack trace server-side via logger, and
// responds with a fixed, opaque body. The stack (and the panic message
// itself) never reaches the caller: returning it would hand an attacker a
// map of the internals, and RespondError already establishes "server_error"
// as the opaque code for anything unmapped (T-01-SRV-01).
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic recovered",
					"panic", rec,
					"stack", string(debug.Stack()),
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			}
		}()
		c.Next()
	}
}
