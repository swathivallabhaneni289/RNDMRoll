package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/mail"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

// captureSender keeps every email instead of sending it.
type captureSender struct {
	mu   sync.Mutex
	sent []struct{ to, subject, text string }
}

func (s *captureSender) Send(_ context.Context, to, subject, _ string, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, struct{ to, subject, text string }{to, subject, text})
	return nil
}

func (s *captureSender) lastCode(t *testing.T, to string) string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.sent) - 1; i >= 0; i-- {
		if s.sent[i].to == to {
			return regexp.MustCompile(`\b\d{6}\b`).FindString(s.sent[i].text)
		}
	}
	t.Fatalf("no email to %s", to)
	return ""
}

// count is how many emails went out.
func (s *captureSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}

// resetWorld is the two Forgot password routes over the real database, on a
// clock the test moves.
type resetWorld struct {
	t      *testing.T
	pool   *pgxpool.Pool
	engine *gin.Engine // the whole server, for sign-up and log in
	routes *gin.Engine // the Forgot password routes
	h      *PasswordResetHandler
	now    time.Time
}

func newResetWorld(t *testing.T, sender mail.Sender) *resetWorld {
	t.Helper()
	pool := requireIntegrationPool(t)
	w := &resetWorld{t: t, pool: pool, now: time.Now()}
	w.engine = buildIntegrationServerWith(t, pool, integrationOptions{}).Engine()
	users := postgres.NewUserRepo(pool)
	w.h = NewPasswordResetHandler(users, users, auth.NewRefreshService(postgres.NewRefreshTokenRepo(pool), time.Hour),
		postgres.NewPasswordResetRepo(pool), sender, integrationJWTSecret, 15*time.Minute)
	w.h.now = func() time.Time { return w.now }
	// The email goes out after the answer: it finishes before the truncate
	// (cleanups run last first).
	t.Cleanup(w.h.Wait)
	w.routes = gin.New()
	w.h.Register(w.routes.Group("/v1"))
	return w
}

// call sends one request to the routes, waits for any email it started and
// checks the status.
func (w *resetWorld) call(path string, body any, want int) map[string]any {
	w.t.Helper()
	rec := integrationDo(w.t, w.routes, http.MethodPost, path, "", body)
	w.h.Wait()
	if rec.Code != want {
		w.t.Fatalf("%s: %d %s, want %d", path, rec.Code, rec.Body.String(), want)
	}
	return decodeBody(w.t, rec)
}

func (w *resetWorld) forgot(email string) {
	w.t.Helper()
	w.call("/v1/auth/password/forgot", map[string]any{"email": email}, http.StatusOK)
}

func (w *resetWorld) reset(email, code, password string, want int) map[string]any {
	w.t.Helper()
	return w.call("/v1/auth/password/reset", map[string]any{"email": email, "code": code, "new_password": password}, want)
}

// otherCode is a well-formed code that is not code.
func otherCode(code string) string {
	if code == "000000" {
		return "111111"
	}
	return "000000"
}

func TestPasswordResetIntegration(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email, old, fresh = "reset-me@example.com", "Correct-horse-battery-staple1", "Brand-new-password-2!"
	sess := integrationOnboardAccount(t, w.engine, email, old, "Reset Me", "reset_me")

	// An unknown email gets the same answer and no email.
	w.forgot("nobody@example.com")
	if n := box.count(); n != 0 {
		t.Fatalf("emailed a stranger: %d emails", n)
	}
	w.forgot("  RESET-me@Example.com ")
	code := box.lastCode(t, email)
	box.mu.Lock()
	subject := box.sent[0].subject
	box.mu.Unlock()
	if code == "" || !strings.Contains(subject, code) {
		t.Fatalf("code email: %q %q", subject, code)
	}

	// A weak password, a wrong code, an unknown email: refused.
	if got := w.reset(email, code, "weak", http.StatusBadRequest); got["field"] != "new_password" {
		t.Fatalf("weak: %v", got)
	}
	if got := w.reset(email, otherCode(code), fresh, http.StatusBadRequest); got["field"] != "code" {
		t.Fatalf("wrong code: %v", got)
	}
	w.reset("nobody@example.com", code, fresh, http.StatusBadRequest)

	// The right code: a new session, the new password works, the old one and
	// the old session do not, and the code is used up.
	got := w.reset(email, code, fresh, http.StatusOK)
	if got["access_token"] == "" || dig(t, got, "user", "username") != "reset_me" {
		t.Fatalf("session: %v", got)
	}
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": fresh}); rec.Code != http.StatusOK {
		t.Fatalf("new password: %d", rec.Code)
	}
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": old}); rec.Code == http.StatusOK {
		t.Fatal("old password still works")
	}
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": sess.RefreshToken}); rec.Code == http.StatusOK {
		t.Fatal("old session still works")
	}
	w.reset(email, code, "Another-password-3!", http.StatusBadRequest)

	// Five wrong tries use the code up, even for the right code after.
	w.forgot(email)
	code = box.lastCode(t, email)
	for i := 0; i < 5; i++ {
		w.reset(email, otherCode(code), "Another-password-3!", http.StatusBadRequest)
	}
	w.reset(email, code, "Another-password-3!", http.StatusBadRequest)
}

// A new code never resets the wrong tries: they carry over while a code is
// alive, a used-up code gets no new one until it expires, and only then does
// a new code start from zero.
func TestPasswordResetIntegration_NewCodeKeepsWrongTries(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email, fresh = "tries@example.com", "Brand-new-password-2!"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Tries", "tries_me")

	w.forgot(email)
	first := box.lastCode(t, email)
	for i := 0; i < 3; i++ {
		w.reset(email, otherCode(first), fresh, http.StatusBadRequest)
	}
	// Two minutes on, a new code keeps those 3: 2 more wrong tries use it up.
	w.now = w.now.Add(2 * time.Minute)
	w.forgot(email)
	if n := box.count(); n != 2 {
		t.Fatalf("emails %d, want 2", n)
	}
	second := box.lastCode(t, email)
	for i := 0; i < 2; i++ {
		w.reset(email, otherCode(second), fresh, http.StatusBadRequest)
	}
	w.reset(email, second, fresh, http.StatusBadRequest)

	// Used up: asking again sends nothing until the code's 30 minutes are up.
	w.now = w.now.Add(2 * time.Minute)
	w.forgot(email)
	if n := box.count(); n != 2 {
		t.Fatalf("a used-up code got a new one: %d emails", n)
	}
	w.now = w.now.Add(resetCodeTTL)
	w.forgot(email)
	third := box.lastCode(t, email)
	w.reset(email, otherCode(third), fresh, http.StatusBadRequest)
	w.reset(email, third, fresh, http.StatusOK)
}

// Asking again within a minute sends nothing and keeps the first code, so a
// quick second tap cannot spoil the email already on its way.
func TestPasswordResetIntegration_AskingTwiceWithinAMinuteSendsOneCode(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email = "twice@example.com"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Twice", "twice_me")

	w.forgot(email)
	code := box.lastCode(t, email)
	w.now = w.now.Add(30 * time.Second)
	w.forgot(email)
	if n := box.count(); n != 1 {
		t.Fatalf("emails %d, want 1", n)
	}
	w.reset(email, code, "Brand-new-password-2!", http.StatusOK)
}

// The right code sent several times at once changes the password once: only
// one request can spend the code, and it does so before the change.
func TestPasswordResetIntegration_RightCodeAtOnceWorksOnce(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email = "race-right@example.com"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Race Right", "race_right")
	w.forgot(email)
	code := box.lastCode(t, email)

	password := func(i int) string { return fmt.Sprintf("Brand-new-password-%d!", i) }
	reqs := make([]*http.Request, 5)
	for i := range reqs {
		reqs[i] = integrationRequest(t, http.MethodPost, "/v1/auth/password/reset", "",
			map[string]any{"email": email, "code": code, "new_password": password(i)})
	}
	var won []int
	for i, rec := range integrationAtOnce(w.routes, reqs) {
		if rec.Code == http.StatusOK {
			won = append(won, i)
		} else if rec.Code != http.StatusBadRequest || decodeBody(t, rec)["field"] != "code" {
			t.Fatalf("request %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if len(won) != 1 {
		t.Fatalf("one code changed the password %d times", len(won))
	}
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": password(won[0])}); rec.Code != http.StatusOK {
		t.Fatalf("the winner's password: %d", rec.Code)
	}
}

// Wrong guesses sent at once are checked up to the limit and no further:
// each try is counted and read in one step.
func TestPasswordResetIntegration_WrongGuessesAtOnceStopAtTheLimit(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email = "race-wrong@example.com"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Race Wrong", "race_wrong")
	w.forgot(email)
	code := box.lastCode(t, email)

	reqs := make([]*http.Request, 20)
	for i := range reqs {
		reqs[i] = integrationRequest(t, http.MethodPost, "/v1/auth/password/reset", "",
			map[string]any{"email": email, "code": otherCode(code), "new_password": "Brand-new-password-2!"})
	}
	for i, rec := range integrationAtOnce(w.routes, reqs) {
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("guess %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	var attempts int
	if err := w.pool.QueryRow(context.Background(), `
		select r.attempts from password_resets r join users u on u.id = r.user_id where u.email = $1`, email).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != resetMaxAttempts {
		t.Fatalf("20 wrong guesses at once were checked %d times, want %d", attempts, resetMaxAttempts)
	}
	w.reset(email, code, "Brand-new-password-2!", http.StatusBadRequest)
}

// heldSender holds every email until released, and keeps the context's
// error at the moment each one goes out.
type heldSender struct {
	release chan struct{}
	mu      sync.Mutex
	errs    []error
}

func (s *heldSender) Send(ctx context.Context, _, _, _, _ string) error {
	<-s.release
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs = append(s.errs, ctx.Err())
	return nil
}

// Forgot answers at once for an account just as for a stranger, before the
// email goes out, and the email does not ride on the request's context.
func TestPasswordResetIntegration_ForgotAnswersBeforeTheEmail(t *testing.T) {
	sender := &heldSender{release: make(chan struct{})}
	var once sync.Once
	release := func() { once.Do(func() { close(sender.release) }) }
	w := newResetWorld(t, sender)
	t.Cleanup(release) // runs before the world's wait
	integrationOnboardAccount(t, w.engine, "held@example.com", "Correct-horse-battery-staple1", "Held", "held_me")

	for _, email := range []string{"nobody@example.com", "held@example.com"} {
		ctx, cancel := context.WithCancel(context.Background())
		req := integrationRequest(t, http.MethodPost, "/v1/auth/password/forgot", "", map[string]any{"email": email}).WithContext(ctx)
		rec := httptest.NewRecorder()
		done := make(chan struct{})
		go func() {
			defer close(done)
			w.routes.ServeHTTP(rec, req)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			cancel()
			t.Fatalf("forgot for %s waited on the email", email)
		}
		// The real server ends the request's context once it has answered.
		cancel()
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "{}" {
			t.Fatalf("forgot for %s: %d %s", email, rec.Code, rec.Body.String())
		}
	}
	release()
	w.h.Wait()
	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.errs) != 1 || sender.errs[0] != nil {
		t.Fatalf("emails sent with context errors %v, want one email with none", sender.errs)
	}
}
