package httpapi

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func inviteEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterInvite(engine, "rndmroll")
	return engine
}

func inviteGet(t *testing.T, engine *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestInvitePage(t *testing.T) {
	engine := inviteEngine()
	get := func(path string) string {
		t.Helper()
		rec := inviteGet(t, engine, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		return rec.Body.String()
	}
	page := get("/i/Bob_Brown")
	for _, want := range []string{
		"<title>Join @bob_brown on RNDMRoll</title>",
		"@bob_brown invited you.",
		`<a class="button" href="rndmroll://u/bob_brown">Open in RNDMRoll</a>`,
		"one spin. one real moment.",
		"Coming soon to the App Store.",
		"<span>RNDMRoll</span>", // the header beside the logo
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("invite page has no %q: %s", want, page)
		}
	}
	// Anything that is not a username shape is never shown or linked: the
	// page says "You're invited." and has no button.
	for _, path := range []string{"/i/%3Cscript%3E", "/i/!!", "/i/a", "/i/" + strings.Repeat("a", 21)} {
		bad := get(path)
		if strings.Contains(bad, "<script") || strings.Contains(bad, "rndmroll://") || strings.Contains(bad, "Open in RNDMRoll") ||
			!strings.Contains(bad, "You're invited.") || !strings.Contains(bad, "<title>RNDMRoll</title>") ||
			!strings.Contains(bad, "Coming soon to the App Store.") || !strings.Contains(bad, "<span>RNDMRoll</span>") {
			t.Fatalf("%s: %s", path, bad)
		}
	}
	// "assets" is a username shape too, and the files under /i/assets/ do
	// not take its invite away.
	if page := get("/i/assets"); !strings.Contains(page, "@assets invited you.") || !strings.Contains(page, `href="rndmroll://u/assets"`) {
		t.Fatalf("@assets: %s", page)
	}
}

// The route only ever shows a checked username, but the template must still
// escape whatever it is given, in the title and in the heading.
func TestInvitePageEscapesTheUsername(t *testing.T) {
	var out bytes.Buffer
	err := invitePage.Execute(&out, struct {
		Username string
		OpenURL  template.URL
	}{Username: `<b>"&'`, OpenURL: "rndmroll://u/x"})
	if err != nil {
		t.Fatal(err)
	}
	page := out.String()
	escaped := "&lt;b&gt;&#34;&amp;&#39;"
	for _, want := range []string{
		"<title>Join @" + escaped + " on RNDMRoll</title>",
		"<h1>@" + escaped + " invited you.</h1>",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("page has no %q: %s", want, page)
		}
	}
	if strings.Contains(page, "<b>") {
		t.Fatalf("the username is not escaped: %s", page)
	}
}

func TestInvitePagePolicy(t *testing.T) {
	rec := inviteGet(t, inviteEngine(), "/i/bob_brown")
	want := "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	if got := rec.Header().Get("Content-Security-Policy"); got != want {
		t.Fatalf("policy %q, want %q", got, want)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("content type %q", got)
	}
}

// Every file the page names under /i/assets/ is served, as the right type,
// kept for a year, and is exactly the embedded file.
func TestInviteAssets(t *testing.T) {
	engine := inviteEngine()
	page := inviteGet(t, engine, "/i/bob_brown").Body.String()
	named := map[string]bool{}
	for _, m := range regexp.MustCompile(`/i/assets/([A-Za-z0-9_.-]+)`).FindAllStringSubmatch(page, -1) {
		named[m[1]] = true
	}
	for _, font := range []string{"Domine_600SemiBold.ttf", "WorkSans_400Regular.ttf", "WorkSans_600SemiBold.ttf"} {
		if !named[font] {
			t.Fatalf("the page does not load %s", font)
		}
	}
	for name := range named {
		if _, ok := inviteAssetTypes[name]; !ok {
			t.Fatalf("the page names /i/assets/%s, which is not served", name)
		}
	}

	for name, contentType := range inviteAssetTypes {
		rec := inviteGet(t, engine, "/i/assets/"+name)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", name, rec.Code)
		}
		h := rec.Header()
		if h.Get("Content-Type") != contentType || h.Get("Cache-Control") != "public, max-age=31536000, immutable" ||
			h.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s: headers %v", name, h)
		}
		want, err := inviteAssetFiles.ReadFile("invite_assets/" + name)
		if err != nil || !bytes.Equal(rec.Body.Bytes(), want) {
			t.Fatalf("%s: body is not the embedded file (%v)", name, err)
		}
		if contentType == "font/ttf" && !bytes.HasPrefix(want, []byte{0, 1, 0, 0}) {
			t.Fatalf("%s is not a TrueType font", name)
		}
		if strings.HasSuffix(name, ".txt") && !strings.Contains(string(want), "SIL OPEN FONT LICENSE Version 1.1") {
			t.Fatalf("%s is not the font licence", name)
		}
	}

	// Anything else under /i/assets/ is a 404, other files in the package included.
	for _, path := range []string{"/i/assets/nope.ttf", "/i/assets/invite_page.html", "/i/assets/invite.go",
		"/i/assets/..%2Finvite.go", "/i/assets/domine_600semibold.ttf", "/i/assets/x/y"} {
		if rec := inviteGet(t, engine, path); rec.Code != http.StatusNotFound {
			t.Fatalf("%s: %d, want 404", path, rec.Code)
		}
	}
}
