package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
)

// BoardRepo is board.Repository over the pool. It reads storage keys only
// and never builds a link or a label.
type BoardRepo struct {
	pool *pgxpool.Pool
}

// NewBoardRepo builds a BoardRepo backed by pool.
func NewBoardRepo(pool *pgxpool.Pool) *BoardRepo {
	return &BoardRepo{pool: pool}
}

var _ board.Repository = (*BoardRepo)(nil)

// boardEntrySelect reads an entry with its owner, its spin's category and
// zone, and the owner's late flag.
const boardEntrySelect = `
	select e.id, ` + personColumns + `, e.local_date::text, s.category_id::text, s.category_name, s.tz_name,
	       e.title, e.stars, e.reaction, e.comment, coalesce(p.late, false), e.created_at,
	       e.archived_at is not null, e.comments_off
	from entries e
	join spins s on s.id = e.spin_id and s.user_id = e.user_id
	join users u on u.id = e.user_id
	left join posted_days p on p.user_id = e.user_id and p.local_date = e.local_date
`

func scanBoardEntry(row interface{ Scan(...any) error }) (board.EntryRow, error) {
	var e board.EntryRow
	err := row.Scan(&e.ID, &e.Owner.ID, &e.Owner.Name, &e.Owner.Username, &e.Owner.PhotoURL,
		&e.LocalDate, &e.CategoryID, &e.CategoryName, &e.TZName,
		&e.Title, &e.Stars, &e.Reaction, &e.Note, &e.Late, &e.CreatedAt, &e.Archived, &e.CommentsOff)
	e.CreatedAt = e.CreatedAt.UTC()
	e.Photos = []board.PhotoRow{}
	return e, err
}

// photosOf fills each row's photos in position order with one query.
func (r *BoardRepo) photosOf(ctx context.Context, rows []board.EntryRow) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(rows))
	at := make(map[uuid.UUID]int, len(rows))
	for i, e := range rows {
		ids[i] = e.ID
		at[e.ID] = i
	}
	prows, err := r.pool.Query(ctx, `
		select entry_id, id::text, storage_key, position from entry_photos
		where entry_id = any($1::uuid[]) order by entry_id, position`, ids)
	if err != nil {
		return err
	}
	defer prows.Close()
	for prows.Next() {
		var entryID uuid.UUID
		var p board.PhotoRow
		if err := prows.Scan(&entryID, &p.ID, &p.Key, &p.Position); err != nil {
			return err
		}
		i := at[entryID]
		rows[i].Photos = append(rows[i].Photos, p)
	}
	return prows.Err()
}

// EntryByID implements board.Repository.
func (r *BoardRepo) EntryByID(ctx context.Context, id uuid.UUID) (board.EntryRow, error) {
	e, err := scanBoardEntry(r.pool.QueryRow(ctx, boardEntrySelect+` where e.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return board.EntryRow{}, board.ErrNotFound
	}
	if err != nil {
		return board.EntryRow{}, err
	}
	rows := []board.EntryRow{e}
	if err := r.photosOf(ctx, rows); err != nil {
		return board.EntryRow{}, err
	}
	return rows[0], nil
}

// PostedDates implements board.Repository.
func (r *BoardRepo) PostedDates(ctx context.Context, viewer uuid.UUID, upTo string, before *string, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		select local_date::text from posted_days
		where user_id = $1 and local_date <= $2::date and ($3::date is null or local_date < $3::date)
		order by local_date desc
		limit $4`, viewer, upTo, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// notMuted is the where fragment "the viewer $1 has not muted the person
// whose id is col": a mute takes their posts off the Board (and its counts)
// and nothing else.
func notMuted(col string) string {
	return `not exists (select 1 from mutes m where m.muter_id = $1 and m.muted_id = ` + col + `)`
}

// visibleOwner is the where fragment of the list rule for entry alias e and
// viewer $1: the viewer's own, or a person the viewer follows (accepted)
// with no block either way. Archived entries are never listed.
var visibleOwner = `e.archived_at is null and (e.user_id = $1 or (
		exists (select 1 from follows f where f.follower_id = $1 and f.followee_id = e.user_id and f.status = 'accepted')
		and not ` + blockedEither("$1::uuid", "e.user_id") + `))`

// Cards implements board.Repository. The dates are the viewer's posted
// dates, which is the rest of the rule.
func (r *BoardRepo) Cards(ctx context.Context, viewer uuid.UUID, dates []string) ([]board.EntryRow, error) {
	rows, err := r.pool.Query(ctx, boardEntrySelect+`
		where e.local_date = any($2::date[]) and `+visibleOwner+` and `+notMuted("e.user_id")+`
		order by e.local_date desc, e.created_at desc, e.id desc`, viewer, dates)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []board.EntryRow{}
	for rows.Next() {
		e, err := scanBoardEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := r.photosOf(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// CommentCounts implements board.Repository.
func (r *BoardRepo) CommentCounts(ctx context.Context, viewer uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	out := map[uuid.UUID]int{}
	if len(entryIDs) == 0 {
		return out, nil
	}
	// The same comments as the post page: a reply counts only while its
	// top-level comment is shown (board.Thread).
	rows, err := r.pool.Query(ctx, `
		select c.entry_id, count(*) from post_comments c
		left join post_comments p on p.id = c.parent_id
		where c.entry_id = any($2::uuid[]) and not `+blockedEither("$1::uuid", "c.user_id")+`
		  and (c.parent_id is null or not `+blockedEither("$1::uuid", "p.user_id")+`)
		group by c.entry_id`, viewer, entryIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// MyReactions implements board.Repository.
func (r *BoardRepo) MyReactions(ctx context.Context, viewer uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(entryIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		select entry_id, reaction from post_reactions where user_id = $1 and entry_id = any($2::uuid[])`, viewer, entryIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// followedClause is the where fragment for "u is someone viewer $1 follows
// (accepted) with no block either way".
var followedClause = `exists (select 1 from follows f where f.follower_id = $1 and f.followee_id = u.id and f.status = 'accepted')
		and not ` + blockedEither("$1::uuid", "u.id")

// PostedCount implements board.Repository.
func (r *BoardRepo) PostedCount(ctx context.Context, viewer uuid.UUID, date string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		select count(*) from posted_days p join users u on u.id = p.user_id
		where p.local_date = $2::date and `+followedClause+` and `+notMuted("u.id"), viewer, date).Scan(&n)
	return n, err
}

// NamesOnDay implements board.Repository.
func (r *BoardRepo) NamesOnDay(ctx context.Context, viewer uuid.UUID, date string, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		select coalesce(u.name, '') from entries e join users u on u.id = e.user_id
		where e.local_date = $2::date and e.archived_at is null and `+followedClause+` and `+notMuted("u.id")+`
		order by e.created_at, e.id
		limit $3`, viewer, date, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// Comments implements board.Repository.
func (r *BoardRepo) Comments(ctx context.Context, viewer, entryID uuid.UUID) ([]board.CommentRow, error) {
	rows, err := r.pool.Query(ctx, `
		select c.id, c.entry_id, `+personColumns+`, e.user_id, c.body, c.created_at, c.parent_id
		from post_comments c join users u on u.id = c.user_id join entries e on e.id = c.entry_id
		where c.entry_id = $2 and not `+blockedEither("$1::uuid", "c.user_id")+`
		order by c.created_at, c.id`, viewer, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []board.CommentRow{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanComment(row interface{ Scan(...any) error }) (board.CommentRow, error) {
	var c board.CommentRow
	err := row.Scan(&c.ID, &c.EntryID, &c.Author.ID, &c.Author.Name, &c.Author.Username, &c.Author.PhotoURL,
		&c.EntryOwner, &c.Body, &c.CreatedAt, &c.ParentID)
	c.CreatedAt = c.CreatedAt.UTC()
	return c, err
}

// Received implements board.Repository.
func (r *BoardRepo) Received(ctx context.Context, owner, entryID uuid.UUID) ([]board.Received, error) {
	rows, err := r.pool.Query(ctx, `
		select `+personColumns+`, pr.reaction, pr.updated_at
		from post_reactions pr join users u on u.id = pr.user_id
		where pr.entry_id = $2 and pr.user_id <> $1 and not `+blockedEither("$1::uuid", "pr.user_id")+`
		order by pr.updated_at desc, pr.user_id`, owner, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []board.Received{}
	for rows.Next() {
		var rc board.Received
		if err := rows.Scan(&rc.Person.ID, &rc.Person.Name, &rc.Person.Username, &rc.Person.PhotoURL, &rc.Reaction, &rc.At); err != nil {
			return nil, err
		}
		rc.At = rc.At.UTC()
		out = append(out, rc)
	}
	return out, rows.Err()
}

// SetReaction implements board.Repository. A change keeps the first
// created_at and moves updated_at.
func (r *BoardRepo) SetReaction(ctx context.Context, entryID, userID uuid.UUID, reaction string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into post_reactions (entry_id, user_id, reaction, created_at, updated_at)
		values ($1, $2, $3, $4, $4)
		on conflict (entry_id, user_id) do update set reaction = excluded.reaction, updated_at = excluded.updated_at
		where post_reactions.reaction <> excluded.reaction`, entryID, userID, reaction, at)
	return err
}

// ClearReaction implements board.Repository.
func (r *BoardRepo) ClearReaction(ctx context.Context, entryID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `delete from post_reactions where entry_id = $1 and user_id = $2`, entryID, userID)
	return err
}

// InsertComment implements board.Repository.
func (r *BoardRepo) InsertComment(ctx context.Context, entryID, userID uuid.UUID, body string, at time.Time, parentID *uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		insert into post_comments (entry_id, user_id, body, created_at, parent_id) values ($1, $2, $3, $4, $5)
		returning id`, entryID, userID, body, at, parentID).Scan(&id)
	if err != nil && isForeignKeyViolation(err) {
		return uuid.UUID{}, board.ErrNotFound
	}
	return id, err
}

// isForeignKeyViolation reports a foreign key failure (the entry or the
// parent comment went away between the visibility read and the insert).
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// CommentByID implements board.Repository.
func (r *BoardRepo) CommentByID(ctx context.Context, id uuid.UUID) (board.CommentRow, error) {
	c, err := scanComment(r.pool.QueryRow(ctx, `
		select c.id, c.entry_id, `+personColumns+`, e.user_id, c.body, c.created_at, c.parent_id
		from post_comments c join users u on u.id = c.user_id join entries e on e.id = c.entry_id
		where c.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return board.CommentRow{}, board.ErrNotFound
	}
	return c, err
}

// DeleteComment implements board.Repository.
func (r *BoardRepo) DeleteComment(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `delete from post_comments where id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return board.ErrNotFound
	}
	return nil
}
