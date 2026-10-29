// Command admin-link opens the admin page in the browser, logged in, without a
// password (developer, 2026-10-29: "make it easy to access the admin page").
// Run it as `make admin`, which reads JWT_SECRET from .env as `make dev` does.
//
// It signs a one-time link with JWT_SECRET (internal/adminlink), the secret
// the admin page's cookie is signed with, so it opens nothing that secret
// could not already open. The link works once, for two minutes, and is not
// printed unless asked (-print), so it stays out of the terminal.
//
//	make admin                          the first ADMIN_EMAILS account
//	make admin EMAIL=admin@example.com  that admin account
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/adminlink"
)

func main() {
	base := flag.String("url", "http://localhost:8080", "where the server answers")
	email := flag.String("email", "", "the admin to log in as (default: the first ADMIN_EMAILS account)")
	printOnly := flag.Bool("print", false, "print the link instead of opening it")
	flag.Parse()
	if err := run(strings.TrimRight(*base, "/"), strings.ToLower(strings.TrimSpace(*email)), *printOnly); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(base, email string, printOnly bool) error {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < adminlink.MinSecretBytes {
		return errors.New("JWT_SECRET is missing or too short. make admin reads it from .env, like the server")
	}
	if err := checkServer(base); err != nil {
		return err
	}
	token, err := adminlink.New([]byte(secret), email, time.Now())
	if err != nil {
		return err
	}
	link := base + "/admin/link?token=" + url.QueryEscape(token)
	if printOnly {
		fmt.Println(link)
		fmt.Fprintln(os.Stderr, "The link works once, for two minutes.")
		return nil
	}
	if err := exec.Command("open", link).Run(); err != nil {
		return fmt.Errorf("could not open the browser (%v). Run go run ./cmd/admin-link -print and open the link yourself", err)
	}
	fmt.Println("Opened the admin page in your browser.")
	return nil
}

// checkServer says plainly when the server is not running, or runs without
// the admin page (no ADMIN_EMAILS), instead of a browser error page.
func checkServer(base string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(base + "/admin/login")
	if err != nil {
		return fmt.Errorf("the server isn't answering at %s. Start it, then run make admin again", base)
	}
	res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return errors.New("the admin page is off. Start the server with ADMIN_EMAILS set to your account's email")
	}
	return nil
}
