package board

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// Repository is the board's storage. The list reads apply the same rule as
// social.CanView in SQL: the viewer's own entries, and the entries of people
// the viewer follows (accepted) with no block either way and not archived,
// on dates the viewer posted. Archived entries are left off every list,
// the owner's own included (the owner reads them on the post page).
type Repository interface {
	// EntryByID reads any entry (any owner). ErrNotFound when missing.
	EntryByID(ctx context.Context, id uuid.UUID) (EntryRow, error)
	// PostedDates are the viewer's posted dates, newest first, at most
	// upTo, strictly before `before` when set, at most limit.
	PostedDates(ctx context.Context, viewer uuid.UUID, upTo string, before *string, limit int) ([]string, error)
	// Cards are the visible entries on those dates, newest first.
	Cards(ctx context.Context, viewer uuid.UUID, dates []string) ([]EntryRow, error)
	// PersonEntries are owner's entries the viewer may see (the same rule,
	// on the viewer's posted days), newest first, after the entry `after`
	// when set, at most limit.
	PersonEntries(ctx context.Context, viewer, owner uuid.UUID, after *uuid.UUID, limit int) ([]EntryRow, error)
	// PostCount is how many posts owner has, archived ones left out.
	PostCount(ctx context.Context, owner uuid.UUID) (int, error)
	// CommentCounts and MyReactions are the viewer's view of entryIDs.
	// Comments by someone with a block with the viewer are not counted.
	CommentCounts(ctx context.Context, viewer uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]int, error)
	MyReactions(ctx context.Context, viewer uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]string, error)
	// PostedCount is how many people the viewer follows posted date.
	PostedCount(ctx context.Context, viewer uuid.UUID, date string) (int, error)
	// NamesOnDay are the names of people the viewer follows with a visible
	// entry on date, in posting order.
	NamesOnDay(ctx context.Context, viewer uuid.UUID, date string, limit int) ([]string, error)
	// Comments of an entry, oldest first (top-level comments and replies
	// alike, with ParentID set on a reply), without writers who have a block
	// with the viewer. Thread puts them in page order.
	Comments(ctx context.Context, viewer, entryID uuid.UUID) ([]CommentRow, error)
	// Received are the reactions on owner's entry, newest change first,
	// without people who have a block with the owner.
	Received(ctx context.Context, owner, entryID uuid.UUID) ([]Received, error)
	SetReaction(ctx context.Context, entryID, userID uuid.UUID, reaction string, at time.Time) error
	ClearReaction(ctx context.Context, entryID, userID uuid.UUID) error
	// InsertComment writes a comment; parentID is the top-level comment of a
	// reply, nil for a top-level comment. ErrNotFound when the entry or the
	// parent went away.
	InsertComment(ctx context.Context, entryID, userID uuid.UUID, body string, at time.Time, parentID *uuid.UUID) (uuid.UUID, error)
	// CommentByID reads a comment with its entry's owner. ErrNotFound when
	// missing.
	CommentByID(ctx context.Context, id uuid.UUID) (CommentRow, error)
	DeleteComment(ctx context.Context, id uuid.UUID) error
	// SetCommentHidden hides (at set) or shows (at nil) a comment.
	// ErrNotFound when it went away.
	SetCommentHidden(ctx context.Context, id uuid.UUID, at *time.Time) error
}

// Facts is the spin slice's view of the viewer (spin.Service).
type Facts interface {
	BoardFacts(ctx context.Context, userID string) (spin.BoardFacts, error)
	ViewerClock(ctx context.Context, userID string) (time.Time, *time.Location, error)
}

// Service is the board use cases.
type Service struct {
	repo     Repository
	social   *social.Service
	facts    Facts
	clk      clock.Clock
	photoURL func(string) string
}

// NewService builds a Service. photoURL turns a storage key into its link;
// it is called only for entries the viewer may see.
func NewService(repo Repository, soc *social.Service, facts Facts, clk clock.Clock, photoURL func(string) string) *Service {
	return &Service{repo: repo, social: soc, facts: facts, clk: clk, photoURL: photoURL}
}

// link builds a photo link only when the rule allows the viewer to see it.
func (s *Service) link(f social.Facts, key string) string {
	if !social.CanView(f) || s.photoURL == nil || key == "" {
		return ""
	}
	return s.photoURL(key)
}

// Board returns one page of the Board. limit is 1 to MaxDays.
func (s *Service) Board(ctx context.Context, viewer uuid.UUID, before *string, limit int) (Board, error) {
	bf, err := s.facts.BoardFacts(ctx, viewer.String())
	if err != nil {
		return Board{}, err
	}
	follows, err := s.social.FollowsAnyone(ctx, viewer)
	if err != nil {
		return Board{}, err
	}
	out := Board{
		State:    Decide(follows, bf.Posted, bf.Skipped),
		DayLabel: DayLabelToday,
		Days:     []Day{},
	}
	if bf.LateWindow {
		out.DayLabel = DayLabelLastNight
	}
	if out.State == StateNoFollows {
		return out, nil
	}
	dip := bf.DayInPlay.String()
	if out.State == StateLocked {
		n, err := s.repo.PostedCount(ctx, viewer, dip)
		if err != nil {
			return Board{}, err
		}
		names, err := s.repo.NamesOnDay(ctx, viewer, dip, MaxNames)
		if err != nil {
			return Board{}, err
		}
		first := make([]string, 0, len(names))
		for _, n := range names {
			first = append(first, FirstName(n))
		}
		out.Locked = &Locked{PostedCount: n, Names: first, Category: bf.CategoryName, Action: bf.Action}
	}

	dates, err := s.repo.PostedDates(ctx, viewer, dip, before, limit+1)
	if err != nil {
		return Board{}, err
	}
	if len(dates) > limit {
		dates = dates[:limit]
		last := dates[len(dates)-1]
		out.NextBefore = &last
	}
	if len(dates) == 0 {
		return out, nil
	}
	rows, err := s.repo.Cards(ctx, viewer, dates)
	if err != nil {
		return Board{}, err
	}
	cards, err := s.cardsOf(ctx, viewer, rows, bf.Zone)
	if err != nil {
		return Board{}, err
	}
	byDate := map[string][]Card{}
	for _, c := range cards {
		byDate[c.LocalDate] = append(byDate[c.LocalDate], c)
	}
	for _, d := range dates {
		list := byDate[d]
		if len(list) == 0 {
			continue
		}
		date, err := spin.ParseDate(d)
		if err != nil {
			return Board{}, err
		}
		out.Days = append(out.Days, Day{LocalDate: d, Label: DayLabel(date, bf.DayInPlay), Cards: list})
	}
	return out, nil
}

// cardsOf turns visible rows into cards with the viewer's counts.
func (s *Service) cardsOf(ctx context.Context, viewer uuid.UUID, rows []EntryRow, zone *time.Location) ([]Card, error) {
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	counts, err := s.repo.CommentCounts(ctx, viewer, ids)
	if err != nil {
		return nil, err
	}
	mine, err := s.repo.MyReactions(ctx, viewer, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Card, 0, len(rows))
	for _, r := range rows {
		// Every listed row passed the rule in SQL: the viewer's own, or a
		// followed person's on a date the viewer posted, with no block.
		f := social.Facts{IsOwner: r.Owner.ID == viewer, Follows: true, ViewerPostedDay: true}
		out = append(out, s.cardOf(r, f, viewer, counts[r.ID], mine, zone))
	}
	return out, nil
}

func (s *Service) cardOf(r EntryRow, f social.Facts, viewer uuid.UUID, comments int, mine map[uuid.UUID]string, zone *time.Location) Card {
	c := Card{
		EntryID:         r.ID,
		Author:          r.Owner,
		Mine:            r.Owner.ID == viewer,
		LocalDate:       r.LocalDate,
		CategoryID:      r.CategoryID,
		CategoryName:    r.CategoryName,
		Title:           r.Title,
		Stars:           r.Stars,
		Reaction:        r.Reaction,
		Late:            r.Late,
		PostedAt:        r.CreatedAt,
		PostedLocalTime: spin.PostedLocalTime(r.CreatedAt, r.TZName, zone),
		CommentCount:    comments,
	}
	if len(r.Photos) > 0 {
		c.PhotoURL = s.link(f, r.Photos[0].Key)
	}
	if name, ok := mine[r.ID]; ok {
		n := name
		c.MyReaction = &n
	}
	return c
}

// visible reads an entry and the facts for the viewer; a missing entry and
// one the viewer may not see are both ErrNotFound.
func (s *Service) visible(ctx context.Context, viewer, entryID uuid.UUID) (EntryRow, social.Facts, error) {
	row, err := s.repo.EntryByID(ctx, entryID)
	if err != nil {
		return EntryRow{}, social.Facts{}, err
	}
	f, err := s.social.ViewFacts(ctx, viewer, row.Owner.ID, row.LocalDate, row.Archived)
	if err != nil {
		return EntryRow{}, social.Facts{}, err
	}
	if !social.CanView(f) {
		return EntryRow{}, social.Facts{}, ErrNotFound
	}
	return row, f, nil
}

// Post returns the post page. The viewer needs a stored zone (the labels).
func (s *Service) Post(ctx context.Context, viewer, entryID uuid.UUID) (Post, error) {
	now, zone, err := s.facts.ViewerClock(ctx, viewer.String())
	if err != nil {
		return Post{}, err
	}
	row, f, err := s.visible(ctx, viewer, entryID)
	if err != nil {
		return Post{}, err
	}
	ids := []uuid.UUID{row.ID}
	mine, err := s.repo.MyReactions(ctx, viewer, ids)
	if err != nil {
		return Post{}, err
	}
	rows, err := s.visibleComments(ctx, viewer, row.ID)
	if err != nil {
		return Post{}, err
	}
	comments := make([]Comment, 0, len(rows))
	for _, c := range rows {
		comments = append(comments, s.commentOf(c, viewer, row.Owner.ID, now, zone))
	}
	p := Post{
		Card:        s.cardOf(row, f, viewer, len(comments), mine, zone),
		Note:        row.Note,
		Photos:      make([]Photo, 0, len(row.Photos)),
		Comments:    comments,
		CanReact:    social.CanReact(f),
		CanComment:  social.CanComment(f, row.CommentsOff),
		CommentsOff: row.CommentsOff,
	}
	for _, ph := range row.Photos {
		p.Photos = append(p.Photos, Photo{ID: ph.ID, URL: s.link(f, ph.Key), Position: ph.Position})
	}
	if f.IsOwner {
		if p.Received, err = s.repo.Received(ctx, viewer, row.ID); err != nil {
			return Post{}, err
		}
	}
	return p, nil
}

// visibleComments are the comments of an entry the viewer may see, in page
// order (Thread): each top-level comment followed by its replies.
func (s *Service) visibleComments(ctx context.Context, viewer, entryID uuid.UUID) ([]CommentRow, error) {
	rows, err := s.repo.Comments(ctx, viewer, entryID)
	if err != nil {
		return nil, err
	}
	return Thread(rows), nil
}

func (s *Service) commentOf(c CommentRow, viewer, owner uuid.UUID, now time.Time, zone *time.Location) Comment {
	return Comment{
		ID:        c.ID,
		Author:    c.Author,
		Text:      c.Body,
		At:        c.CreatedAt,
		AtLabel:   spin.ViewerLabel(c.CreatedAt, now, zone),
		CanDelete: social.CanDeleteComment(c.Author.ID == viewer, owner == viewer),
		CanHide:   owner == viewer && c.Author.ID != viewer,
		// The writer of a hidden comment is never told it is hidden.
		Hidden:   c.Hidden && owner == viewer,
		ParentID: c.ParentID,
	}
}

// SetReaction sets (a name) or clears (nil) the viewer's reaction. Order:
// the post must be visible, not the viewer's own, then the name valid.
func (s *Service) SetReaction(ctx context.Context, viewer, entryID uuid.UUID, reaction *string) (*string, error) {
	row, f, err := s.visible(ctx, viewer, entryID)
	if err != nil {
		return nil, err
	}
	if f.IsOwner {
		return nil, ErrOwnPost
	}
	if reaction == nil {
		return nil, s.repo.ClearReaction(ctx, row.ID, viewer)
	}
	if !spin.IsReaction(*reaction) {
		return nil, &FieldProblem{Field: "reaction", Reason: "invalid"}
	}
	if err := s.repo.SetReaction(ctx, row.ID, viewer, *reaction, s.clk.NowFor(viewer.String()).UTC()); err != nil {
		return nil, err
	}
	name := *reaction
	return &name, nil
}

// AddComment writes a comment, or a reply when parentID is set. Order: the
// post must be visible, comments on, the parent (when set) a comment on this
// post the viewer can see, then the text valid. A reply to a reply is stored
// under that reply's top-level comment (one level).
func (s *Service) AddComment(ctx context.Context, viewer, entryID uuid.UUID, text string, parentID *uuid.UUID) (Comment, error) {
	row, f, err := s.visible(ctx, viewer, entryID)
	if err != nil {
		return Comment{}, err
	}
	if row.CommentsOff {
		return Comment{}, ErrCommentsOff
	}
	if !social.CanComment(f, row.CommentsOff) {
		return Comment{}, ErrNotFound
	}
	var top *uuid.UUID
	if parentID != nil {
		if top, err = s.replyParent(ctx, viewer, row.ID, *parentID); err != nil {
			return Comment{}, err
		}
	}
	body, problem := CleanComment(text)
	if problem != nil {
		return Comment{}, problem
	}
	now, zone, err := s.facts.ViewerClock(ctx, viewer.String())
	if errors.Is(err, spin.ErrZoneMissing) {
		now, zone, err = s.clk.NowFor(viewer.String()), time.UTC, nil
	}
	if err != nil {
		return Comment{}, err
	}
	now = now.UTC()
	id, err := s.repo.InsertComment(ctx, row.ID, viewer, body, now, top)
	if err != nil {
		return Comment{}, err
	}
	me, err := s.social.Person(ctx, viewer)
	if err != nil {
		return Comment{}, err
	}
	return s.commentOf(CommentRow{ID: id, EntryID: row.ID, Author: me, EntryOwner: row.Owner.ID, Body: body, CreatedAt: now, ParentID: top},
		viewer, row.Owner.ID, now, zone), nil
}

// replyParent finds the comment a reply answers among the comments of the
// entry the viewer can see (the same list as the post page) and returns the
// top-level comment the reply is stored under. ErrNotFound when it is not
// there: missing, on another post, or hidden by a block.
func (s *Service) replyParent(ctx context.Context, viewer, entryID, parentID uuid.UUID) (*uuid.UUID, error) {
	rows, err := s.visibleComments(ctx, viewer, entryID)
	if err != nil {
		return nil, err
	}
	for _, c := range rows {
		if c.ID != parentID {
			continue
		}
		top := c.ID
		if c.ParentID != nil {
			top = *c.ParentID
		}
		return &top, nil
	}
	return nil, ErrNotFound
}

// DeleteComment deletes a comment for its writer or the post's owner.
// Deleting a top-level comment deletes its replies (the database cascades).
func (s *Service) DeleteComment(ctx context.Context, viewer, commentID uuid.UUID) error {
	c, err := s.repo.CommentByID(ctx, commentID)
	if err != nil {
		return err
	}
	if !social.CanDeleteComment(c.Author.ID == viewer, c.EntryOwner == viewer) {
		return ErrNotFound
	}
	return s.repo.DeleteComment(ctx, commentID)
}
