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
// zone, the owner's late flag, and what moderation says about it for the
// viewer, who is $1 in every statement that uses it: removed, under review,
// and reported by the viewer.
var boardEntrySelect = `
	select e.id, ` + personColumns + `, e.local_date::text, s.category_id::text, s.category_name, s.tz_name,
	       e.title, e.stars, e.reaction, e.comment, coalesce(p.late, false), e.created_at,
	       e.archived_at is not null, e.comments_off,
	       e.removed_at is not null, ` + postUnderReview("e") + `, ` + postReportedBy("$1::uuid", "e") + `
	from entries e
	join spins s on s.id = e.spin_id and s.user_id = e.user_id
	join users u on u.id = e.user_id
	left join posted_days p on p.user_id = e.user_id and p.local_date = e.local_date
`

func scanBoardEntry(row interface{ Scan(...any) error }) (board.EntryRow, error) {
	var e board.EntryRow
	err := row.Scan(&e.ID, &e.Owner.ID, &e.Owner.Name, &e.Owner.Username, &e.Owner.PhotoKey,
		&e.LocalDate, &e.CategoryID, &e.CategoryName, &e.TZName,
		&e.Title, &e.Stars, &e.Reaction, &e.Note, &e.Late, &e.CreatedAt, &e.Archived, &e.CommentsOff,
		&e.Removed, &e.UnderReview, &e.Reported)
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

// EntryFor implements board.Repository. Nothing is filtered here: the
// service decides with social.CanView, from the facts read with the row.
func (r *BoardRepo) EntryFor(ctx context.Context, viewer, id uuid.UUID) (board.EntryRow, error) {
	e, err := scanBoardEntry(r.pool.QueryRow(ctx, boardEntrySelect+` where e.id = $2`, viewer, id))
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
// viewer $1: social.CanView (postVisibleTo), and archived entries are never
// listed, the viewer's own included.
var visibleOwner = `e.archived_at is null and ` + postVisibleTo("$1::uuid", "e")

// Cards implements board.Repository. The dates are the viewer's posted
// dates; visibleOwner checks the posted day of someone else's post too, as
// the rule it shares with the other reads, at the cost of one key probe.
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
	// top-level comment is shown (board.Thread), so every rule on the comment
	// is checked on its parent too.
	rows, err := r.pool.Query(ctx, `
		select c.entry_id, count(*) from post_comments c
		join entries e on e.id = c.entry_id
		left join post_comments p on p.id = c.parent_id
		where c.entry_id = any($2::uuid[]) and not `+blockedEither("$1::uuid", "c.user_id")+` and `+shownComment("c", "e")+`
		  and `+commentShownTo("$1::uuid", "c")+`
		  and (c.parent_id is null or (not `+blockedEither("$1::uuid", "p.user_id")+` and `+shownComment("p", "e")+`
		    and `+commentShownTo("$1::uuid", "p")+`))
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

// NamesOnDay implements board.Repository. A post moderation keeps from the
// viewer gives no name; its posted day still counts in PostedCount, as a
// deleted post's does.
func (r *BoardRepo) NamesOnDay(ctx context.Context, viewer uuid.UUID, date string, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		select coalesce(u.name, '') from entries e join users u on u.id = e.user_id
		where e.local_date = $2::date and e.archived_at is null and `+followedClause+` and `+notMuted("u.id")+`
		  and `+postShownTo("$1::uuid", "e")+`
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

// Comments implements board.Repository. A comment under review is listed
// for its writer only, so its under_review is set on the viewer's own.
func (r *BoardRepo) Comments(ctx context.Context, viewer, entryID uuid.UUID) ([]board.CommentRow, error) {
	rows, err := r.pool.Query(ctx, `
		select `+commentColumns+`
		from post_comments c join users u on u.id = c.user_id join entries e on e.id = c.entry_id
		where c.entry_id = $2 and not `+blockedEither("$1::uuid", "c.user_id")+` and `+shownComment("c", "e")+`
		  and `+commentShownTo("$1::uuid", "c")+`
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

// commentColumns are the columns scanComment reads, for the comment alias c,
// its writer u and its entry e.
var commentColumns = `c.id, c.entry_id, ` + personColumns + `, e.user_id, c.body, c.created_at, c.parent_id,
	c.hidden_at is not null, c.photo_key, ` + commentUnderReview("c")

func scanComment(row interface{ Scan(...any) error }) (board.CommentRow, error) {
	var c board.CommentRow
	err := row.Scan(&c.ID, &c.EntryID, &c.Author.ID, &c.Author.Name, &c.Author.Username, &c.Author.PhotoKey,
		&c.EntryOwner, &c.Body, &c.CreatedAt, &c.ParentID, &c.Hidden, &c.PhotoKey, &c.UnderReview)
	c.CreatedAt = c.CreatedAt.UTC()
	return c, err
}

// likeShownTo is a where fragment true when the post's owner o sees the like
// under alias pr (a post_reactions row, whatever its name): it is not their
// own, and there is no block either way between them and its maker, whose
// account is not gone (blockedEither already reads goneAccount for both). It
// is the one rule for which likes an owner sees and counts, read by
// Received, DiaryReader.Social and NotificationsRepo.Likes, so the number
// beside the heart and the Likes list always agree.
func likeShownTo(o, pr string) string {
	return `(` + pr + `.user_id <> ` + o + ` and not ` + blockedEither(o, pr+".user_id") + `)`
}

// Received implements board.Repository.
func (r *BoardRepo) Received(ctx context.Context, owner, entryID uuid.UUID) ([]board.Received, error) {
	rows, err := r.pool.Query(ctx, `
		select `+personColumns+`, pr.reaction, pr.updated_at
		from post_reactions pr join users u on u.id = pr.user_id
		where pr.entry_id = $2 and `+likeShownTo("$1::uuid", "pr")+`
		order by pr.updated_at desc, pr.user_id`, owner, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []board.Received{}
	for rows.Next() {
		var rc board.Received
		if err := rows.Scan(&rc.Person.ID, &rc.Person.Name, &rc.Person.Username, &rc.Person.PhotoKey, &rc.Reaction, &rc.At); err != nil {
			return nil, err
		}
		rc.At = rc.At.UTC()
		out = append(out, rc)
	}
	return out, rows.Err()
}

// SetReaction implements board.Repository. A change keeps the first
// created_at and moves updated_at. A like that races the post's or the
// account's delete (the owner's delete, the removed-post purge or the
// account purge) is ErrNotFound, as a comment is, so it answers 404 and not
// 500.
func (r *BoardRepo) SetReaction(ctx context.Context, entryID, userID uuid.UUID, reaction string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into post_reactions (entry_id, user_id, reaction, created_at, updated_at)
		values ($1, $2, $3, $4, $4)
		on conflict (entry_id, user_id) do update set reaction = excluded.reaction, updated_at = excluded.updated_at
		where post_reactions.reaction <> excluded.reaction`, entryID, userID, reaction, at)
	if err != nil && isForeignKeyViolation(err) {
		return board.ErrNotFound
	}
	return err
}

// ClearReaction implements board.Repository.
func (r *BoardRepo) ClearReaction(ctx context.Context, entryID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `delete from post_reactions where entry_id = $1 and user_id = $2`, entryID, userID)
	return err
}

// commentPhotoIndex is the unique index that gives a photo file to one
// comment only (migration 0013).
const commentPhotoIndex = "post_comments_photo_key_idx"

// InsertComment implements board.Repository.
func (r *BoardRepo) InsertComment(ctx context.Context, entryID, userID uuid.UUID, body string, at time.Time, parentID *uuid.UUID, photoKey *string) (uuid.UUID, time.Time, bool, error) {
	var id uuid.UUID
	// A photo file already used by an entry, or by a second look as proof,
	// is refused like a reused one.
	if photoKey != nil {
		var used bool
		if err := r.pool.QueryRow(ctx, `select exists (select 1 from entry_photos where storage_key = $1) or `+proofKeyHeld("$1::text"),
			*photoKey).Scan(&used); err != nil {
			return uuid.UUID{}, time.Time{}, false, err
		}
		if used {
			return uuid.UUID{}, time.Time{}, false, &board.FieldProblem{Field: "photo", Reason: "invalid"}
		}
	}
	err := r.pool.QueryRow(ctx, `
		insert into post_comments (entry_id, user_id, body, created_at, parent_id, photo_key) values ($1, $2, $3, $4, $5, $6)
		returning id`, entryID, userID, body, at, parentID, photoKey).Scan(&id)
	if err != nil && isForeignKeyViolation(err) {
		return uuid.UUID{}, time.Time{}, false, board.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == commentPhotoIndex && photoKey != nil {
		return r.commentSentAgain(ctx, entryID, userID, body, parentID, *photoKey)
	}
	if err != nil {
		return uuid.UUID{}, time.Time{}, false, err
	}
	return id, at, true, nil
}

// commentSentAgain answers an insert whose photo file is already on a
// comment. The phone gives up on an answer after 10 seconds and keeps the
// photo, so a retry sends the same key: when the comment holding it has the
// same writer, post, thread and words, it is this comment, saved the first
// time, and it comes back with made false. It is looked up after the insert
// fails, not before, so two sends racing each other end the same way. Any
// other comment holding the file is a reuse: FieldProblem photo invalid.
func (r *BoardRepo) commentSentAgain(ctx context.Context, entryID, userID uuid.UUID, body string, parentID *uuid.UUID, photoKey string) (uuid.UUID, time.Time, bool, error) {
	var id uuid.UUID
	var at time.Time
	err := r.pool.QueryRow(ctx, `
		select id, created_at from post_comments
		where photo_key = $1 and user_id = $2 and entry_id = $3 and parent_id is not distinct from $4::uuid and body = $5`,
		photoKey, userID, entryID, parentID, body).Scan(&id, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, time.Time{}, false, &board.FieldProblem{Field: "photo", Reason: "invalid"}
	}
	if err != nil {
		return uuid.UUID{}, time.Time{}, false, err
	}
	return id, at.UTC(), false, nil
}

// isForeignKeyViolation reports a foreign key failure (the entry or the
// parent comment went away between the visibility read and the insert).
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// shownComment is the where fragment "viewer $1 may read comment alias c on
// entry alias e": not hidden, or the viewer wrote it, or owns the post.
func shownComment(c, e string) string {
	return `(` + c + `.hidden_at is null or ` + c + `.user_id = $1 or ` + e + `.user_id = $1)`
}

// CommentByID implements board.Repository. A removed comment, a reply under a
// removed comment and a comment on a removed post are not found: the writer's
// and the post owner's delete and hide must not reach what an admin may
// still bring back. The cleanup reads (deleteCommentRows) do not go through
// here.
func (r *BoardRepo) CommentByID(ctx context.Context, id uuid.UUID) (board.CommentRow, error) {
	c, err := scanComment(r.pool.QueryRow(ctx, `
		select `+commentColumns+`
		from post_comments c join users u on u.id = c.user_id join entries e on e.id = c.entry_id
		left join post_comments p on p.id = c.parent_id
		where c.id = $1 and c.removed_at is null and e.removed_at is null
		  and (c.parent_id is null or p.removed_at is null)`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return board.CommentRow{}, board.ErrNotFound
	}
	return c, err
}

// DeleteComment implements board.Repository.
func (r *BoardRepo) DeleteComment(ctx context.Context, id uuid.UUID) ([]string, error) {
	return deleteCommentRows(ctx, r.pool, id)
}

// deleteCommentRows deletes a comment and its replies and returns their
// photo keys, leaving out a file that a photo row or a second look also
// names, since that post still shows it or the admin may still look at it
// (the same rule as commentPhotoKeysOf). ErrNotFound
// when the comment was not there.
func deleteCommentRows(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) ([]string, error) {
	rows, err := pool.Query(ctx, `
		delete from post_comments c where c.id = $1 or c.parent_id = $1
		returning c.id = $1, coalesce(c.photo_key, ''),
		          exists (select 1 from entry_photos ep where ep.storage_key = c.photo_key) or `+proofKeyHeld("c.photo_key"), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := false
	keys := []string{}
	for rows.Next() {
		var self, onPost bool
		var key string
		if err := rows.Scan(&self, &key, &onPost); err != nil {
			return nil, err
		}
		found = found || self
		if key != "" && !onPost {
			keys = append(keys, key)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !found {
		return nil, board.ErrNotFound
	}
	return keys, nil
}

// SetCommentHidden implements board.Repository. A comment removed after
// CommentByID read it is left as it is.
func (r *BoardRepo) SetCommentHidden(ctx context.Context, id uuid.UUID, at *time.Time) error {
	tag, err := r.pool.Exec(ctx, `update post_comments set hidden_at = $2 where id = $1 and removed_at is null`, id, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return board.ErrNotFound
	}
	return nil
}
