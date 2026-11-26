package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// signupRig is the pending sign-up store over the test database, with the
// code rules the handler uses.
type signupRig struct {
	t    *testing.T
	pool *pgxpool.Pool
	repo *postgres.PendingSignupRepo
}

const (
	signupTestTries = 5
	signupTestCodes = 5
	signupTestCap   = 5
)

func newSignupRig(t *testing.T) *signupRig {
	t.Helper()
	pool := requireTestPool(t)
	return &signupRig{t: t, pool: pool, repo: postgres.NewPendingSignupRepo(pool)}
}

// start stores a sign-up of email with username and the code codeHash, made
// at at and live until expiresAt, and answers its id and token hash. It
// fails the test unless the sign-up was stored.
func (r *signupRig) start(email, username string, codeHash []byte, at, expiresAt time.Time) (uuid.UUID, []byte) {
	r.t.Helper()
	p := user.PendingSignup{ID: uuid.New(), NewAccount: newAccount(email, username)}
	tokenHash := []byte("token-" + p.ID.String())
	stored, err := r.repo.Start(context.Background(), p, tokenHash, codeHash, expiresAt, at, at.Add(-user.SignupCapWindow), signupTestCap)
	if err != nil || !stored {
		r.t.Fatalf("Start(%s): stored %v, err %v", email, stored, err)
	}
	return p.ID, tokenHash
}

// tries is how many tries the sign-up id has used, and whether it waits.
func (r *signupRig) tries(id uuid.UUID) (int, bool) {
	r.t.Helper()
	var n int
	err := r.pool.QueryRow(context.Background(), `select attempts from pending_signups where id = $1`, id).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		r.t.Fatal(err)
	}
	return n, true
}

// waiting is how many sign-ups of email wait, in any case.
func (r *signupRig) waiting(email string) int {
	r.t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(), `select count(*) from pending_signups where lower(email) = lower($1)`, email).Scan(&n); err != nil {
		r.t.Fatal(err)
	}
	return n
}

// The cap counts the sign-ups of an email, in any case, started in the
// window before the start: the sixth in an hour is not stored, another
// email is, and an hour after the five the email may start again.
func TestPendingSignupRepo_Start_CapsTheStartsOfAnEmailInTheWindow(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	at := time.Now()
	start := func(email string, at time.Time) bool {
		t.Helper()
		p := user.PendingSignup{ID: uuid.New(), NewAccount: newAccount(email, "cap_user")}
		stored, err := r.repo.Start(ctx, p, []byte("token-"+p.ID.String()), []byte("code"), at.Add(30*time.Minute), at,
			at.Add(-user.SignupCapWindow), signupTestCap)
		if err != nil {
			t.Fatalf("Start(%s): %v", email, err)
		}
		return stored
	}
	for i := range signupTestCap {
		if !start("cap@example.com", at) {
			t.Fatalf("start %d was refused", i+1)
		}
	}
	for _, email := range []string{"cap@example.com", "CAP@Example.com"} {
		if start(email, at) {
			t.Fatalf("a sixth start of %s was stored", email)
		}
	}
	if !start("other-cap@example.com", at) {
		t.Fatal("another email was refused")
	}
	if start("cap@example.com", at.Add(user.SignupCapWindow-time.Second)) {
		t.Fatal("a second short of the hour, the email was let through")
	}
	if !start("cap@example.com", at.Add(user.SignupCapWindow)) {
		t.Fatal("an hour on, the email was refused")
	}
	if n := r.waiting("cap@example.com"); n != signupTestCap+1 {
		t.Fatalf("%d sign-ups of the email, want %d", n, signupTestCap+1)
	}
}

// Find answers a sign-up by its token while its code is live, with every
// field the account will be made with, and user.ErrSignupExpired after, or
// for a token no sign-up has.
func TestPendingSignupRepo_Find_AnswersOnlyALiveSignUp(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	at := time.Now()
	id, tokenHash := r.start("Find-Me@Example.com", "find_me", []byte("code"), at, at.Add(30*time.Minute))
	p, err := r.repo.Find(ctx, tokenHash, at.Add(29*time.Minute))
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if p.ID != id || p.Email != "Find-Me@Example.com" || p.PasswordHash != "bcrypt-hash" || p.Name != "Full Name" ||
		p.Username != "find_me" || p.Bio == nil || *p.Bio != "a short bio" || p.Birthday != "1990-06-15" {
		t.Fatalf("Find: %+v", p)
	}
	for name, find := range map[string]func() (*user.PendingSignup, error){
		"at its expiry":  func() (*user.PendingSignup, error) { return r.repo.Find(ctx, tokenHash, at.Add(30*time.Minute)) },
		"another token":  func() (*user.PendingSignup, error) { return r.repo.Find(ctx, []byte("no-such-token"), at) },
		"an empty token": func() (*user.PendingSignup, error) { return r.repo.Find(ctx, nil, at) },
	} {
		if p, err := find(); !errors.Is(err, user.ErrSignupExpired) || p != nil {
			t.Fatalf("Find %s: %+v, %v, want user.ErrSignupExpired", name, p, err)
		}
	}
}

// Try counts a try only for a wrong code; the right code and the code the
// live one replaced cost nothing. Used up, every code is wrong and nothing
// more is counted; expired or gone, the sign-up is user.ErrSignupExpired.
func TestPendingSignupRepo_Try_CountsOnlyAWrongCode(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	at := time.Now()
	a, b, wrong := []byte("code-a"), []byte("code-b"), []byte("wrong-code")
	id, _ := r.start("try@example.com", "try_me", a, at, at.Add(30*time.Minute))
	try := func(codeHash []byte, at time.Time, wantRight, wantReplaced bool, wantTries int) {
		t.Helper()
		right, replaced, err := r.repo.Try(ctx, id, codeHash, at, signupTestTries)
		if err != nil || right != wantRight || replaced != wantReplaced {
			t.Fatalf("Try(%s): right %v replaced %v err %v, want right %v replaced %v", codeHash, right, replaced, err, wantRight, wantReplaced)
		}
		if tries, _ := r.tries(id); tries != wantTries {
			t.Fatalf("after Try(%s) the sign-up has %d tries, want %d", codeHash, tries, wantTries)
		}
	}
	try(a, at, true, false, 0)
	try(a, at, true, false, 0)
	try(wrong, at, false, false, 1)

	later := at.Add(2 * time.Minute)
	if stored, err := r.repo.Reissue(ctx, id, b, later.Add(30*time.Minute), later, later.Add(-time.Minute), signupTestTries, signupTestCodes); err != nil || !stored {
		t.Fatalf("Reissue: stored %v, err %v", stored, err)
	}
	try(a, later, false, true, 1)
	try(b, later, true, false, 1)
	for n := 2; n <= signupTestTries; n++ {
		try(wrong, later, false, false, n)
	}
	try(b, later, false, false, signupTestTries)
	try(a, later, false, false, signupTestTries)

	for name, tc := range map[string]struct {
		id uuid.UUID
		at time.Time
	}{"expired": {id, later.Add(30 * time.Minute)}, "gone": {uuid.New(), later}} {
		if right, replaced, err := r.repo.Try(ctx, tc.id, b, tc.at, signupTestTries); !errors.Is(err, user.ErrSignupExpired) || right || replaced {
			t.Fatalf("Try %s: right %v replaced %v err %v, want user.ErrSignupExpired", name, right, replaced, err)
		}
	}
}

// Reissue keeps the tries and the code it replaced. A code under a minute old
// stands (no error, nothing stored), even the fifth; a sixth code, a used-up
// sign-up, an expired one and one that is gone are user.ErrSignupExpired.
func TestPendingSignupRepo_Reissue_TheRulesOfANewCode(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	at := time.Now()
	id, _ := r.start("reissue@example.com", "reissue_me", []byte("code-1"), at, at.Add(30*time.Minute))
	reissue := func(id uuid.UUID, code string, at time.Time) (bool, error) {
		t.Helper()
		return r.repo.Reissue(ctx, id, []byte(code), at.Add(30*time.Minute), at, at.Add(-time.Minute), signupTestTries, signupTestCodes)
	}

	if stored, err := reissue(id, "too-soon", at.Add(59*time.Second)); err != nil || stored {
		t.Fatalf("within the minute: stored %v, err %v, want the code to stand", stored, err)
	}
	if _, _, err := r.repo.Try(ctx, id, []byte("wrong"), at, signupTestTries); err != nil {
		t.Fatal(err)
	}
	when := at
	for n := 2; n <= signupTestCodes; n++ {
		when = when.Add(time.Minute)
		if stored, err := reissue(id, "code-"+string(rune('0'+n)), when); err != nil || !stored {
			t.Fatalf("code %d: stored %v, err %v", n, stored, err)
		}
	}
	if tries, _ := r.tries(id); tries != 1 {
		t.Fatalf("after 4 new codes the sign-up has %d tries, want the 1 it had", tries)
	}
	if right, replaced, err := r.repo.Try(ctx, id, []byte("code-4"), when, signupTestTries); err != nil || right || !replaced {
		t.Fatalf("the code the fifth replaced: right %v replaced %v err %v", right, replaced, err)
	}
	if stored, err := reissue(id, "code-6", when.Add(30*time.Second)); err != nil || stored {
		t.Fatalf("within a minute of the fifth: stored %v, err %v, want the code to stand", stored, err)
	}
	if stored, err := reissue(id, "code-6", when.Add(time.Minute)); !errors.Is(err, user.ErrSignupExpired) || stored {
		t.Fatalf("a sixth code: stored %v, err %v, want user.ErrSignupExpired", stored, err)
	}

	used, _ := r.start("used-up@example.com", "used_up", []byte("code"), at, at.Add(30*time.Minute))
	for range signupTestTries {
		if _, _, err := r.repo.Try(ctx, used, []byte("wrong"), at, signupTestTries); err != nil {
			t.Fatal(err)
		}
	}
	for name, tc := range map[string]struct {
		id uuid.UUID
		at time.Time
	}{
		"used up, a minute on":  {used, at.Add(time.Minute)},
		"used up, while fresh":  {used, at.Add(10 * time.Second)},
		"expired":               {used, at.Add(31 * time.Minute)},
		"gone":                  {uuid.New(), at.Add(time.Minute)},
		"the first, expired":    {id, when.Add(31 * time.Minute)},
		"the first, past fresh": {id, when.Add(2 * time.Minute)},
	} {
		if stored, err := reissue(tc.id, "another", tc.at); !errors.Is(err, user.ErrSignupExpired) || stored {
			t.Fatalf("%s: stored %v, err %v, want user.ErrSignupExpired", name, stored, err)
		}
	}
}

// Finish makes the verified account only with the live code, and deletes
// every sign-up of the email with it, in any case, and no other email's.
// With nothing made, nothing is deleted: a replaced code, a username or an
// email taken since the start, and a sign-up that is gone.
func TestPendingSignupRepo_Finish_MakesTheAccountAndDeletesTheEmailsSignUps(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	at := time.Now()
	live := at.Add(30 * time.Minute)
	a, _ := r.start("finish@example.com", "finish_a", []byte("code-a"), at, live)
	b, _ := r.start("Finish@Example.com", "finish_b", []byte("code-b"), at, live)
	r.start("other-finish@example.com", "finish_c", []byte("code-c"), at, live)

	in := newAccount("Finish@Example.com", "finish_b")
	in.EmailVerified = true
	if u, err := r.repo.Finish(ctx, b, []byte("code-a"), in); !errors.Is(err, user.ErrSignupCodeReplaced) || u != nil {
		t.Fatalf("Finish with another code: %v %v, want user.ErrSignupCodeReplaced", u, err)
	}
	if n := r.waiting("finish@example.com"); n != 2 {
		t.Fatalf("a refused Finish deleted sign-ups: %d wait, want 2", n)
	}
	u, err := r.repo.Finish(ctx, b, []byte("code-b"), in)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if !u.EmailVerified || u.EmailVerifiedVia == nil || *u.EmailVerifiedVia != user.VerifiedViaPasswordFlow ||
		u.Email != "Finish@Example.com" || u.Username == nil || *u.Username != "finish_b" || !u.HasBirthday {
		t.Fatalf("the account: %+v", u)
	}
	if n := r.waiting("finish@example.com"); n != 0 {
		t.Fatalf("%d sign-ups of the email still wait", n)
	}
	if n := r.waiting("other-finish@example.com"); n != 1 {
		t.Fatalf("%d sign-ups of another email wait, want 1", n)
	}
	if u, err := r.repo.Finish(ctx, a, []byte("code-a"), newAccount("finish@example.com", "finish_a")); !errors.Is(err, user.ErrSignupExpired) || u != nil {
		t.Fatalf("Finish of a sign-up that went with the account: %v %v, want user.ErrSignupExpired", u, err)
	}

	name, _ := r.start("name-taken@example.com", "finish_b", []byte("code-n"), at, live)
	if _, err := r.repo.Finish(ctx, name, []byte("code-n"), newAccount("name-taken@example.com", "finish_b")); !errors.Is(err, user.ErrUsernameTaken) {
		t.Fatalf("Finish with a username taken since: %v, want user.ErrUsernameTaken", err)
	}
	email, _ := r.start("FINISH@example.com", "finish_e", []byte("code-e"), at, live)
	if _, err := r.repo.Finish(ctx, email, []byte("code-e"), newAccount("FINISH@example.com", "finish_e")); !errors.Is(err, user.ErrEmailTaken) {
		t.Fatalf("Finish with an email taken since: %v, want user.ErrEmailTaken", err)
	}
	for _, id := range []uuid.UUID{name, email} {
		if _, ok := r.tries(id); !ok {
			t.Fatal("a Finish the insert refused deleted its sign-up")
		}
	}
	var accounts int
	if err := r.pool.QueryRow(ctx, `select count(*) from users`).Scan(&accounts); err != nil || accounts != 1 {
		t.Fatalf("%d accounts (err %v), want 1", accounts, err)
	}
}

// Two sign-ups of one email finished at the same moment make one account;
// the other finds its sign-up gone, and neither meets a deadlock.
func TestPendingSignupRepo_Finish_TwoSignUpsOfOneEmailAtOnce(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	at := time.Now()
	for round := range 10 {
		email := "at-once-" + string(rune('a'+round)) + "@example.com"
		ids := []uuid.UUID{}
		for i := range 2 {
			id, _ := r.start(email, "at_once_"+string(rune('a'+round))+string(rune('0'+i)), []byte("code"), at, at.Add(30*time.Minute))
			ids = append(ids, id)
		}
		errs := make([]error, 2)
		var wg sync.WaitGroup
		begin := make(chan struct{})
		for i, id := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-begin
				in := newAccount(email, "at_once_"+string(rune('a'+round))+string(rune('0'+i)))
				in.EmailVerified = true
				_, errs[i] = r.repo.Finish(ctx, id, []byte("code"), in)
			}()
		}
		close(begin)
		wg.Wait()
		made := 0
		for _, err := range errs {
			switch {
			case err == nil:
				made++
			case !errors.Is(err, user.ErrSignupExpired):
				t.Fatalf("round %d: %v, want nil or user.ErrSignupExpired", round, err)
			}
		}
		if made != 1 || r.waiting(email) != 0 {
			t.Fatalf("round %d: %d accounts made, %d sign-ups wait", round, made, r.waiting(email))
		}
	}
}

// The sweep deletes the sign-ups whose code has expired and that started
// over an hour ago, and only those: a young one whose code expired stays
// (the hourly cap counts it), and so does an old one with a live code.
func TestPendingSignupRepo_SweepSignups_DeletesOnlyOldExpiredSignUps(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	now := time.Now()
	oldExpired, _ := r.start("sweep-1@example.com", "sweep_one", []byte("c"), now.Add(-2*time.Hour), now.Add(-90*time.Minute))
	justOld, _ := r.start("sweep-2@example.com", "sweep_two", []byte("c"), now.Add(-61*time.Minute), now.Add(-31*time.Minute))
	youngExpired, _ := r.start("sweep-3@example.com", "sweep_three", []byte("c"), now.Add(-40*time.Minute), now.Add(-10*time.Minute))
	oldLive, _ := r.start("sweep-4@example.com", "sweep_four", []byte("c"), now.Add(-2*time.Hour), now.Add(10*time.Minute))
	fresh, _ := r.start("sweep-5@example.com", "sweep_five", []byte("c"), now, now.Add(30*time.Minute))

	n, err := r.repo.SweepSignups(ctx)
	if err != nil || n != 2 {
		t.Fatalf("SweepSignups: %d, %v, want 2", n, err)
	}
	for id, want := range map[uuid.UUID]bool{oldExpired: false, justOld: false, youngExpired: true, oldLive: true, fresh: true} {
		if _, ok := r.tries(id); ok != want {
			t.Fatalf("after the sweep, sign-up %s waits: %v, want %v", id, ok, want)
		}
	}
	if n, err := r.repo.SweepSignups(ctx); err != nil || n != 0 {
		t.Fatalf("a second sweep: %d, %v, want 0", n, err)
	}
}

// TestMigrations_0023IsReversible: at the latest version pending_signups
// holds the sign-ups waiting for their code. Going down to 22 drops the
// table and leaves every account as it was; going up again adds it back,
// empty, with its indexes.
func TestMigrations_0023IsReversible(t *testing.T) {
	requireMigrationDatabase(t)
	ctx := context.Background()
	if latest := latestMigration(t); latest < 23 {
		t.Fatalf("latest migration is %d, want at least 23", latest)
	}
	count := func(pool *pgxpool.Pool, q string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	tables := func(pool *pgxpool.Pool) int {
		t.Helper()
		return count(pool, `select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'pending_signups'`)
	}
	const indexes = `select count(*) from pg_indexes where schemaname = 'public' and tablename = 'pending_signups'
		and indexname in ('pending_signups_pkey', 'pending_signups_token_hash_key', 'pending_signups_email_idx', 'pending_signups_expires_idx')`

	withFreshPool(t, func(pool *pgxpool.Pool) {
		if _, err := pool.Exec(ctx, "truncate users, pending_signups cascade"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if tables(pool) != 1 || count(pool, indexes) != 4 {
			t.Fatal("no pending_signups table, or not its four indexes, at the latest version")
		}
		if _, err := postgres.NewUserRepo(pool).CreateComplete(ctx, newAccount("m23_kept@example.com", "m23_kept")); err != nil {
			t.Fatalf("CreateComplete: %v", err)
		}
		p := user.PendingSignup{ID: uuid.New(), NewAccount: newAccount("m23_waiting@example.com", "m23_waiting")}
		now := time.Now()
		if stored, err := postgres.NewPendingSignupRepo(pool).Start(ctx, p, []byte("token"), []byte("code"), now.Add(30*time.Minute), now,
			now.Add(-time.Hour), signupTestCap); err != nil || !stored {
			t.Fatalf("Start: %v %v", stored, err)
		}
	})

	migrateGoTo(t, 22)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 22 || dirty {
			t.Fatalf("after goto 22: version %d dirty=%v", v, dirty)
		}
		if tables(pool) != 0 {
			t.Fatal("goto 22 leaves the pending_signups table")
		}
		if n := count(pool, `select count(*) from users where username = 'm23_kept'`); n != 1 {
			t.Fatalf("goto 22 keeps %d of the account, want 1", n)
		}
	})

	migrateGoTo(t, 23)
	withFreshPool(t, func(pool *pgxpool.Pool) {
		if v, dirty := schemaVersion(t, pool); v != 23 || dirty {
			t.Fatalf("after goto 23: version %d dirty=%v", v, dirty)
		}
		if tables(pool) != 1 || count(pool, indexes) != 4 || count(pool, `select count(*) from pending_signups`) != 0 {
			t.Fatal("goto 23 does not add pending_signups back, empty, with its indexes")
		}
		if _, err := pool.Exec(ctx, "truncate users, pending_signups cascade"); err != nil {
			t.Fatalf("final truncate: %v", err)
		}
	})
}
