package httpapi

import "testing"

// The password-changed email says where the change logged the person out: a
// reset ends every session, a change in Settings keeps the phone it was made
// on. Everything else is the same.
func TestChangedMessage_SaysWhereThePersonWasLoggedOut(t *testing.T) {
	const rest = "\n\nIf this was you, you can ignore this email.\n\n" +
		"If it wasn't you, reset your password right away from the app's Log in page."
	for _, c := range []struct{ loggedOut, first string }{
		{"every phone", "Your RNDMRoll password was just changed, and you were logged out on every phone."},
		{"every other phone", "Your RNDMRoll password was just changed, and you were logged out on every other phone."},
	} {
		subject, html, text := changedMessage(c.loggedOut)
		if subject != "Your RNDMRoll password was changed" {
			t.Errorf("%s: subject %q", c.loggedOut, subject)
		}
		if text != c.first+rest {
			t.Errorf("%s: text %q", c.loggedOut, text)
		}
		wantHTML := "<p>" + c.first + "</p><p>If this was you, you can ignore this email.</p>" +
			"<p>If it wasn't you, reset your password right away from the app's Log in page.</p>"
		if html != wantHTML {
			t.Errorf("%s: html %q", c.loggedOut, html)
		}
	}
}
