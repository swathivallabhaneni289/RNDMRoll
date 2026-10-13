package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// SpinRepo is a pgx-backed implementation of spin.Repository. It reads and
// writes only the Phase 2 tables (user_time_zones, user_spin_state,
// wheel_categories, spins, posted_days, goal_weeks) and the Phase 3 tables
// (entries, entry_photos; see entry_repo.go) and never touches users.
type SpinRepo struct {
	pool *pgxpool.Pool
}

// NewSpinRepo builds a SpinRepo backed by pool.
func NewSpinRepo(pool *pgxpool.Pool) *SpinRepo {
	return &SpinRepo{pool: pool}
}

var _ spin.Repository = (*SpinRepo)(nil)

// sqlstateLockNotAvailable is raised when lock_timeout expires.
const sqlstateLockNotAvailable = "55P03"

// mapSpinErr turns a lock timeout into spin.ErrBusy (the HTTP layer answers
// 503) and leaves every other error as it is.
func mapSpinErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateLockNotAvailable {
		return spin.ErrBusy
	}
	return err
}

func parseUserID(userID string) (uuid.UUID, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("spin: user id is not a uuid: %w", err)
	}
	return id, nil
}

// Zone implements spin.Repository.
func (r *SpinRepo) Zone(ctx context.Context, userID string) (string, bool, error) {
	uid, err := parseUserID(userID)
	if err != nil {
		return "", false, err
	}
	var name string
	err = r.pool.QueryRow(ctx, `select tz_name from user_time_zones where user_id = $1`, uid).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return name, true, nil
}

// InUserTx implements spin.Repository.
//
// Order inside the transaction: a 5 second lock timeout; insert the state
// row if it is missing (on conflict do nothing); when this call inserted it,
// insert the six starting categories; lock the state row for update; run fn.
// A concurrent first caller's insert waits for ours to commit, does nothing,
// and then waits on the row lock, so it finds the categories.
func (r *SpinRepo) InUserTx(ctx context.Context, userID string, seed spin.Seed, fn func(spin.Tx) error) error {
	uid, err := parseUserID(userID)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapSpinErr(err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()

	if _, err := tx.Exec(ctx, `set local lock_timeout = '5s'`); err != nil {
		return mapSpinErr(err)
	}

	tag, err := tx.Exec(ctx, `
		insert into user_spin_state (user_id, seeded_at, seeded_local_date, seeded_tz, weekly_goal)
		values ($1, $2, $3::date, $4, $5)
		on conflict (user_id) do nothing`,
		uid, seed.At.UTC(), seed.LocalDate.String(), seed.TZName, seed.Goal)
	if err != nil {
		return mapSpinErr(err)
	}
	if tag.RowsAffected() == 1 {
		defaults := spin.DefaultWheel()
		names := make([]string, len(defaults))
		weights := make([]int32, len(defaults))
		positions := make([]int32, len(defaults))
		for i, c := range defaults {
			names[i], weights[i], positions[i] = c.Name, int32(c.Weight), int32(i)
		}
		if _, err := tx.Exec(ctx, `
			insert into wheel_categories (user_id, name, weight, position)
			select $1, t.name, t.weight, t.position
			from unnest($2::text[], $3::int[], $4::int[]) as t(name, weight, position)`,
			uid, names, weights, positions); err != nil {
			return mapSpinErr(err)
		}
	}

	var locked uuid.UUID
	if err := tx.QueryRow(ctx, `select user_id from user_spin_state where user_id = $1 for update`, uid).Scan(&locked); err != nil {
		return mapSpinErr(err)
	}

	if err := fn(&pgTx{ctx: ctx, tx: tx, uid: uid}); err != nil {
		return mapSpinErr(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return mapSpinErr(err)
	}
	committed = true
	return nil
}

// pgTx is one person's transaction. Every statement names the person.
type pgTx struct {
	ctx context.Context
	tx  pgx.Tx
	uid uuid.UUID
}

var _ spin.Tx = (*pgTx)(nil)

func (t *pgTx) State() (spin.UserState, error) {
	var st spin.UserState
	var seededDate string
	err := t.tx.QueryRow(t.ctx, `
		select seeded_at, seeded_local_date::text, seeded_tz, weekly_goal, bonus_held, wheel_version
		from user_spin_state where user_id = $1`, t.uid).
		Scan(&st.SeededAt, &seededDate, &st.SeededTZ, &st.WeeklyGoal, &st.BonusHeld, &st.WheelVersion)
	if err != nil {
		return spin.UserState{}, err
	}
	st.SeededAt = st.SeededAt.UTC()
	if st.SeededLocalDate, err = spin.ParseDate(seededDate); err != nil {
		return spin.UserState{}, err
	}
	return st, nil
}

func (t *pgTx) Zone() (string, bool, error) {
	var name string
	err := t.tx.QueryRow(t.ctx, `select tz_name from user_time_zones where user_id = $1`, t.uid).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return name, true, nil
}

func (t *pgTx) SetZone(name string) error {
	_, err := t.tx.Exec(t.ctx, `
		insert into user_time_zones (user_id, tz_name) values ($1, $2)
		on conflict (user_id) do update set tz_name = excluded.tz_name, updated_at = now()`,
		t.uid, name)
	return err
}

func (t *pgTx) LiveCategories() ([]spin.Category, error) {
	rows, err := t.tx.Query(t.ctx, `
		select id::text, name, weight, position
		from wheel_categories
		where user_id = $1 and archived_at is null
		order by position, created_at, id`, t.uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []spin.Category
	for rows.Next() {
		var c spin.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Weight, &c.Position); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (t *pgTx) CategoryCount() (int, error) {
	var n int
	err := t.tx.QueryRow(t.ctx, `select count(*) from wheel_categories where user_id = $1`, t.uid).Scan(&n)
	return n, err
}

func (t *pgTx) ExistingCategoryIDs(ids []string) (map[string]bool, error) {
	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		u, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("spin: category id is not a uuid: %w", err)
		}
		parsed = append(parsed, u)
	}
	rows, err := t.tx.Query(t.ctx, `
		select id::text from wheel_categories
		where user_id = $1 and id = any($2::uuid[])`, t.uid, parsed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		found[id] = true
	}
	return found, rows.Err()
}

func (t *pgTx) ApplyWheel(change spin.WheelChange, now time.Time) error {
	for _, row := range change.Rows {
		if row.ID == "" {
			if _, err := t.tx.Exec(t.ctx, `
				insert into wheel_categories (user_id, name, weight, position)
				values ($1, $2, $3, $4)`, t.uid, row.Name, row.Weight, row.Position); err != nil {
				return err
			}
			continue
		}
		id, err := uuid.Parse(row.ID)
		if err != nil {
			return fmt.Errorf("spin: category id is not a uuid: %w", err)
		}
		tag, err := t.tx.Exec(t.ctx, `
			update wheel_categories set name = $3, weight = $4, position = $5
			where id = $1 and user_id = $2 and archived_at is null`,
			id, t.uid, row.Name, row.Weight, row.Position)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return errors.New("spin: wheel row names a category that is not live")
		}
	}
	if len(change.Archive) > 0 {
		ids := make([]uuid.UUID, 0, len(change.Archive))
		for _, s := range change.Archive {
			id, err := uuid.Parse(s)
			if err != nil {
				return fmt.Errorf("spin: category id is not a uuid: %w", err)
			}
			ids = append(ids, id)
		}
		if _, err := t.tx.Exec(t.ctx, `
			update wheel_categories set archived_at = $3
			where user_id = $1 and id = any($2::uuid[]) and archived_at is null`,
			t.uid, ids, now.UTC()); err != nil {
			return err
		}
	}
	return nil
}

func (t *pgTx) DeleteUnusedArchived() (int, error) {
	tag, err := t.tx.Exec(t.ctx, `
		delete from wheel_categories c
		where c.user_id = $1 and c.archived_at is not null
		  and not exists (select 1 from spins s where s.category_id = c.id and s.user_id = c.user_id)`, t.uid)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (t *pgTx) BumpWheelVersion() (int, error) {
	var v int
	err := t.tx.QueryRow(t.ctx, `
		update user_spin_state set wheel_version = wheel_version + 1
		where user_id = $1 returning wheel_version`, t.uid).Scan(&v)
	return v, err
}

// spinColumns is the one column list every spin read selects, in the order
// scanSpin reads it. The wheel is selected as text so it can be decoded with
// encoding/json whatever the wire format of jsonb is.
const spinColumns = `id::text, local_date::text, tz_name, kind, seq, category_id::text, category_name,
	wheel::text, winner_index, spun_at, window_opens_at, window_closes_at, result, settled_at, created_at`

func scanSpin(row pgx.Row) (*spin.Spin, error) {
	var s spin.Spin
	var dateStr, kind, wheelText, result string
	if err := row.Scan(&s.ID, &dateStr, &s.TZName, &kind, &s.Seq, &s.CategoryID, &s.CategoryName,
		&wheelText, &s.WinnerIndex, &s.SpunAt, &s.WindowOpensAt, &s.WindowClosesAt, &result, &s.SettledAt, &s.CreatedAt); err != nil {
		return nil, err
	}
	d, err := spin.ParseDate(dateStr)
	if err != nil {
		return nil, err
	}
	s.LocalDate = d
	s.Kind = spin.SpinKind(kind)
	s.Result = spin.SpinResult(result)
	if err := json.Unmarshal([]byte(wheelText), &s.Wheel); err != nil {
		return nil, fmt.Errorf("spin: stored wheel is not valid: %w", err)
	}
	s.SpunAt, s.WindowOpensAt, s.WindowClosesAt, s.CreatedAt = s.SpunAt.UTC(), s.WindowOpensAt.UTC(), s.WindowClosesAt.UTC(), s.CreatedAt.UTC()
	if s.SettledAt != nil {
		settled := s.SettledAt.UTC()
		s.SettledAt = &settled
	}
	return &s, nil
}

func (t *pgTx) spinQuery(where string, args ...any) (*spin.Spin, error) {
	s, err := scanSpin(t.tx.QueryRow(t.ctx, `select `+spinColumns+` from spins where user_id = $1 and `+where, append([]any{t.uid}, args...)...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

func (t *pgTx) DailySpinOfDate(d spin.Date) (*spin.Spin, error) {
	return t.spinQuery(`local_date = $2::date and seq = 0`, d.String())
}

func (t *pgTx) LatestSpinOfDate(d spin.Date) (*spin.Spin, error) {
	return t.spinQuery(`local_date = $2::date order by seq desc limit 1`, d.String())
}

func (t *pgTx) LastDailySpunAt() (*time.Time, error) {
	var at *time.Time
	err := t.tx.QueryRow(t.ctx, `select max(spun_at) from spins where user_id = $1 and kind = 'daily'`, t.uid).Scan(&at)
	if err != nil {
		return nil, err
	}
	if at == nil {
		return nil, nil
	}
	u := at.UTC()
	return &u, nil
}

func (t *pgTx) InsertSpin(s spin.Spin) (spin.Spin, bool, error) {
	categoryID, err := uuid.Parse(s.CategoryID)
	if err != nil {
		return spin.Spin{}, false, fmt.Errorf("spin: category id is not a uuid: %w", err)
	}
	wheel, err := json.Marshal(s.Wheel)
	if err != nil {
		return spin.Spin{}, false, err
	}
	stored, err := scanSpin(t.tx.QueryRow(t.ctx, `
		insert into spins (user_id, local_date, tz_name, kind, seq, category_id, category_name, wheel,
		                   winner_index, spun_at, window_opens_at, window_closes_at)
		values ($1, $2::date, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12)
		on conflict do nothing
		returning `+spinColumns,
		t.uid, s.LocalDate.String(), s.TZName, string(s.Kind), s.Seq, categoryID, s.CategoryName, string(wheel),
		s.WinnerIndex, s.SpunAt.UTC(), s.WindowOpensAt.UTC(), s.WindowClosesAt.UTC()))
	if err == nil {
		return *stored, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return spin.Spin{}, false, err
	}
	// Another writer holds the key. Read their row in this transaction (a
	// unique violation would have aborted it).
	existing, err := t.spinQuery(`local_date = $2::date and seq = $3`, s.LocalDate.String(), s.Seq)
	if err != nil {
		return spin.Spin{}, false, err
	}
	if existing == nil {
		return spin.Spin{}, false, errors.New("spin: insert conflicted with another spin constraint")
	}
	return *existing, false, nil
}

func (t *pgTx) SpendBonus() error {
	tag, err := t.tx.Exec(t.ctx, `
		update user_spin_state set bonus_held = 0
		where user_id = $1 and bonus_held = 1`, t.uid)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return spin.ErrNoBonus
	}
	return nil
}

func (t *pgTx) GrantBonus() (bool, error) {
	tag, err := t.tx.Exec(t.ctx, `
		update user_spin_state set bonus_held = 1
		where user_id = $1 and bonus_held = 0`, t.uid)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (t *pgTx) PostedDay(d spin.Date) (*spin.PostedDay, error) {
	var p spin.PostedDay
	var dateStr, source string
	err := t.tx.QueryRow(t.ctx, `
		select local_date::text, source, late from posted_days
		where user_id = $1 and local_date = $2::date`, t.uid, d.String()).Scan(&dateStr, &source, &p.Late)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if p.Date, err = spin.ParseDate(dateStr); err != nil {
		return nil, err
	}
	p.Source = spin.PostedSource(source)
	return &p, nil
}

func (t *pgTx) PostedDays(from, to spin.Date) ([]spin.PostedDay, error) {
	rows, err := t.tx.Query(t.ctx, `
		select local_date::text, source, late from posted_days
		where user_id = $1 and local_date between $2::date and $3::date
		order by local_date`, t.uid, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []spin.PostedDay
	for rows.Next() {
		var p spin.PostedDay
		var dateStr, source string
		if err := rows.Scan(&dateStr, &source, &p.Late); err != nil {
			return nil, err
		}
		if p.Date, err = spin.ParseDate(dateStr); err != nil {
			return nil, err
		}
		p.Source = spin.PostedSource(source)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (t *pgTx) InsertPostedDay(p spin.PostedDay) (bool, error) {
	tag, err := t.tx.Exec(t.ctx, `
		insert into posted_days (user_id, local_date, source, late)
		values ($1, $2::date, $3, $4)
		on conflict do nothing`, t.uid, p.Date.String(), string(p.Source), p.Late)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (t *pgTx) InsertGoalWeek(g spin.GoalWeek) (bool, error) {
	tag, err := t.tx.Exec(t.ctx, `
		insert into goal_weeks (user_id, week_start, goal, goal_met_at, bonus_granted)
		values ($1, $2::date, $3, $4, $5)
		on conflict do nothing`, t.uid, g.WeekStart.String(), g.Goal, g.MetAt.UTC(), g.BonusGranted)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// ClearTestData deletes the person's entries first (their photo rows go with
// them by cascade, and an entry points at a spin), then ALL their posted days
// of any source, then spins and goal weeks, and sets bonus_held to 0.
func (t *pgTx) ClearTestData() error {
	for _, q := range []string{
		`delete from entries where user_id = $1`,
		`delete from posted_days where user_id = $1`,
		`delete from spins where user_id = $1`,
		`delete from goal_weeks where user_id = $1`,
		`update user_spin_state set bonus_held = 0 where user_id = $1`,
	} {
		if _, err := t.tx.Exec(t.ctx, q, t.uid); err != nil {
			return err
		}
	}
	return nil
}
