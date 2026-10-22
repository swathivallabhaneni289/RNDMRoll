package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/mail"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
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
	w.h = NewPasswordResetHandler(users, auth.NewRefreshService(postgres.NewRefreshTokenRepo(pool), time.Hour),
		postgres.NewPasswordResetRepo(pool), sender, integrationJWTSecret, 15*time.Minute)
	w.h.now = func() time.Time { return w.now }
	// No answer floor here: TestPasswordReset_EveryAnswerWaitsForTheFloor
	// checks it, and the rest would wait for it on every reset.
	w.h.floor = 0
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

// codeTries is how many tries the code of email has used, and whether the
// person has a code at all.
func (w *resetWorld) codeTries(email string) (tries int, ok bool) {
	w.t.Helper()
	err := w.pool.QueryRow(context.Background(), `
		select r.attempts from password_resets r join users u on u.id = r.user_id where u.email = $1`, email).Scan(&tries)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		w.t.Fatal(err)
	}
	return tries, true
}

// resetFailing sends a reset whose change, should it get that far, fails at
// its last write, after the new password: Postgres text cannot hold a NUL, so
// the new session's device name cannot be stored.
func (w *resetWorld) resetFailing(email, code, password string, want int) {
	w.t.Helper()
	req := integrationRequest(w.t, http.MethodPost, "/v1/auth/password/reset", "",
		map[string]any{"email": email, "code": code, "new_password": password})
	req.Header.Set("User-Agent", "phone\x00")
	rec := httptest.NewRecorder()
	w.routes.ServeHTTP(rec, req)
	if rec.Code != want {
		w.t.Fatalf("a reset whose change cannot be stored: %d %s, want %d", rec.Code, rec.Body.String(), want)
	}
}

// resetGivenUp sends a reset and gives up while its change waits, as the app
// does when the connection drops: another transaction holds the account's
// row, so the change waits at its first write to it. It returns once the
// reset has finished.
func (w *resetWorld) resetGivenUp(email, code, password string) {
	t := w.t
	t.Helper()
	ctx := context.Background()
	hold, err := w.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Rollback(ctx) //nolint:errcheck // no-op once rolled back
	if _, err := hold.Exec(ctx, `select 1 from users where email = $1 for update`, email); err != nil {
		t.Fatal(err)
	}

	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	req := integrationRequest(t, http.MethodPost, "/v1/auth/password/reset", "",
		map[string]any{"email": email, "code": code, "new_password": password}).WithContext(reqCtx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.routes.ServeHTTP(httptest.NewRecorder(), req)
	}()
	for start := time.Now(); ; time.Sleep(5 * time.Millisecond) {
		var waiting int
		if err := w.pool.QueryRow(ctx, `
			select count(*) from pg_stat_activity where datname = current_database() and wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			break
		}
		if time.Since(start) > 10*time.Second {
			t.Fatal("the change never waited for the held row")
		}
	}
	// The app gives up, as net/http does when the connection drops. Nothing
	// more is sent for the change, so it can no longer commit, and the row is
	// let go at once: what the reset does next may wait for the given-up
	// transaction to end, and must not wait for this test as well.
	cancel()
	if err := hold.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the given-up reset did not finish")
	}
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
	wrongBody := w.reset(email, otherCode(code), fresh, http.StatusBadRequest)
	if wrongBody["field"] != "code" || wrongBody["reason"] != reasonWrongCode || wrongBody["message"] != "That code isn't right or has expired." {
		t.Fatalf("wrong code: %v", wrongBody)
	}
	if got := w.reset("nobody@example.com", code, fresh, http.StatusBadRequest); fmt.Sprint(got) != fmt.Sprint(wrongBody) {
		t.Fatalf("an unknown email: %v, want the wrong-code answer %v", got, wrongBody)
	}

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
// one request can spend the code, in the same transaction as the change.
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
	// A request that lost is told the code was used (it was right when it
	// tried it), or, when it tried after the winner spent it, that it is
	// wrong.
	var won []int
	for i, rec := range integrationAtOnce(w.routes, reqs) {
		if rec.Code == http.StatusOK {
			won = append(won, i)
			continue
		}
		body := decodeBody(t, rec)
		if rec.Code != http.StatusBadRequest || body["field"] != "code" || (body["reason"] != reasonUsed && body["reason"] != reasonWrongCode) {
			t.Fatalf("request %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if len(won) != 1 {
		t.Fatalf("one code changed the password %d times", len(won))
	}
	// The requests that lost gave nothing back: the spent code stays gone.
	if tries, ok := w.codeTries(email); ok {
		t.Fatalf("the spent code is back, with %d tries", tries)
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

// One address may start at most 10 Forgot password requests a minute, whatever
// the emails: forgot starts work after its answer, so a stranger cycling
// through emails cannot pile that work up.
func TestPasswordResetIntegration_OneAddressIsLimited(t *testing.T) {
	w := newResetWorld(t, &captureSender{})
	for i := range 11 {
		req := integrationRequest(t, http.MethodPost, "/v1/auth/password/forgot", "",
			map[string]any{"email": fmt.Sprintf("nobody-%d@example.com", i)})
		req.RemoteAddr = "198.51.100.7:40000"
		rec := httptest.NewRecorder()
		w.routes.ServeHTTP(rec, req)
		w.h.Wait()
		want := http.StatusOK
		if i == 10 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Fatalf("request %d: %d %s, want %d", i+1, rec.Code, rec.Body.String(), want)
		}
	}
}

// The limit of 5 a minute per address and email, which both routes share,
// counts the email the handlers use. JSON keys match in any case and the last
// of two wins, so a decoy email first must not buy more requests.
func TestPasswordResetIntegration_LimitCountsTheEmailTheHandlersUse(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email = "victim@example.com"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Victim", "victim_x")
	const remote = "198.51.100.60:40000"
	send := func(path, body string) *httptest.ResponseRecorder {
		rec := postRawJSON(t, w.routes, remote, path, body)
		w.h.Wait()
		return rec
	}

	for i := range 5 {
		body := fmt.Sprintf(`{"email":"decoy-%d@example.com","Email":"victim@example.com"}`, i)
		if rec := send("/v1/auth/password/forgot", body); rec.Code != http.StatusOK {
			t.Fatalf("forgot %d: %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	// Forgot used the email named last: the code went to the account.
	box.lastCode(t, email)

	for name, c := range map[string]struct{ path, body string }{
		"a 6th forgot behind a new decoy": {"/v1/auth/password/forgot",
			`{"email":"decoy-5@example.com","Email":"victim@example.com"}`},
		"a reset naming the email in capitals": {"/v1/auth/password/reset",
			`{"EMAIL":" Victim@Example.com ","code":"123456","new_password":"Brand-new-password-2!"}`},
	} {
		if rec := send(c.path, c.body); rec.Code != http.StatusTooManyRequests {
			t.Fatalf("%s: %d %s, want 429", name, rec.Code, rec.Body.String())
		}
	}
	// Another email from the same address is not limited.
	if rec := send("/v1/auth/password/forgot", `{"email":"someone-else@example.com"}`); rec.Code != http.StatusOK {
		t.Fatalf("another email: %d %s", rec.Code, rec.Body.String())
	}
}

// A change that fails part way leaves the code as it was: the code is spent
// in the same transaction that sets the password, so both happen or neither,
// and the right code costs no try.
func TestPasswordResetIntegration_FailedChangeKeepsTheCode(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email, old, fresh = "failed-change@example.com", "Correct-horse-battery-staple1", "Brand-new-password-2!"
	integrationOnboardAccount(t, w.engine, email, old, "Failed Change", "failed_change")
	w.forgot(email)
	code := box.lastCode(t, email)

	w.resetFailing(email, code, fresh, http.StatusInternalServerError)

	// Nothing changed: the code is still there with no try counted, the old
	// password still works, and the same code sets the new one.
	if tries, ok := w.codeTries(email); !ok || tries != 0 {
		t.Fatalf("after a failed change the code has %d tries (there: %v), want 0", tries, ok)
	}
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": old}); rec.Code != http.StatusOK {
		t.Fatalf("old password after a failed change: %d", rec.Code)
	}
	w.reset(email, code, fresh, http.StatusOK)
}

// The app giving up while the change waits leaves the code as it was.
func TestPasswordResetIntegration_CancelledChangeKeepsTheCode(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email, old, fresh = "cancelled-change@example.com", "Correct-horse-battery-staple1", "Brand-new-password-2!"
	integrationOnboardAccount(t, w.engine, email, old, "Cancelled Change", "cancelled_change")
	w.forgot(email)
	code := box.lastCode(t, email)

	w.resetGivenUp(email, code, fresh)

	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": old}); rec.Code != http.StatusOK {
		t.Fatalf("old password after a given-up change: %d", rec.Code)
	}
	w.reset(email, code, fresh, http.StatusOK)
}

// The right code on the last try costs no try, so when its change fails the
// code still works. Counted, it would use the code up, and no new code would
// be sent until the old one expired.
func TestPasswordResetIntegration_FailedChangeOnTheLastTryKeepsTheCode(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email, fresh = "failed-last-try@example.com", "Brand-new-password-2!"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Failed Last Try", "failed_last_try")
	w.forgot(email)
	code := box.lastCode(t, email)
	for range resetMaxAttempts - 1 {
		w.reset(email, otherCode(code), fresh, http.StatusBadRequest)
	}

	w.resetFailing(email, code, fresh, http.StatusInternalServerError)
	if tries, ok := w.codeTries(email); !ok || tries != resetMaxAttempts-1 {
		t.Fatalf("after a failed change on the last try the code has %d tries (there: %v), want %d", tries, ok, resetMaxAttempts-1)
	}
	w.reset(email, code, fresh, http.StatusOK)
}

// The same when the app gives up on the change while it waits.
func TestPasswordResetIntegration_CancelledChangeOnTheLastTryKeepsTheCode(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email, old, fresh = "cancelled-last-try@example.com", "Correct-horse-battery-staple1", "Brand-new-password-2!"
	integrationOnboardAccount(t, w.engine, email, old, "Cancelled Last Try", "cancelled_last_try")
	w.forgot(email)
	code := box.lastCode(t, email)
	for range resetMaxAttempts - 1 {
		w.reset(email, otherCode(code), fresh, http.StatusBadRequest)
	}

	w.resetGivenUp(email, code, fresh)
	if tries, ok := w.codeTries(email); !ok || tries != resetMaxAttempts-1 {
		t.Fatalf("after a given-up change on the last try the code has %d tries (there: %v), want %d", tries, ok, resetMaxAttempts-1)
	}
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": old}); rec.Code != http.StatusOK {
		t.Fatalf("old password after a given-up change: %d", rec.Code)
	}
	w.reset(email, code, fresh, http.StatusOK)
}

// Only a wrong code costs a try, and it stays counted, even beside a failed
// change of the right code, so wrong guesses still stop at the limit.
func TestPasswordResetIntegration_WrongTriesStayCounted(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email, fresh = "wrong-tries@example.com", "Brand-new-password-2!"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Wrong Tries", "wrong_tries")
	w.forgot(email)
	code := box.lastCode(t, email)

	// Each wrong code is sent as the failing change is, and counted all the
	// same.
	wrongTry := func(want int) {
		t.Helper()
		w.resetFailing(email, otherCode(code), fresh, http.StatusBadRequest)
		if tries, ok := w.codeTries(email); !ok || tries != want {
			t.Fatalf("after a wrong try the code has %d tries (there: %v), want %d", tries, ok, want)
		}
	}
	wrongTry(1)
	wrongTry(2)
	// The right code's change fails: it costs no try, and the wrong ones
	// stay.
	w.resetFailing(email, code, fresh, http.StatusInternalServerError)
	if tries, ok := w.codeTries(email); !ok || tries != 2 {
		t.Fatalf("after a failed change beside 2 wrong tries the code has %d tries (there: %v), want 2", tries, ok)
	}
	wrongTry(3)
	wrongTry(4)
	wrongTry(5)
	w.reset(email, code, fresh, http.StatusBadRequest)
}

// A new password over bcrypt's 72 bytes, though within 72 characters, is
// refused before the try is counted, so it never uses up the code.
func TestPasswordResetIntegration_PasswordOver72BytesCostsNoTry(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email = "long-bytes@example.com"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Long Bytes", "long_bytes")
	w.forgot(email)
	code := box.lastCode(t, email)

	long := "Aa1!" + strings.Repeat("é", 35) // 39 characters, 74 bytes
	for i := 0; i < resetMaxAttempts; i++ {
		got := w.reset(email, code, long, http.StatusBadRequest)
		if got["field"] != "new_password" || got["message"] != fieldReasonText[reasonTooLong] {
			t.Fatalf("a password over 72 bytes: %v", got)
		}
	}
	if tries, ok := w.codeTries(email); !ok || tries != 0 {
		t.Fatalf("passwords over 72 bytes used %d tries (code there: %v), want 0", tries, ok)
	}
	w.reset(email, code, "Brand-new-password-2!", http.StatusOK)
}

// distinctCodes skips the test in the one-in-a-million case that two emailed
// codes came out the same, which would make an old code the right one.
func distinctCodes(t *testing.T, codes ...string) {
	t.Helper()
	seen := map[string]bool{}
	for _, c := range codes {
		if seen[c] {
			t.Skip("two codes came out the same")
		}
		seen[c] = true
	}
}

// The code a newer one replaced is told apart (400 field "code", reason
// "replaced") and costs no try. Only the code the newest one replaced is: an
// older one, or one whose replacement has been spent, is just wrong.
func TestPasswordResetIntegration_ReplacedCodeIsToldAndCostsNoTry(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email, fresh = "replaced@example.com", "Brand-new-password-2!"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Replaced", "replaced_me")

	w.forgot(email)
	first := box.lastCode(t, email)
	w.reset(email, otherCode(first), fresh, http.StatusBadRequest) // 1 try
	w.now = w.now.Add(2 * time.Minute)
	w.forgot(email)
	second := box.lastCode(t, email)
	distinctCodes(t, first, second)

	for range resetMaxAttempts + 1 {
		got := w.reset(email, first, fresh, http.StatusBadRequest)
		if got["field"] != "code" || got["reason"] != reasonReplaced ||
			got["message"] != "That code was replaced by a newer one. Use the newest code in your email." {
			t.Fatalf("the replaced code: %v", got)
		}
	}
	if tries, ok := w.codeTries(email); !ok || tries != 1 {
		t.Fatalf("after the replaced code 6 times the code has %d tries (there: %v), want 1", tries, ok)
	}

	// A third code: the first is now just wrong and costs a try; the second is
	// the replaced one.
	w.now = w.now.Add(2 * time.Minute)
	w.forgot(email)
	third := box.lastCode(t, email)
	distinctCodes(t, first, second, third)
	if got := w.reset(email, first, fresh, http.StatusBadRequest); got["reason"] != reasonWrongCode {
		t.Fatalf("a code two back: %v", got)
	}
	if got := w.reset(email, second, fresh, http.StatusBadRequest); got["reason"] != reasonReplaced {
		t.Fatalf("the code just replaced: %v", got)
	}
	if tries, ok := w.codeTries(email); !ok || tries != 2 {
		t.Fatalf("the code has %d tries (there: %v), want 2", tries, ok)
	}

	// The newest code works; once it is spent, the replaced one is just wrong.
	w.reset(email, third, fresh, http.StatusOK)
	if got := w.reset(email, second, "Another-password-3!", http.StatusBadRequest); got["reason"] != reasonWrongCode {
		t.Fatalf("the replaced code after the spend: %v", got)
	}
}

// Every refusal of a code other than "replaced" is the same answer, byte for
// byte: a wrong code, an email with no account, a disabled account, a used-up
// code (even the right one, or the one it replaced), an expired code and a
// spent one. The email with no account and the disabled account run the same
// try statement as the others (for no one), so the work matches too.
func TestPasswordResetIntegration_EveryOtherRefusalIsTheSameAnswer(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	store := &countingResetStore{ResetStore: postgres.NewPasswordResetRepo(w.pool)}
	w.h.store = store
	const fresh = "Brand-new-password-2!"
	accountID := func(email string) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := w.pool.QueryRow(context.Background(), `select id from users where email = $1`, email).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	raw := func(email, code string) string {
		t.Helper()
		rec := integrationDo(t, w.routes, http.MethodPost, "/v1/auth/password/reset", "",
			map[string]any{"email": email, "code": code, "new_password": fresh})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("reset for %s: %d %s", email, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	const known = "same-answer@example.com"
	integrationOnboardAccount(t, w.engine, known, "Correct-horse-battery-staple1", "Same Answer", "same_answer")
	w.forgot(known)
	code := box.lastCode(t, known)
	want := raw(known, otherCode(code))
	if !strings.Contains(want, `"reason":"wrong_code"`) || !strings.Contains(want, `"field":"code"`) {
		t.Fatalf("the wrong-code answer: %s", want)
	}
	if got := store.last(); got != accountID(known) {
		t.Fatalf("a wrong code was tried for %s", got)
	}

	// An email with no account: the same answer, after the same statement.
	if got := raw("nobody@example.com", code); got != want {
		t.Fatalf("an unknown email: %s, want %s", got, want)
	}
	if got := store.last(); got != uuid.Nil {
		t.Fatalf("an unknown email was tried for %s, want no one", got)
	}

	// A disabled account, with its right code: the same, and the code is
	// neither spent nor counted.
	const disabled = "same-disabled@example.com"
	integrationOnboardAccount(t, w.engine, disabled, "Correct-horse-battery-staple1", "Same Disabled", "same_disabled")
	w.forgot(disabled)
	disabledCode := box.lastCode(t, disabled)
	if _, err := w.pool.Exec(context.Background(), `update users set disabled_at = now() where email = $1`, disabled); err != nil {
		t.Fatal(err)
	}
	if got := raw(disabled, disabledCode); got != want {
		t.Fatalf("a disabled account: %s, want %s", got, want)
	}
	if got := store.last(); got != uuid.Nil {
		t.Fatalf("a disabled account was tried for %s, want no one", got)
	}
	if tries, ok := w.codeTries(disabled); !ok || tries != 0 {
		t.Fatalf("the disabled account's code has %d tries (there: %v), want 0", tries, ok)
	}

	// Used up: the right code, and the code it replaced, get the same answer.
	w.now = w.now.Add(2 * time.Minute)
	w.forgot(known)
	newer := box.lastCode(t, known)
	distinctCodes(t, code, newer)
	for range resetMaxAttempts - 1 {
		raw(known, otherCode(newer))
	}
	for _, c := range []string{newer, code} {
		if got := raw(known, c); got != want {
			t.Fatalf("code %s of a used-up code: %s, want %s", c, got, want)
		}
	}

	// Expired: a new code once the old one has run out, then that one expires
	// too.
	w.now = w.now.Add(resetCodeTTL)
	w.forgot(known)
	last := box.lastCode(t, known)
	w.now = w.now.Add(resetCodeTTL + time.Second)
	if got := raw(known, last); got != want {
		t.Fatalf("an expired code: %s, want %s", got, want)
	}

	// Spent: a code that set the password.
	w.now = w.now.Add(time.Second)
	w.forgot(known)
	spent := box.lastCode(t, known)
	w.reset(known, spent, fresh, http.StatusOK)
	if got := raw(known, spent); got != want {
		t.Fatalf("a spent code: %s, want %s", got, want)
	}
}

// countingResetStore records whom each try was for.
type countingResetStore struct {
	ResetStore
	mu    sync.Mutex
	tries []uuid.UUID
}

func (s *countingResetStore) Try(ctx context.Context, userID uuid.UUID, codeHash []byte, at time.Time, maxTries int) (bool, bool, error) {
	s.mu.Lock()
	s.tries = append(s.tries, userID)
	s.mu.Unlock()
	return s.ResetStore.Try(ctx, userID, codeHash, at, maxTries)
}

// last is whom the latest try was for.
func (s *countingResetStore) last() uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.tries) == 0 {
		return uuid.UUID{1}
	}
	return s.tries[len(s.tries)-1]
}

// failingEmailLookup is a user.Repository whose email lookup fails.
type failingEmailLookup struct{ user.Repository }

func (failingEmailLookup) GetByEmailCI(context.Context, string) (*user.User, error) {
	return nil, errors.New("httpapi test: the database is down")
}

// A lookup that fails is a server error, not a wrong code, which the app
// would count as a try.
func TestPasswordReset_LookupFailureIsNotAWrongCode(t *testing.T) {
	store := &countingResetStore{}
	h := NewPasswordResetHandler(failingEmailLookup{newFakeUserRepo()}, auth.NewRefreshService(newFakeRefreshRepo(), time.Hour),
		store, &captureSender{}, []byte("test-secret"), 15*time.Minute)
	h.floor = 0
	router := newTestRouter(t, TestDeps{})
	h.Register(router.Group("/v1"))

	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/password/reset",
		map[string]any{"email": "someone@example.com", "code": "123456", "new_password": "Brand-new-password-2!"})
	if rec.Code != http.StatusInternalServerError || decodeBody(t, rec)["error"] != string(CodeServerError) {
		t.Fatalf("a failed lookup: %d %s, want 500 server_error", rec.Code, rec.Body.String())
	}
	if len(store.tries) != 0 {
		t.Fatalf("a failed lookup still tried the code %d times", len(store.tries))
	}
}

// hookedResetStore runs afterRightTry once, when a try finds the right code,
// before the change: whatever another request, a new code or an admin does
// in that moment.
type hookedResetStore struct {
	ResetStore
	afterRightTry func()
}

func (s *hookedResetStore) Try(ctx context.Context, userID uuid.UUID, codeHash []byte, at time.Time, maxTries int) (bool, bool, error) {
	right, replaced, err := s.ResetStore.Try(ctx, userID, codeHash, at, maxTries)
	if hook := s.afterRightTry; right && hook != nil {
		s.afterRightTry = nil
		hook()
	}
	return right, replaced, err
}

// hook makes the world's store run afterRightTry at the next right try.
func (w *resetWorld) hook(afterRightTry func()) {
	w.h.store = &hookedResetStore{ResetStore: postgres.NewPasswordResetRepo(w.pool), afterRightTry: afterRightTry}
}

// The right code that a new code replaces while its change runs (the person
// asked again at that moment) is told "replaced", as the code the newest one
// replaced always is: no try is counted, nothing changes, and the newest code
// works.
func TestPasswordResetIntegration_RightCodeReplacedMeanwhileIsToldReplaced(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email, old, fresh = "replaced-meanwhile@example.com", "Correct-horse-battery-staple1", "Brand-new-password-2!"
	integrationOnboardAccount(t, w.engine, email, old, "Replaced Meanwhile", "replaced_mw")
	w.forgot(email)
	code := box.lastCode(t, email)

	w.hook(func() {
		w.now = w.now.Add(2 * time.Minute)
		w.h.sendCode(context.Background(), email)
	})
	got := w.reset(email, code, fresh, http.StatusBadRequest)
	if got["field"] != "code" || got["reason"] != reasonReplaced || got["message"] != fieldReasonText[reasonReplaced] {
		t.Fatalf("the right code replaced meanwhile: %v", got)
	}
	if tries, ok := w.codeTries(email); !ok || tries != 0 {
		t.Fatalf("the code has %d tries (there: %v), want 0", tries, ok)
	}
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": old}); rec.Code != http.StatusOK {
		t.Fatalf("the old password: %d", rec.Code)
	}
	newest := box.lastCode(t, email)
	distinctCodes(t, code, newest)
	w.reset(email, newest, fresh, http.StatusOK)
}

// The right code that another request used first (the app sent it twice, or
// a second phone) is told "used", costs no try, and changes nothing more: the
// password is the one the first request set.
func TestPasswordResetIntegration_RightCodeUsedMeanwhileIsToldUsed(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email, first, second = "used-meanwhile@example.com", "First-new-password-2!", "Second-new-password-3!"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Used Meanwhile", "used_mw")
	w.forgot(email)
	code := box.lastCode(t, email)

	w.hook(func() { w.reset(email, code, first, http.StatusOK) })
	got := w.reset(email, code, second, http.StatusBadRequest)
	if got["field"] != "code" || got["reason"] != reasonUsed || got["message"] != fieldReasonText[reasonUsed] {
		t.Fatalf("the right code used meanwhile: %v", got)
	}
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": first}); rec.Code != http.StatusOK {
		t.Fatalf("the first request's password: %d", rec.Code)
	}
	if rec := integrationDo(t, w.engine, http.MethodPost, "/v1/auth/login", "", map[string]any{"login": email, "password": second}); rec.Code == http.StatusOK {
		t.Fatal("the second request changed the password too")
	}
	// Typed again later, the spent code is simply wrong.
	if got := w.reset(email, code, second, http.StatusBadRequest); got["reason"] != reasonWrongCode {
		t.Fatalf("the spent code later: %v", got)
	}
}

// An account an admin disables while its reset runs keeps its password, and
// the reset gets the answer any wrong code gets, byte for byte; the disable
// deletes the code.
func TestPasswordResetIntegration_AnAccountDisabledMeanwhileGetsTheWrongCodeAnswer(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	const email, old, fresh = "disabled-meanwhile@example.com", "Correct-horse-battery-staple1", "Brand-new-password-2!"
	integrationOnboardAccount(t, w.engine, email, old, "Disabled Meanwhile", "disabled_mw")
	w.forgot(email)
	code := box.lastCode(t, email)
	raw := func(code string) string {
		t.Helper()
		rec := integrationDo(t, w.routes, http.MethodPost, "/v1/auth/password/reset", "",
			map[string]any{"email": email, "code": code, "new_password": fresh})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	want := raw(otherCode(code))
	var id uuid.UUID
	var hashBefore string
	if err := w.pool.QueryRow(context.Background(), `select id, password_hash from users where email = $1`, email).Scan(&id, &hashBefore); err != nil {
		t.Fatal(err)
	}

	w.hook(func() {
		at := time.Now()
		if err := postgres.NewAdminRepo(w.pool).SetDisabled(context.Background(), id, &at); err != nil {
			t.Error(err)
		}
	})
	if got := raw(code); got != want {
		t.Fatalf("disabled while the reset ran: %s, want the wrong-code answer %s", got, want)
	}
	var hashAfter string
	if err := w.pool.QueryRow(context.Background(), `select password_hash from users where id = $1`, id).Scan(&hashAfter); err != nil || hashAfter != hashBefore {
		t.Fatalf("the password changed (err %v)", err)
	}
	if _, ok := w.codeTries(email); ok {
		t.Fatal("the disable kept the code")
	}
}

// Every answer of reset comes no sooner than the floor after the request came
// in, whatever it is: a body that is not JSON, a code of the wrong shape, a
// weak password, a wrong code for an account (a try is written), an email
// with no account and a disabled account (nothing is written), the replaced
// code, the used code, a limit, and the new session. The wait is asked for
// before the first byte, once per answer; here the clock stands still, so it
// is the whole floor every time.
func TestPasswordResetIntegration_EveryAnswerWaitsForTheFloor(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	var waits []time.Duration
	w.h.floor = resetAnswerFloor
	w.h.sleep = func(d time.Duration) { waits = append(waits, d) }
	const email, fresh = "floor@example.com", "Brand-new-password-2!"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Floor", "floor_me")
	const disabled = "floor-disabled@example.com"
	integrationOnboardAccount(t, w.engine, disabled, "Correct-horse-battery-staple1", "Floor Disabled", "floor_disabled")
	if _, err := w.pool.Exec(context.Background(), `update users set disabled_at = now() where email = $1`, disabled); err != nil {
		t.Fatal(err)
	}
	w.forgot(email)
	first := box.lastCode(t, email)
	w.now = w.now.Add(2 * time.Minute)
	w.forgot(email)
	code := box.lastCode(t, email)
	distinctCodes(t, first, code)

	send := func(what string, body string, wantStatus int, wantReason string) {
		t.Helper()
		waits = nil
		rec := postRawJSON(t, w.routes, integrationRemote(), "/v1/auth/password/reset", body)
		w.h.Wait()
		if rec.Code != wantStatus {
			t.Fatalf("%s: %d %s, want %d", what, rec.Code, rec.Body.String(), wantStatus)
		}
		if wantReason != "" && decodeBody(t, rec)["reason"] != wantReason {
			t.Fatalf("%s: %s, want reason %s", what, rec.Body.String(), wantReason)
		}
		if len(waits) == 0 || waits[len(waits)-1] != resetAnswerFloor {
			t.Fatalf("%s: waits %v, want the answer to wait the floor %v", what, waits, resetAnswerFloor)
		}
	}
	body := func(email, code, password string) string {
		return fmt.Sprintf(`{"email":%q,"code":%q,"new_password":%q}`, email, code, password)
	}
	send("not JSON", `nope`, http.StatusBadRequest, "")
	send("a code of the wrong shape", body(email, "12345", fresh), http.StatusBadRequest, reasonInvalid)
	send("a weak password", body(email, code, "weak"), http.StatusBadRequest, reasonTooShort)
	send("a wrong code", body(email, otherCode(code), fresh), http.StatusBadRequest, reasonWrongCode)
	send("an email with no account", body("nobody@example.com", code, fresh), http.StatusBadRequest, reasonWrongCode)
	send("a disabled account", body(disabled, code, fresh), http.StatusBadRequest, reasonWrongCode)
	send("the replaced code", body(email, first, fresh), http.StatusBadRequest, reasonReplaced)
	w.hook(func() { w.reset(email, code, "Other-new-password-4!", http.StatusOK) })
	send("the used code", body(email, code, fresh), http.StatusBadRequest, reasonUsed)
	w.forgot(email) // the last code was spent, so a new one comes at once
	send("the new session", body(email, box.lastCode(t, email), fresh), http.StatusOK, "")
	// The per-address and email limit: five a minute.
	remote := "198.51.100.90:1"
	for i := range 6 {
		waits = nil
		rec := postRawJSON(t, w.routes, remote, "/v1/auth/password/reset", body("limited@example.com", "123456", fresh))
		want := http.StatusBadRequest
		if i == 5 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want || len(waits) != 1 || waits[0] != resetAnswerFloor {
			t.Fatalf("limit try %d: %d, waits %v", i+1, rec.Code, waits)
		}
	}
}

// The floor holds the answer itself: on the real clock, a wrong code and a
// new session both take at least the floor.
func TestPasswordResetIntegration_TheFloorHoldsTheAnswer(t *testing.T) {
	box := &captureSender{}
	w := newResetWorld(t, box)
	w.h.now = time.Now
	w.h.floor = 150 * time.Millisecond
	const email = "floor-real@example.com"
	integrationOnboardAccount(t, w.engine, email, "Correct-horse-battery-staple1", "Floor Real", "floor_real")
	w.forgot(email)
	code := box.lastCode(t, email)
	for _, c := range []struct {
		code string
		want int
	}{{otherCode(code), http.StatusBadRequest}, {code, http.StatusOK}} {
		start := time.Now()
		w.reset(email, c.code, "Brand-new-password-2!", c.want)
		if took := time.Since(start); took < w.h.floor {
			t.Fatalf("an answer %d came after %v, sooner than the floor %v", c.want, took, w.h.floor)
		}
	}
}

// A lookup that fails waits for the floor too.
func TestPasswordReset_ALookupFailureWaitsForTheFloor(t *testing.T) {
	h := NewPasswordResetHandler(failingEmailLookup{newFakeUserRepo()}, auth.NewRefreshService(newFakeRefreshRepo(), time.Hour),
		&countingResetStore{}, &captureSender{}, []byte("test-secret"), 15*time.Minute)
	if h.floor != resetAnswerFloor {
		t.Fatalf("a new handler's floor is %v, want %v", h.floor, resetAnswerFloor)
	}
	var waits []time.Duration
	h.now = frozenClock
	h.sleep = func(d time.Duration) { waits = append(waits, d) }
	router := newTestRouter(t, TestDeps{})
	h.Register(router.Group("/v1"))
	rec := doJSONRequest(t, router, http.MethodPost, "/v1/auth/password/reset",
		map[string]any{"email": "someone@example.com", "code": "123456", "new_password": "Brand-new-password-2!"})
	if rec.Code != http.StatusInternalServerError || len(waits) != 1 || waits[0] != resetAnswerFloor {
		t.Fatalf("a failed lookup: %d, waits %v", rec.Code, waits)
	}
}

// The floor waits on a timer, never on the request's context: a client that
// half-closes its connection ends the context but still reads the answer, so
// the answer must come no sooner than the floor all the same.
func TestPasswordReset_TheFloorOutlastsAnEndedRequestContext(t *testing.T) {
	h := NewPasswordResetHandler(failingEmailLookup{newFakeUserRepo()}, auth.NewRefreshService(newFakeRefreshRepo(), time.Hour),
		&countingResetStore{}, &captureSender{}, []byte("test-secret"), 15*time.Minute)
	// The handler's own sleep and the real clock, with a short floor.
	h.floor = 80 * time.Millisecond
	router := newTestRouter(t, TestDeps{})
	h.Register(router.Group("/v1"))
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/password/reset",
		strings.NewReader(`{"email":"someone@example.com","code":"123456","new_password":"Brand-new-password-2!"}`)).WithContext(ended)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	start := time.Now()
	router.ServeHTTP(rec, req)
	if took := time.Since(start); took < h.floor {
		t.Fatalf("with the request's context ended, the answer came after %v, sooner than the floor %v", took, h.floor)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("a failed lookup: %d %s", rec.Code, rec.Body.String())
	}
}

// wrongCodeStore answers every code as wrong.
type wrongCodeStore struct{ ResetStore }

func (wrongCodeStore) Try(context.Context, uuid.UUID, []byte, time.Time, int) (bool, bool, error) {
	return false, false, nil
}

// A code that is not exactly six ASCII digits is told invalid and is never
// checked against the store, so it costs no try. Six bytes are not enough:
// each code below is six bytes long. A code with spaces around it is still
// checked.
func TestPasswordReset_ACodeOfTheWrongShapeCostsNoTry(t *testing.T) {
	store := &countingResetStore{ResetStore: wrongCodeStore{}}
	h := NewPasswordResetHandler(newFakeUserRepo(), auth.NewRefreshService(newFakeRefreshRepo(), time.Hour),
		store, &captureSender{}, []byte("test-secret"), 15*time.Minute)
	h.now = frozenClock
	h.floor = 0
	router := newTestRouter(t, TestDeps{})
	h.Register(router.Group("/v1"))
	send := func(n int, code string) map[string]any {
		t.Helper()
		body := fmt.Sprintf(`{"email":"someone@example.com","code":%q,"new_password":"Brand-new-password-2!"}`, code)
		rec := postRawJSON(t, router, fmt.Sprintf("198.51.100.%d:1", n), "/v1/auth/password/reset", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("code %q: %d %s, want 400", code, rec.Code, rec.Body.String())
		}
		return decodeBody(t, rec)
	}
	for i, code := range []string{"abcdef", "12345a", "1234é", "١٢٣", "１２", "+12345", "12 456", "12345", "1234567", ""} {
		got := send(i+1, code)
		if got["field"] != "code" || got["reason"] != reasonInvalid {
			t.Fatalf("code %q: %v, want field code, reason %s", code, got, reasonInvalid)
		}
	}
	if len(store.tries) != 0 {
		t.Fatalf("codes of the wrong shape were tried %d times, want none", len(store.tries))
	}
	if got := send(50, " 123456 "); got["reason"] != reasonWrongCode || len(store.tries) != 1 {
		t.Fatalf("a padded code: %v, %d tries; want it checked once, as a wrong code", got, len(store.tries))
	}
}
