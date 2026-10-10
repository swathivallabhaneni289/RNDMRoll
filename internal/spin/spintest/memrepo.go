// Package spintest holds the in-memory spin repository and the service suite
// that both it and the Postgres repository must pass. It imports package
// spin, so it can only be used from tests outside that package.
package spintest

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// MemRepo is an in-memory spin.Repository with one lock per person. A
// transaction works on a copy of the person's data and replaces it only when
// the callback returns nil, so a callback that fails leaves nothing behind,
// as a rolled-back Postgres transaction would.
type MemRepo struct {
	mu    sync.Mutex
	users map[string]*memUser
}

// NewMemRepo returns an empty repository.
func NewMemRepo() *MemRepo {
	return &MemRepo{users: make(map[string]*memUser)}
}

type memUser struct {
	mu   sync.Mutex // the person's lock
	data *memData   // committed data; nil until the state row is made
}

type memData struct {
	state   spin.UserState
	zone    string
	hasZone bool
	cats    []spin.Category // every row, archived ones included, in creation order
	spins   []spin.Spin
	posted  map[spin.Date]spin.PostedDay
	goals   map[spin.Date]spin.GoalWeek
}

func (d *memData) clone() *memData {
	c := *d
	c.cats = make([]spin.Category, len(d.cats))
	for i, cat := range d.cats {
		if cat.ArchivedAt != nil {
			at := *cat.ArchivedAt
			cat.ArchivedAt = &at
		}
		c.cats[i] = cat
	}
	c.spins = make([]spin.Spin, len(d.spins))
	for i, s := range d.spins {
		s.Wheel = append([]spin.WheelEntry(nil), s.Wheel...)
		c.spins[i] = s
	}
	c.posted = make(map[spin.Date]spin.PostedDay, len(d.posted))
	for k, v := range d.posted {
		c.posted[k] = v
	}
	c.goals = make(map[spin.Date]spin.GoalWeek, len(d.goals))
	for k, v := range d.goals {
		c.goals[k] = v
	}
	return &c
}

func (r *MemRepo) user(id string) *memUser {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		u = &memUser{}
		r.users[id] = u
	}
	return u
}

// Zone reads the committed zone name.
func (r *MemRepo) Zone(_ context.Context, userID string) (string, bool, error) {
	u := r.user(userID)
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.data == nil || !u.data.hasZone {
		return "", false, nil
	}
	return u.data.zone, true, nil
}

// InUserTx implements spin.Repository.
func (r *MemRepo) InUserTx(_ context.Context, userID string, seed spin.Seed, fn func(spin.Tx) error) error {
	u := r.user(userID)
	u.mu.Lock()
	defer u.mu.Unlock()

	var work *memData
	if u.data == nil {
		work = &memData{
			state: spin.UserState{
				SeededAt:        seed.At,
				SeededLocalDate: seed.LocalDate,
				SeededTZ:        seed.TZName,
				WeeklyGoal:      seed.Goal,
				WheelVersion:    1,
			},
			posted: make(map[spin.Date]spin.PostedDay),
			goals:  make(map[spin.Date]spin.GoalWeek),
		}
		for i, c := range spin.DefaultWheel() {
			work.cats = append(work.cats, spin.Category{ID: uuid.NewString(), Name: c.Name, Weight: c.Weight, Position: i})
		}
	} else {
		work = u.data.clone()
	}
	if err := fn(&memTx{d: work}); err != nil {
		return err
	}
	u.data = work
	return nil
}

// State reads a person's committed state row, for tests.
func (r *MemRepo) State(userID string) (spin.UserState, bool) {
	u := r.user(userID)
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.data == nil {
		return spin.UserState{}, false
	}
	return u.data.state, true
}

// Counts returns the number of spin rows and category rows a person has.
func (r *MemRepo) Counts(userID string) (spins, categories int) {
	u := r.user(userID)
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.data == nil {
		return 0, 0
	}
	return len(u.data.spins), len(u.data.cats)
}

// SetSpinResult sets the stored result of one spin, as the entry phase will.
func (r *MemRepo) SetSpinResult(userID, spinID string, result spin.SpinResult) error {
	u := r.user(userID)
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.data != nil {
		for i := range u.data.spins {
			if u.data.spins[i].ID == spinID {
				u.data.spins[i].Result = result
				return nil
			}
		}
	}
	return errors.New("spintest: no such spin")
}

// ForceZone stores a zone name without checking it, so a test can leave a
// name that no longer loads.
func (r *MemRepo) ForceZone(userID, name string) error {
	u := r.user(userID)
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.data == nil {
		return errors.New("spintest: no state row yet")
	}
	u.data.zone, u.data.hasZone = name, true
	return nil
}

// FillArchived adds archived categories directly: `unreferenced` with no
// spin, and `referenced` each with a daily spin on a distinct past date.
func (r *MemRepo) FillArchived(userID string, unreferenced, referenced int) error {
	u := r.user(userID)
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.data == nil {
		return errors.New("spintest: no state row yet")
	}
	at := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < unreferenced+referenced; i++ {
		id := uuid.NewString()
		u.data.cats = append(u.data.cats, spin.Category{ID: id, Name: "Filler", Weight: 1, Position: 1000 + i, ArchivedAt: &at})
		if i >= unreferenced {
			day := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i-unreferenced)
			u.data.spins = append(u.data.spins, spin.Spin{
				ID: uuid.NewString(), LocalDate: spin.DateOf(day), TZName: "UTC", Kind: spin.KindDaily,
				CategoryID: id, CategoryName: "Filler", Wheel: []spin.WheelEntry{{ID: id, Name: "Filler", Weight: 1}},
				SpunAt: day, WindowOpensAt: day, WindowClosesAt: day, Result: spin.ResultOpen,
			})
		}
	}
	return nil
}

// memTx is one person's transaction over a working copy of their data.
type memTx struct {
	d *memData
}

func (t *memTx) State() (spin.UserState, error) { return t.d.state, nil }

func (t *memTx) Zone() (string, bool, error) { return t.d.zone, t.d.hasZone, nil }

func (t *memTx) SetZone(name string) error {
	t.d.zone, t.d.hasZone = name, true
	return nil
}

func (t *memTx) LiveCategories() ([]spin.Category, error) {
	var live []spin.Category
	for _, c := range t.d.cats {
		if c.ArchivedAt == nil {
			live = append(live, c)
		}
	}
	sort.SliceStable(live, func(i, j int) bool { return live[i].Position < live[j].Position })
	return live, nil
}

func (t *memTx) CategoryCount() (int, error) { return len(t.d.cats), nil }

func (t *memTx) ExistingCategoryIDs(ids []string) (map[string]bool, error) {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	found := make(map[string]bool)
	for _, c := range t.d.cats {
		if want[c.ID] {
			found[c.ID] = true
		}
	}
	return found, nil
}

func (t *memTx) ApplyWheel(change spin.WheelChange, now time.Time) error {
	for _, row := range change.Rows {
		if row.ID == "" {
			t.d.cats = append(t.d.cats, spin.Category{ID: uuid.NewString(), Name: row.Name, Weight: row.Weight, Position: row.Position})
			continue
		}
		updated := false
		for i := range t.d.cats {
			if t.d.cats[i].ID == row.ID && t.d.cats[i].ArchivedAt == nil {
				t.d.cats[i].Name, t.d.cats[i].Weight, t.d.cats[i].Position = row.Name, row.Weight, row.Position
				updated = true
			}
		}
		if !updated {
			return errors.New("spintest: wheel row names a category that is not live")
		}
	}
	archive := make(map[string]bool, len(change.Archive))
	for _, id := range change.Archive {
		archive[id] = true
	}
	for i := range t.d.cats {
		if archive[t.d.cats[i].ID] && t.d.cats[i].ArchivedAt == nil {
			at := now
			t.d.cats[i].ArchivedAt = &at
		}
	}
	return nil
}

func (t *memTx) DeleteUnusedArchived() (int, error) {
	used := make(map[string]bool, len(t.d.spins))
	for _, s := range t.d.spins {
		used[s.CategoryID] = true
	}
	kept := t.d.cats[:0:0]
	removed := 0
	for _, c := range t.d.cats {
		if c.ArchivedAt != nil && !used[c.ID] {
			removed++
			continue
		}
		kept = append(kept, c)
	}
	t.d.cats = kept
	return removed, nil
}

func (t *memTx) BumpWheelVersion() (int, error) {
	t.d.state.WheelVersion++
	return t.d.state.WheelVersion, nil
}

func (t *memTx) DailySpinOfDate(d spin.Date) (*spin.Spin, error) {
	for i := range t.d.spins {
		if s := t.d.spins[i]; s.LocalDate == d && s.Seq == 0 {
			return &s, nil
		}
	}
	return nil, nil
}

func (t *memTx) LatestSpinOfDate(d spin.Date) (*spin.Spin, error) {
	var best *spin.Spin
	for i := range t.d.spins {
		s := t.d.spins[i]
		if s.LocalDate == d && (best == nil || s.Seq > best.Seq) {
			c := s
			best = &c
		}
	}
	return best, nil
}

func (t *memTx) LastDailySpunAt() (*time.Time, error) {
	var last *time.Time
	for _, s := range t.d.spins {
		if s.Kind == spin.KindDaily && (last == nil || s.SpunAt.After(*last)) {
			at := s.SpunAt
			last = &at
		}
	}
	return last, nil
}

func (t *memTx) InsertSpin(s spin.Spin) (spin.Spin, bool, error) {
	categoryExists := false
	for _, c := range t.d.cats {
		if c.ID == s.CategoryID {
			categoryExists = true
		}
	}
	if !categoryExists {
		return spin.Spin{}, false, errors.New("spintest: spin names a category that does not exist")
	}
	for _, existing := range t.d.spins {
		if existing.LocalDate == s.LocalDate && existing.Seq == s.Seq {
			return existing, false, nil
		}
	}
	if s.Kind == spin.KindBonus {
		for _, existing := range t.d.spins {
			if existing.LocalDate == s.LocalDate && existing.Kind == spin.KindBonus {
				return spin.Spin{}, false, errors.New("spintest: a bonus spin already exists for that date")
			}
		}
	}
	s.ID = uuid.NewString()
	s.Result = spin.ResultOpen
	s.CreatedAt = s.SpunAt
	s.Wheel = append([]spin.WheelEntry(nil), s.Wheel...)
	t.d.spins = append(t.d.spins, s)
	return s, true, nil
}

func (t *memTx) SpendBonus() error {
	if t.d.state.BonusHeld != 1 {
		return spin.ErrNoBonus
	}
	t.d.state.BonusHeld = 0
	return nil
}

func (t *memTx) GrantBonus() (bool, error) {
	if t.d.state.BonusHeld != 0 {
		return false, nil
	}
	t.d.state.BonusHeld = 1
	return true, nil
}

func (t *memTx) PostedDay(d spin.Date) (*spin.PostedDay, error) {
	if p, ok := t.d.posted[d]; ok {
		return &p, nil
	}
	return nil, nil
}

func (t *memTx) PostedDays(from, to spin.Date) ([]spin.PostedDay, error) {
	var out []spin.PostedDay
	for d, p := range t.d.posted {
		if !d.Before(from) && !d.After(to) {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}

func (t *memTx) InsertPostedDay(p spin.PostedDay) (bool, error) {
	if _, ok := t.d.posted[p.Date]; ok {
		return false, nil
	}
	t.d.posted[p.Date] = p
	return true, nil
}

func (t *memTx) InsertGoalWeek(g spin.GoalWeek) (bool, error) {
	if _, ok := t.d.goals[g.WeekStart]; ok {
		return false, nil
	}
	t.d.goals[g.WeekStart] = g
	return true, nil
}

func (t *memTx) ClearTestData() error {
	t.d.spins = nil
	for d, p := range t.d.posted {
		if p.Source == spin.SourceTest {
			delete(t.d.posted, d)
		}
	}
	t.d.goals = make(map[spin.Date]spin.GoalWeek)
	t.d.state.BonusHeld = 0
	return nil
}

var (
	_ spin.Repository = (*MemRepo)(nil)
	_ spin.Tx         = (*memTx)(nil)
)
