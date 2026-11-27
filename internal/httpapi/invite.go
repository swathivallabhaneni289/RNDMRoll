// Package httpapi: invite.go serves the invite link's page, GET /i/:username
// (developer, 2026-10-19: Find your friends "should work like send invite
// link"). It is public and reads nothing from the database: the username in
// the link is only checked for its shape and shown back, so the page never
// says whether an account exists. Its button opens the app at that person
// (scheme://u/{username}); people without the app read that it is coming to
// the App Store. Once the real web address and universal links are set up
// (launch list), the same link opens the app directly.
//
// The page looks like the app's Welcome page (developer, 2026-11-27): the
// Spin page's logo and name as the header, the Welcome wheel that spins once
// and lands, and the Welcome background moving behind it, all in CSS with no
// script. Its fonts are the app's own Domine and Work Sans, served from
// GET /i/assets/:file.
package httpapi

import (
	"embed"
	"html/template"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

//go:embed invite_page.html
var invitePageFile embed.FS

var invitePage = template.Must(template.ParseFS(invitePageFile, "invite_page.html"))

//go:embed invite_assets
var inviteAssetFiles embed.FS

// inviteAssetTypes lists every file GET /i/assets/:file serves, with its
// type. Anything else is a 404. The fonts are the app's own files from
// @expo-google-fonts, unchanged, with their licences.
var inviteAssetTypes = map[string]string{
	"Domine_600SemiBold.ttf":   "font/ttf",
	"WorkSans_400Regular.ttf":  "font/ttf",
	"WorkSans_600SemiBold.ttf": "font/ttf",
	"OFL-Domine.txt":           "text/plain; charset=utf-8",
	"OFL-WorkSans.txt":         "text/plain; charset=utf-8",
}

type inviteAsset struct {
	body        []byte
	contentType string
}

// inviteAssets reads every listed file once, at start-up, so a missing one
// stops the server instead of failing a visitor.
var inviteAssets = func() map[string]inviteAsset {
	out := make(map[string]inviteAsset, len(inviteAssetTypes))
	for name, contentType := range inviteAssetTypes {
		body, err := inviteAssetFiles.ReadFile("invite_assets/" + name)
		if err != nil {
			panic("httpapi: invite asset " + name + ": " + err.Error())
		}
		out[name] = inviteAsset{body: body, contentType: contentType}
	}
	return out
}()

// invitePolicy is the page's Content-Security-Policy: no script at all,
// inline styles, and fonts from this server only.
const invitePolicy = "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// RegisterInvite mounts GET /i/:username and its files at GET /i/assets/:file.
// scheme is the app's link scheme.
func RegisterInvite(engine *gin.Engine, scheme string) {
	if scheme == "" {
		scheme = "rndmroll"
	}
	engine.GET("/i/assets/:file", func(c *gin.Context) {
		asset, ok := inviteAssets[c.Param("file")]
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		hdr := c.Writer.Header()
		// The file names never change for new contents, so a year is safe.
		hdr.Set("Cache-Control", "public, max-age=31536000, immutable")
		hdr.Set("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, asset.contentType, asset.body)
	})
	engine.GET("/i/:username", func(c *gin.Context) {
		name, problem := social.CleanUsername(c.Param("username"))
		data := struct {
			Username string
			OpenURL  template.URL
		}{}
		if problem == "" {
			data.Username = name
			// The username is checked against the username pattern, so it
			// is safe inside the link.
			data.OpenURL = template.URL(scheme + "://u/" + name)
		}
		hdr := c.Writer.Header()
		hdr.Set("Content-Security-Policy", invitePolicy)
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("Referrer-Policy", "no-referrer")
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusOK)
		if err := invitePage.Execute(c.Writer, data); err != nil {
			log.Printf("httpapi: invite page: %v", err)
		}
	})
}
