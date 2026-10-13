package spin

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
)

const (
	// maxTestDays is the most pretend posted days one test call may mark.
	maxTestDays = 7
	// maxTestDayAgeDays is how far back a pretend posted day may reach.
	maxTestDayAgeDays = 60
)

// Service is the daily spin: one spin a day from 8:00 PM local, one weekly
// bonus spin, the wheel, and the posted-day count that earns the bonus. The
// server decides the date, the window and the winner; the phone only
// reports its time zone.
//
// Every call that changes or reads a person's spin state runs inside one
// transaction that holds the person's state row lock, re-reads the stored
// zone under that lock and reads the clock once after it. Nothing here calls
// the time package for "now": the clock is injected.
type Service struct {
	repo  Repository
	clock clock.Clock
	rand  Rand
}

// NewService builds a Service. clk is the spin slice's own clock (per
// account in test mode, real time otherwise); r picks winners.
func NewService(repo Repository, clk clock.Clock, r Rand) *Service {
	return &Service{repo: repo, clock: clk, rand: r}
}

// call is what one request knows after the lock is taken.
type call struct {
	tx     Tx
	userID string
	now    time.Time
	// loc and tzName are the stored zone. loc is nil only when the call did
	// not require a zone and none is stored.
	loc    *time.Location
	tzName string
	// today is the local date of now in loc (zero when loc is nil).
	today Date
}

// run takes the person's lock and calls fn. When requireZone is true a
// person with no readable stored zone gets ErrZoneMissing, before any
// transaction is opened if the zone is already known to be missing.
func (s *Service) run(ctx context.Context, userID string, requireZone bool, fn func(c *call) error) error {
	// A read before the lock decides the seed and spares a person with no
	// zone a transaction. The zone is read again under the lock below.
	name, hasZone, err := s.repo.Zone(ctx, userID)
	if err != nil {
		return err
	}
	var preLoc *time.Location
	if hasZone {
		preLoc, _ = loadStoredZone(name)
	}
	if requireZone && preLoc == nil {
		return ErrZoneMissing
	}

	at := s.clock.NowFor(userID)
	seed := Seed{At: at, Goal: DefaultWeeklyGoal, TZName: "UTC", LocalDate: LocalDate(at, time.UTC)}
	if preLoc != nil {
		seed.TZName = name
		seed.LocalDate = LocalDate(at, preLoc)
	}

	return s.repo.InUserTx(ctx, userID, seed, func(tx Tx) error {
		c := &call{tx: tx, userID: userID}
		storedName, ok, err := tx.Zone()
		if err != nil {
			return err
		}
		if ok {
			if loc, err := loadStoredZone(storedName); err == nil {
				c.loc, c.tzName = loc, storedName
			}
		}
		if requireZone && c.loc == nil {
			return ErrZoneMissing
		}
		c.now = s.clock.NowFor(userID)
		if c.loc != nil {
			c.today = LocalDate(c.now, c.loc)
		}
		return fn(c)
	})
}

// daySettled is the one rule for a settled day: a posted-day row exists, or
// the date's latest spin's result is not open. The latest spin is the one
// with the highest seq: a bonus spin leaves seq 0 at open for good, and the
// entry phase sets posted and skipped on the latest spin.
func daySettled(posted *PostedDay, latest *Spin) bool {
	return posted != nil || (latest != nil && latest.Result != ResultOpen)
}

// dayFacts returns the date's latest spin as the screen shows it (its result
// is posted when a posted-day row exists, else the stored value) and whether
// the date is settled.
func dayFacts(tx Tx, d Date) (latest *Spin, settled bool, err error) {
	latest, err = tx.LatestSpinOfDate(d)
	if err != nil {
		return nil, false, err
	}
	posted, err := tx.PostedDay(d)
	if err != nil {
		return nil, false, err
	}
	settled = daySettled(posted, latest)
	if latest != nil && posted != nil {
		shown := *latest
		shown.Result = ResultPosted
		latest = &shown
	}
	return latest, settled, nil
}

func entriesOf(cats []Category) []WheelEntry {
	entries := make([]WheelEntry, len(cats))
	for i, c := range cats {
		entries[i] = WheelEntry{ID: c.ID, Name: c.Name, Weight: c.Weight}
	}
	return entries
}

// entrySummaryOf returns the summary of the entry of date d, or nil. It reads
// the entry only when the date's latest spin, as dayFacts returns it, is
// posted or skipped: no other state shows an entry.
func entrySummaryOf(tx Tx, d Date, latest *Spin) (*EntryRef, error) {
	if latest == nil || (latest.Result != ResultPosted && latest.Result != ResultSkipped) {
		return nil, nil
	}
	e, err := tx.EntryOfDate(d)
	if err != nil {
		return nil, err
	}
	return EntryRefOf(e), nil
}

// todayIn builds the Today screen from the facts under the lock.
func (s *Service) todayIn(c *call) (Today, error) {
	tx := c.tx
	prev := PrevDate(c.today, c.loc)

	spin, settled, err := dayFacts(tx, c.today)
	if err != nil {
		return Today{}, err
	}
	yesterday, _, err := dayFacts(tx, prev)
	if err != nil {
		return Today{}, err
	}
	entry, err := entrySummaryOf(tx, c.today, spin)
	if err != nil {
		return Today{}, err
	}
	yesterdayEntry, err := entrySummaryOf(tx, prev, yesterday)
	if err != nil {
		return Today{}, err
	}
	last, err := tx.LastDailySpunAt()
	if err != nil {
		return Today{}, err
	}
	cats, err := tx.LiveCategories()
	if err != nil {
		return Today{}, err
	}
	st, err := tx.State()
	if err != nil {
		return Today{}, err
	}
	weekStart := WeekStart(c.today)
	weekEnd := weekStart.AddDays(daysInWeek - 1)
	posted, err := tx.PostedDays(weekStart, weekEnd)
	if err != nil {
		return Today{}, err
	}
	dates := make([]Date, len(posted))
	for i, p := range posted {
		dates[i] = p.Date
	}
	skipped, err := tx.SkippedDates(weekStart, weekEnd)
	if err != nil {
		return Today{}, err
	}

	return BuildToday(TodayInput{
		Now:             c.now,
		Loc:             c.loc,
		TZName:          c.tzName,
		Spin:            spin,
		YesterdaySpin:   yesterday,
		Entry:           entry,
		YesterdayEntry:  yesterdayEntry,
		Settled:         settled,
		LastDailySpunAt: last,
		Wheel:           entriesOf(cats),
		BonusHeld:       st.BonusHeld == 1,
		Goal:            st.WeeklyGoal,
		PostedDates:     dates,
		SkippedDates:    skipped,
	}), nil
}

// --- time zone ---

// SetTimeZone stores the zone the phone reports and returns the stored name.
// A name that is not a real zone is ErrZoneInvalid.
func (s *Service) SetTimeZone(ctx context.Context, userID, name string) (string, error) {
	loc, err := ParseZone(name)
	if err != nil {
		return "", err
	}
	at := s.clock.NowFor(userID)
	seed := Seed{At: at, LocalDate: LocalDate(at, loc), TZName: name, Goal: DefaultWeeklyGoal}
	err = s.repo.InUserTx(ctx, userID, seed, func(tx Tx) error {
		return tx.SetZone(name)
	})
	if err != nil {
		return "", err
	}
	return name, nil
}

// ZoneOf returns the person's stored zone. ErrZoneMissing when none is
// stored or the stored name no longer loads.
func (s *Service) ZoneOf(ctx context.Context, userID string) (*time.Location, string, error) {
	name, ok, err := s.repo.Zone(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", ErrZoneMissing
	}
	loc, err := loadStoredZone(name)
	if err != nil {
		return nil, "", err
	}
	return loc, name, nil
}

// --- Today, the daily spin and the bonus spin ---

// Today returns the Today screen's data. A first call makes the person's
// state row and starting wheel.
func (s *Service) Today(ctx context.Context, userID string) (Today, error) {
	var out Today
	err := s.run(ctx, userID, true, func(c *call) error {
		t, err := s.todayIn(c)
		if err != nil {
			return err
		}
		out = t
		return nil
	})
	return out, err
}

// SpinOutcome is the answer to a spin or a bonus spin: whether the spin
// already existed (a retry or a second phone) and Today as of the same
// instant.
type SpinOutcome struct {
	Already bool
	Today   Today
}

// Spin makes the day's daily spin, or returns it when it exists.
//
// forDate is the date the phone believes it is spinning for (nil means the
// server's date). In order: a daily spin already on forDate is returned with
// Already true, so an answer lost at 23:59:58 and retried at 00:00:01 still
// gets its spin; forDate before the server date is ErrClosed and after it
// ErrNotOpen; before 8:00 PM, or under 12 hours after the last daily spin,
// is ErrNotOpen; otherwise the server picks by weight and stores the spin
// with a copy of the wheel it used.
func (s *Service) Spin(ctx context.Context, userID string, forDate *Date) (SpinOutcome, error) {
	var out SpinOutcome
	err := s.run(ctx, userID, true, func(c *call) error {
		tx := c.tx
		date := c.today
		if forDate != nil {
			date = *forDate
		}
		existing, err := tx.DailySpinOfDate(date)
		if err != nil {
			return err
		}
		if existing != nil {
			out.Already = true
		} else {
			if err := s.makeDailySpin(c, date); err != nil {
				return err
			}
		}
		t, err := s.todayIn(c)
		if err != nil {
			return err
		}
		out.Today = t
		return nil
	})
	if err != nil {
		return SpinOutcome{}, err
	}
	return out, nil
}

func (s *Service) makeDailySpin(c *call, date Date) error {
	switch {
	case date.Before(c.today):
		return ErrClosed
	case date.After(c.today):
		return ErrNotOpen
	}
	window := WindowFor(c.today, c.loc)
	if c.now.Before(window.OpensAt) {
		return ErrNotOpen
	}
	last, err := c.tx.LastDailySpunAt()
	if err != nil {
		return err
	}
	if !gapAllowsSpin(c.now, last) {
		return ErrNotOpen
	}

	cats, err := c.tx.LiveCategories()
	if err != nil {
		return err
	}
	wheel := entriesOf(cats)
	weights := make([]int, len(wheel))
	for i, e := range wheel {
		weights[i] = e.Weight
	}
	winner := Pick(weights, s.rand)
	if winner < 0 {
		return errors.New("spin: the wheel has nothing to pick")
	}

	_, inserted, err := c.tx.InsertSpin(Spin{
		LocalDate:      c.today,
		TZName:         c.tzName,
		Kind:           KindDaily,
		Seq:            0,
		CategoryID:     wheel[winner].ID,
		CategoryName:   wheel[winner].Name,
		Wheel:          wheel,
		WinnerIndex:    winner,
		SpunAt:         c.now,
		WindowOpensAt:  window.OpensAt,
		WindowClosesAt: window.ClosesAt,
		Result:         ResultOpen,
	})
	if err != nil {
		return err
	}
	if !inserted {
		// Someone else's spin for this date was stored first. Under the
		// state lock this cannot happen; refuse rather than guess.
		return errors.New("spin: the daily spin already exists")
	}
	return nil
}

// Bonus spends the held bonus spin on a new spin for the same day. The new
// category is never the current one and is drawn from the wheel snapshot of
// the day's own spin.
//
// afterSeq is the number of the spin the phone is looking at and forDate the
// date it believes it is; both are required. If forDate is not the server's
// date: the date's latest spin is returned (Already true) when its number is
// above afterSeq, else the answer is ErrClosed (earlier date) or ErrNotOpen
// (later date), and nothing is spent. On the server's date: a latest spin
// above afterSeq is returned with Already true and nothing is spent (so a
// retry or a second phone is safe); an afterSeq above the latest, or a day
// with no daily spin after 8:00 PM, is an after_seq problem; a settled day is
// ErrDaySettled; no held bonus (or one already used that day) is ErrNoBonus.
func (s *Service) Bonus(ctx context.Context, userID string, afterSeq int, forDate Date) (SpinOutcome, error) {
	var out SpinOutcome
	err := s.run(ctx, userID, true, func(c *call) error {
		already, err := s.bonusIn(c, afterSeq, forDate)
		if err != nil {
			return err
		}
		t, err := s.todayIn(c)
		if err != nil {
			return err
		}
		out = SpinOutcome{Already: already, Today: t}
		return nil
	})
	if err != nil {
		return SpinOutcome{}, err
	}
	return out, nil
}

func (s *Service) bonusIn(c *call, afterSeq int, forDate Date) (already bool, err error) {
	tx := c.tx
	if forDate != c.today {
		latest, err := tx.LatestSpinOfDate(forDate)
		if err != nil {
			return false, err
		}
		if latest != nil && latest.Seq > afterSeq {
			return true, nil
		}
		if forDate.Before(c.today) {
			return false, ErrClosed
		}
		return false, ErrNotOpen
	}

	daily, err := tx.DailySpinOfDate(c.today)
	if err != nil {
		return false, err
	}
	if daily == nil {
		if c.now.Before(WindowFor(c.today, c.loc).OpensAt) {
			return false, ErrNotOpen
		}
		return false, &InputError{Field: "after_seq", Reason: "invalid"}
	}
	latest, err := tx.LatestSpinOfDate(c.today)
	if err != nil {
		return false, err
	}
	if latest == nil {
		latest = daily
	}
	switch {
	case latest.Seq > afterSeq:
		return true, nil
	case afterSeq > latest.Seq:
		return false, &InputError{Field: "after_seq", Reason: "invalid"}
	}

	posted, err := tx.PostedDay(c.today)
	if err != nil {
		return false, err
	}
	if daySettled(posted, latest) {
		return false, ErrDaySettled
	}
	if latest.Kind == KindBonus {
		// One bonus spin per date: the database holds only one.
		return false, ErrNoBonus
	}
	if err := tx.SpendBonus(); err != nil {
		return false, err
	}

	// Draw from the wheel snapshot of today's spin, leaving out categories
	// that have since been removed for good (a spin must point at a row).
	ids := make([]string, len(daily.Wheel))
	for i, e := range daily.Wheel {
		ids[i] = e.ID
	}
	exists, err := tx.ExistingCategoryIDs(ids)
	if err != nil {
		return false, err
	}
	available := make([]WheelEntry, 0, len(daily.Wheel))
	for _, e := range daily.Wheel {
		if exists[e.ID] {
			available = append(available, e)
		}
	}
	picked, err := PickOther(available, latest.CategoryID, s.rand)
	if err != nil {
		return false, err
	}
	chosen := available[picked]
	winner := -1
	for i, e := range daily.Wheel {
		if e.ID == chosen.ID {
			winner = i
			break
		}
	}

	window := WindowFor(c.today, c.loc)
	_, inserted, err := tx.InsertSpin(Spin{
		LocalDate:      c.today,
		TZName:         c.tzName,
		Kind:           KindBonus,
		Seq:            latest.Seq + 1,
		CategoryID:     chosen.ID,
		CategoryName:   chosen.Name,
		Wheel:          daily.Wheel,
		WinnerIndex:    winner,
		SpunAt:         c.now,
		WindowOpensAt:  window.OpensAt,
		WindowClosesAt: window.ClosesAt,
		Result:         ResultOpen,
	})
	if err != nil {
		return false, err
	}
	if !inserted {
		// Rolls back the spend above.
		return false, errors.New("spin: the bonus spin already exists")
	}
	return false, nil
}

// --- the wheel ---

// WheelView is a person's live wheel and its version.
type WheelView struct {
	Version    int
	Categories []Category
}

// Wheel returns the person's live wheel in order. A first call makes the
// starting wheel.
func (s *Service) Wheel(ctx context.Context, userID string) (WheelView, error) {
	var out WheelView
	err := s.run(ctx, userID, false, func(c *call) error {
		cats, err := c.tx.LiveCategories()
		if err != nil {
			return err
		}
		st, err := c.tx.State()
		if err != nil {
			return err
		}
		out = WheelView{Version: st.WheelVersion, Categories: cats}
		return nil
	})
	return out, err
}

// SaveWheel replaces the person's whole wheel with list, in order, if
// version is still the current one.
//
// The list is validated before anything is written (ValidateWheel); a stale
// version is ErrStale; an id that is not a canonical UUID, or not one of the
// person's live categories, is unknown_id and never reaches SQL. Live
// categories left out are archived; rows with no id are inserted. At most
// MaxCategoryRows rows are kept: at the cap, archived rows no spin refers to
// are deleted first, and then too_many_total is refused. After the write the
// live count is checked in the same transaction.
func (s *Service) SaveWheel(ctx context.Context, userID string, version int, list []WheelInput) (WheelView, error) {
	if err := ValidateWheel(list); err != nil {
		return WheelView{}, err
	}
	var out WheelView
	err := s.run(ctx, userID, false, func(c *call) error {
		tx := c.tx
		st, err := tx.State()
		if err != nil {
			return err
		}
		if st.WheelVersion != version {
			return ErrStale
		}
		live, err := tx.LiveCategories()
		if err != nil {
			return err
		}
		liveIDs := make(map[string]bool, len(live))
		for _, cat := range live {
			liveIDs[cat.ID] = true
		}

		rows := make([]WheelRow, len(list))
		kept := make(map[string]bool, len(list))
		newRows := 0
		for i, e := range list {
			row := WheelRow{Name: CleanName(e.Name), Weight: e.Weight, Position: i}
			if e.ID != nil {
				id, ok := canonicalUUID(*e.ID)
				if !ok || !liveIDs[id] {
					return &WheelError{Reason: ReasonUnknownID, Index: i}
				}
				if kept[id] {
					return &WheelError{Reason: ReasonDuplicateID, Index: i}
				}
				kept[id] = true
				row.ID = id
			} else {
				newRows++
			}
			rows[i] = row
		}
		var archive []string
		for _, cat := range live {
			if !kept[cat.ID] {
				archive = append(archive, cat.ID)
			}
		}

		total, err := tx.CategoryCount()
		if err != nil {
			return err
		}
		if total+newRows > MaxCategoryRows {
			removed, err := tx.DeleteUnusedArchived()
			if err != nil {
				return err
			}
			total -= removed
			if total+newRows > MaxCategoryRows {
				return &WheelError{Reason: ReasonTooManyTotal, Index: -1}
			}
		}

		if err := tx.ApplyWheel(WheelChange{Rows: rows, Archive: archive}, c.now); err != nil {
			return err
		}
		cats, err := tx.LiveCategories()
		if err != nil {
			return err
		}
		switch {
		case len(cats) < MinWheelEntries:
			return &WheelError{Reason: ReasonTooFew, Index: -1}
		case len(cats) > MaxWheelEntries:
			return &WheelError{Reason: ReasonTooMany, Index: -1}
		}
		next, err := tx.BumpWheelVersion()
		if err != nil {
			return err
		}
		out = WheelView{Version: next, Categories: cats}
		return nil
	})
	return out, err
}

// canonicalUUID returns the lower-case 36 character form of a UUID string,
// and false for anything that is not exactly that shape.
func canonicalUUID(s string) (string, bool) {
	if len(s) != 36 {
		return "", false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return "", false
	}
	return id.String(), true
}

// --- posted days and the weekly bonus ---

// PostedResult says what marking a day did.
type PostedResult struct {
	// Inserted is false when the date already had a posted day.
	Inserted bool
	// Late is true when the post came at or after the date's close.
	Late bool
	// WeekCount is how many days of the date's week are now posted.
	WeekCount int
	// GoalMet is whether the date's week has reached its goal.
	GoalMet bool
	// BonusGranted is true when this call made a bonus spin held.
	BonusGranted bool
}

// MarkPosted counts a date as posted. The entry phase calls it (through
// markPostedIn, inside its own transaction) when a post is saved; the test
// tools call it with source "test". All under the person's lock.
func (s *Service) MarkPosted(ctx context.Context, userID string, date Date, source PostedSource) (PostedResult, error) {
	var out PostedResult
	err := s.run(ctx, userID, true, func(c *call) error {
		r, err := markPostedIn(c.tx, c.now, c.loc, c.userID, date, source)
		if err != nil {
			return err
		}
		out = r
		return nil
	})
	return out, err
}

// markPostedIn is the body of MarkPosted, taking the already open
// transaction so a post and its entry can commit together. It holds every
// rule:
//
//   - a date after the local date is refused (ErrNotOpen);
//   - late is true when now is at or after the date's close;
//   - for source "post", a date whose late window has ended is ErrClosed
//     (source "test" skips that check);
//   - the posted day is stored once; when new, the week is counted, and at
//     the goal a goal-week row is made if absent; when that row is new and
//     no bonus is held, a bonus spin is granted (and the row says so). A week
//     that has a row never pays again, and a week hit while one is held adds
//     none and still counts as paid.
func markPostedIn(tx Tx, now time.Time, loc *time.Location, userID string, date Date, source PostedSource) (PostedResult, error) {
	_ = userID // the transaction is already scoped to this person
	today := LocalDate(now, loc)
	if date.After(today) {
		return PostedResult{}, ErrNotOpen
	}
	window := WindowFor(date, loc)
	late := !now.Before(window.ClosesAt)
	if source == SourcePost && !now.Before(window.LateUntil) {
		return PostedResult{}, ErrClosed
	}

	inserted, err := tx.InsertPostedDay(PostedDay{Date: date, Source: source, Late: late})
	if err != nil {
		return PostedResult{}, err
	}
	res := PostedResult{Inserted: inserted, Late: late}
	if !inserted {
		existing, err := tx.PostedDay(date)
		if err != nil {
			return PostedResult{}, err
		}
		if existing != nil {
			res.Late = existing.Late
		}
	}

	weekStart := WeekStart(date)
	days, err := tx.PostedDays(weekStart, weekStart.AddDays(daysInWeek-1))
	if err != nil {
		return PostedResult{}, err
	}
	st, err := tx.State()
	if err != nil {
		return PostedResult{}, err
	}
	res.WeekCount = len(days)
	res.GoalMet = len(days) >= st.WeeklyGoal
	if !inserted || !res.GoalMet {
		return res, nil
	}

	grant := st.BonusHeld == 0
	made, err := tx.InsertGoalWeek(GoalWeek{WeekStart: weekStart, Goal: st.WeeklyGoal, MetAt: now, BonusGranted: grant})
	if err != nil {
		return PostedResult{}, err
	}
	if made && grant {
		granted, err := tx.GrantBonus()
		if err != nil {
			return PostedResult{}, err
		}
		res.BonusGranted = granted
	}
	return res, nil
}

// --- test tools ---

// TestDaysResult is the week and bonus after pretend days were marked.
type TestDaysResult struct {
	Week      WeekView
	BonusHeld bool
}

// MarkTestDays marks 1 to 7 pretend posted days (source "test"), none after
// the person's local today and none older than 60 days, all in one
// transaction. It exists for the test-only route.
func (s *Service) MarkTestDays(ctx context.Context, userID string, dates []Date) (TestDaysResult, error) {
	if len(dates) == 0 {
		return TestDaysResult{}, &InputError{Field: "dates", Reason: "required"}
	}
	if len(dates) > maxTestDays {
		return TestDaysResult{}, &InputError{Field: "dates", Reason: "invalid"}
	}
	var out TestDaysResult
	err := s.run(ctx, userID, true, func(c *call) error {
		for _, d := range dates {
			if d.After(c.today) {
				return &InputError{Field: "dates", Reason: "future"}
			}
			if c.today.DaysSince(d) > maxTestDayAgeDays {
				return &InputError{Field: "dates", Reason: "invalid"}
			}
		}
		for _, d := range dates {
			if _, err := markPostedIn(c.tx, c.now, c.loc, c.userID, d, SourceTest); err != nil {
				return err
			}
		}
		t, err := s.todayIn(c)
		if err != nil {
			return err
		}
		out = TestDaysResult{Week: t.Week, BonusHeld: t.Bonus.Held}
		return nil
	})
	return out, err
}

// ResetTestData clears the person's entries (their photo rows go with them),
// then ALL their posted days of any source, then their spins and goal weeks,
// and sets the held bonus spin to none. The wheel, the zone and the seeded_*
// values stay. The service knows only keys, so the photo files stay in
// storage.
func (s *Service) ResetTestData(ctx context.Context, userID string) error {
	return s.run(ctx, userID, false, func(c *call) error {
		return c.tx.ClearTestData()
	})
}
