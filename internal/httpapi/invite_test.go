package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInvitePage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterInvite(engine, "rndmroll")
	get := func(path string) string {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		return rec.Body.String()
	}
	page := get("/i/Bob_Brown")
	if !strings.Contains(page, "@bob_brown invited you") || !strings.Contains(page, `href="rndmroll://u/bob_brown"`) {
		t.Fatalf("invite page: %s", page)
	}
	// Anything that is not a username shape is never shown or linked.
	bad := get("/i/%3Cscript%3E")
	if strings.Contains(bad, "<script") || strings.Contains(bad, "rndmroll://") || !strings.Contains(bad, "You're invited") {
		t.Fatalf("bad name: %s", bad)
	}
}
