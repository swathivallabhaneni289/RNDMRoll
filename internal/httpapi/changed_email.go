// Package httpapi: changed_email.go is the email that tells an account's
// owner their password changed. Forgot password's reset and Settings' change
// password both send it, after their answer.
package httpapi

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/mail"
)

// emailWorkDeadline bounds the work a handler runs after its answer
// (emailer.background): Forgot password's lookup, store and code email, the
// password-changed email, and sign-up's code emails.
const emailWorkDeadline = 15 * time.Second

// changedMessage is the email sent once a password changed (developer,
// 2026-11-23), worded as other services word it: what changed, nothing to do
// if it was you, and what to do if it was not. loggedOut is where the change
// logged the person out: "every phone" after a reset, which ends every
// session, and "every other phone" after a change in Settings, which keeps
// the phone it was made on. Plain words, no em dashes.
func changedMessage(loggedOut string) (subject, html, text string) {
	subject = "Your RNDMRoll password was changed"
	changed := "Your RNDMRoll password was just changed, and you were logged out on " + loggedOut + "."
	you := "If this was you, you can ignore this email."
	notYou := "If it wasn't you, reset your password right away from the app's Log in page."
	text = fmt.Sprintf("%s\n\n%s\n\n%s", changed, you, notYou)
	html = fmt.Sprintf("<p>%s</p><p>%s</p><p>%s</p>", changed, you, notYou)
	return subject, html, text
}

// emailer sends a handler's emails after its answer, each on its own
// deadline, never the request's: the answer never waits for an email, and a
// client that gives up never stops one.
type emailer struct {
	sender mail.Sender
	wg     sync.WaitGroup
}

// Wait waits for every email started so far (tests and shutdown).
func (e *emailer) Wait() { e.wg.Wait() }

// background runs work (an email) after the answer, on its own deadline,
// never the request's.
func (e *emailer) background(work func(ctx context.Context)) {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("httpapi: email panic: %v", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), emailWorkDeadline)
		defer cancel()
		work(ctx)
	}()
}

// sendChanged emails the owner of address that their password was just
// changed and they were logged out on loggedOut (changedMessage). A failed
// email changes nothing: the password is set either way.
func (e *emailer) sendChanged(ctx context.Context, address, loggedOut string) {
	subject, html, text := changedMessage(loggedOut)
	if err := e.sender.Send(ctx, address, subject, html, text); err != nil {
		log.Printf("httpapi: password changed email: %v", err)
	}
}
