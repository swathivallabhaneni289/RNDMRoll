// Command set-password sets a new password for an account from the
// terminal, like Django's changepassword (developer, 2026-10-29: a forgotten
// password should not need the phone and a code from the server log). Run it
// as `make password`, which reads DATABASE_URL from .env as `make dev` does.
//
// It asks for the password twice without showing it, so the password never
// lands on the screen, in the shell history or in a log, and checks it with
// the app's own rule (auth.PasswordReason). Setting it ends every app session
// the account had and deletes a Forgot password code asked for before
// (postgres.UserRepo.SetPassword). It runs only in a terminal, and every
// refusal changes nothing.
//
//	make password                          asks for the email or username
//	make password ACCOUNT=you@example.com  that account (an email or a username)
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"golang.org/x/term"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

func main() {
	account := flag.String("account", "", "the account's email or username (asked for when empty)")
	flag.Parse()
	if err := start(*account); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// start checks for a terminal before anything else, so a run without one (a
// pipe, a script, a chat window) neither reads the database nor asks for a
// password it could not hide.
func start(account string) error {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return errors.New("make password hides the password as you type it, so it needs a terminal. Open the Terminal app, go to the RNDMRoll folder and run make password there")
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is missing. make password reads it from .env, like the server")
	}
	state, err := term.GetState(fd)
	if err != nil {
		return err
	}
	stop := restoreOnInterrupt(fd, state)
	defer stop()

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("could not reach the database: %w", err)
	}
	defer pool.Close()
	ask := &terminal{fd: fd, in: bufio.NewReader(os.Stdin), out: os.Stdout}
	return run(ctx, postgres.NewUserRepo(pool), ask, os.Stdout, account)
}

// restoreOnInterrupt puts the terminal back the way it was if Ctrl+C stops
// the command while the typing is hidden; otherwise the shell would go on
// hiding it. The returned func stops watching.
func restoreOnInterrupt(fd int, state *term.State) (stop func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-signals:
			_ = term.Restore(fd, state)
			fmt.Fprintln(os.Stderr, "\nStopped.")
			os.Exit(130)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(signals)
		close(done)
	}
}

// terminal is the person at the keyboard: a line typed in the open (the
// account), and a secret typed without being shown (the password).
type terminal struct {
	fd  int
	in  *bufio.Reader
	out io.Writer
}

func (t *terminal) Line(prompt string) (string, error) {
	fmt.Fprint(t.out, prompt)
	line, err := t.in.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (t *terminal) Secret(prompt string) (string, error) {
	fmt.Fprint(t.out, prompt)
	secret, err := term.ReadPassword(t.fd)
	fmt.Fprintln(t.out) // the Enter that ended it is not shown either
	return string(secret), err
}

// accounts is what the command needs of the user store (postgres.UserRepo).
type accounts interface {
	GetByEmailCI(ctx context.Context, email string) (*user.User, error)
	GetByUsernameCI(ctx context.Context, username string) (*user.User, error)
	SetPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
}

// asker asks the person at the terminal (terminal; a script in the tests).
type asker interface {
	Line(prompt string) (string, error)
	Secret(prompt string) (string, error)
}

// ruleWords say what a password auth.PasswordReason refuses needs instead.
var ruleWords = map[string]string{
	"required":  "the password is empty",
	"too_short": "the password needs at least 8 characters",
	"too_long":  "the password is too long. Use 72 characters or fewer",
	"too_weak":  "the password needs a capital letter, a number and a symbol",
}

// unchanged ends every refusal, so it is plain the old password still works.
const unchanged = ". Nothing changed"

// run asks for the account unless it is given, then for the new password
// twice, and sets it.
func run(ctx context.Context, store accounts, ask asker, out io.Writer, account string) error {
	if account = strings.TrimSpace(account); account == "" {
		line, err := ask.Line("Email or username: ")
		if err != nil || line == "" {
			return errors.New("no account given" + unchanged)
		}
		account = line
	}
	u, err := find(ctx, store, account)
	if errors.Is(err, user.ErrNotFound) {
		return errors.New("no account has that email or username" + unchanged)
	}
	if err != nil {
		return fmt.Errorf("could not look up the account: %w", err)
	}
	name := displayName(u)
	if u.Disabled || u.DeletionRequested {
		return errors.New(name + " is disabled or set to be deleted, so its password can't change" + unchanged)
	}

	fmt.Fprintf(out, "A new password for %s.\n", name)
	password, err := ask.Secret("New password: ")
	if err != nil {
		return fmt.Errorf("could not read the password: %w", err)
	}
	if reason := auth.PasswordReason(password); reason != "" {
		return errors.New(ruleWords[reason] + unchanged)
	}
	if len(password) > auth.MaxPasswordBytes { // bcrypt's ceiling, in bytes
		return errors.New(ruleWords["too_long"] + unchanged)
	}
	again, err := ask.Secret("Type it again: ")
	if err != nil {
		return fmt.Errorf("could not read the password: %w", err)
	}
	if again != password {
		return errors.New("the two passwords don't match" + unchanged)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	switch err := store.SetPassword(ctx, u.ID, hash); {
	case errors.Is(err, user.ErrAccountDisabled):
		return errors.New(name + " is disabled or set to be deleted, so its password can't change" + unchanged)
	case errors.Is(err, user.ErrNotFound):
		return errors.New(name + " is gone" + unchanged)
	case err != nil:
		return fmt.Errorf("could not save the password: %w", err)
	}
	fmt.Fprintf(out, "Done. %s has a new password. Any phone logged in to it is logged out within 15 minutes.\n", name)
	return nil
}

// find looks the account up much as log in does: with an @ after the first
// character it is an email, otherwise a username (a leading @ is fine).
func find(ctx context.Context, store accounts, account string) (*user.User, error) {
	if strings.Contains(strings.TrimPrefix(account, "@"), "@") {
		return store.GetByEmailCI(ctx, account)
	}
	name, reason := social.CleanUsername(account)
	if reason != "" {
		return nil, user.ErrNotFound
	}
	return store.GetByUsernameCI(ctx, name)
}

// displayName names the account as the app shows it: @username, or the email
// of an account that has no username yet.
func displayName(u *user.User) string {
	if u.Username != nil && *u.Username != "" {
		return "@" + *u.Username
	}
	return u.Email
}
