package mail

import (
	"context"
	"fmt"
	"io"
)

// logSender writes each outgoing message to an io.Writer instead of
// delivering it over the network. This is the MAIL_DRIVER=log driver: a
// first-class local-development configuration, not a stand-in for
// unfinished work. The token flow, expiry, and single-use enforcement in
// Service are identical regardless of which Sender is plugged in; only the
// transport differs.
type logSender struct {
	out io.Writer
}

// NewLogSender returns a Sender that writes the recipient and the full
// message (including the verification link) to out and always succeeds
// with no network call.
func NewLogSender(out io.Writer) Sender {
	return &logSender{out: out}
}

func (s *logSender) Send(ctx context.Context, to, subject, html, text string) error {
	fmt.Fprintf(s.out, "mail: to=%s subject=%q\n%s\n", to, subject, text)
	return nil
}
