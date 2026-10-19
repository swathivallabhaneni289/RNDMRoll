// Package httpapi: invite.go serves the invite link's page, GET /i/:username
// (developer, 2026-10-19: Find your friends "should work like send invite
// link"). It is public and reads nothing from the database: the username in
// the link is only checked for its shape and shown back, so the page never
// says whether an account exists. Its button opens the app at that person
// (scheme://u/{username}); people without the app read that it is coming to
// the App Store. Once the real web address and universal links are set up
// (launch list), the same link opens the app directly.
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

// RegisterInvite mounts GET /i/:username. scheme is the app's link scheme.
func RegisterInvite(engine *gin.Engine, scheme string) {
	if scheme == "" {
		scheme = "rndmroll"
	}
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
		hdr.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("Referrer-Policy", "no-referrer")
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusOK)
		if err := invitePage.Execute(c.Writer, data); err != nil {
			log.Printf("httpapi: invite page: %v", err)
		}
	})
}
