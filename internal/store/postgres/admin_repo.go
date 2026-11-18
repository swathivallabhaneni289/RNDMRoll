package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
)

// AdminRepo is admin.Repository over the pool. Unlike every other read, it
// sees disabled accounts, removed posts and comments and ignores blocks and
// reports: it is the admin's view.
type AdminRepo struct {
	pool *pgxpool.Pool
}

// NewAdminRepo builds an AdminRepo backed by pool.
func NewAdminRepo(pool *pgxpool.Pool) *AdminRepo {
	return &AdminRepo{pool: pool}
}

var _ admin.Repository = (*AdminRepo)(nil)

// A removed post or comment (migration 0020) is judged against one line,
// removalCutoff: now on the database's clock less admin.RemovalGrace. One
// removed after the line is inside the grace: the Removed tab lists it and
// Bring it back works. One removed at the line or before it is past the
// grace: only the purge touches it. insideRemovalGrace and pastRemovalGrace
// are each other's exact opposite over the same expression, as insideGrace
// and pastGrace are for accounts, and every rule reads now(), never a time
// sent from Go.
var (
	removalInterval = fmt.Sprintf("interval '%d seconds'", int64(admin.RemovalGrace/time.Second))
	removalCutoff   = "(now() - " + removalInterval + ")"
)

// insideRemovalGrace is a where fragment true when the removed_at column col
// is set and inside admin.RemovalGrace.
func insideRemovalGrace(col string) string { return "(" + col + " > " + removalCutoff + ")" }

// pastRemovalGrace is a where fragment true when col is set and past
// admin.RemovalGrace: the purge's line.
func pastRemovalGrace(col string) string { return "(" + col + " <= " + removalCutoff + ")" }

// daysLeft is the whole days, rounded up and never below 0, from now until
// the removed_at column col leaves admin.RemovalGrace.
func daysLeft(col string) string {
	return "greatest(ceil(extract(epoch from (" + col + " + " + removalInterval + " - now())) / 86400), 0)::int"
}

// adminPersonColumns are a person's columns as the admin page reads them (users
// as u), in scanAdminPerson's order. The email is never among them. The last
// is how many of their posts and comments were removed and not brought back:
// removals in the decision log with no restored row after them for the same
// item. "After" is by created_at, the database's clock, since the log's own
// at is the owner's clock and can stand still (a test clock).
const adminPersonColumns = `u.id, coalesce(u.username, ''), coalesce(u.name, ''), coalesce(u.bio, ''), coalesce(u.avatar_key, ''),
	u.created_at, u.disabled_at, u.deletion_requested_at,
	(select count(*) from moderation_events mrb where mrb.owner_id = u.id and mrb.action = 'removed'
	   and not exists (select 1 from moderation_events mra where mra.owner_id = u.id and mra.item_id = mrb.item_id
	                   and mra.action = 'restored' and mra.created_at > mrb.created_at))::int`

// scanAdminPerson reads adminPersonColumns into p, ahead of rest.
func scanAdminPerson(row pgx.Row, p *admin.Person, rest ...any) error {
	return row.Scan(append([]any{&p.ID, &p.Username, &p.Name, &p.Bio, &p.PhotoKey, &p.CreatedAt, &p.DisabledAt,
		&p.DeletionRequestedAt, &p.RemovedBefore}, rest...)...)
}

// allReports is every report in the three tables as the page reads it, with
// owner: the person reported, or whose post or comment it is. A post report
// carries the post as it is now (its title, its day, its text and every
// photo, in order), so the admin sees what was reported in full; a post that
// is gone has none of them. A removed post is still there, so it shows in
// full. hidden is set on an open report whose post or comment is hidden
// while under review now, by the same fragment as every read of the app,
// unless a removal keeps it from everyone (moderatedItem.removed). Callers
// filter it by owner or by resolved_at.
var allReports = `
	select 'person' as kind, ur.reported_id as owner, ur.reporter_id as id, coalesce(rp.username, '') as reporter,
	       ur.reason, ur.created_at, ur.resolved_at, coalesce(ur.resolution, '') as resolution,
	       null::uuid as entry_id, '' as title, '' as title_now, '' as local_date, '' as note,
	       array[]::text[] as photo_keys, null::uuid as comment_id, '' as body, '' as photo_key, false as hidden
	from user_reports ur join users rp on rp.id = ur.reporter_id
	union all
	select 'post', pr.owner_id, pr.id, coalesce(rp.username, ''), pr.reason, pr.created_at, pr.resolved_at,
	       coalesce(pr.resolution, ''), pr.entry_id, pr.title, coalesce(e.title, ''),
	       coalesce(e.local_date::text, ''), coalesce(e.comment, ''),
	       coalesce((select array_agg(ph.storage_key order by ph.position) from entry_photos ph
	                 where ph.entry_id = pr.entry_id), array[]::text[]),
	       null::uuid, '', '',
	       case when pr.resolved_at is null and e.id is not null and not ` + moderatedItems[admin.KindPost].removed("e") + `
	            then ` + postUnderReview("e") + ` else false end
	from post_reports pr join users rp on rp.id = pr.reporter_id
	left join entries e on e.id = pr.entry_id
	union all
	select 'comment', cr.owner_id, cr.id, coalesce(rp.username, ''), cr.reason, cr.created_at, cr.resolved_at,
	       coalesce(cr.resolution, ''), null::uuid, '', '', '', '', array[]::text[],
	       cr.comment_id, cr.body, coalesce(pc.photo_key, ''),
	       case when cr.resolved_at is null and pc.id is not null and not ` + moderatedItems[admin.KindComment].removed("pc") + `
	            then ` + commentUnderReview("pc") + ` else false end
	from comment_reports cr join users rp on rp.id = cr.reporter_id
	left join post_comments pc on pc.id = cr.comment_id`

// reportColumns are allReports' columns (as r) in scanReport's order.
const reportColumns = `r.kind, r.id, r.reporter, r.reason, r.created_at, r.resolved_at, r.resolution,
	r.entry_id, r.title, r.title_now, r.local_date, r.note, r.photo_keys, r.comment_id, r.body, r.photo_key, r.hidden`

// reportFields are rep's fields in reportColumns' order.
func reportFields(rep *admin.Report) []any {
	return []any{&rep.Kind, &rep.ID, &rep.Reporter, &rep.Reason, &rep.CreatedAt, &rep.ResolvedAt,
		&rep.Resolution, &rep.EntryID, &rep.Title, &rep.TitleNow, &rep.LocalDate, &rep.Note, &rep.PhotoKeys,
		&rep.CommentID, &rep.Body, &rep.PhotoKey, &rep.Hidden}
}

// OpenReports implements admin.Repository: every open report with whose it
// is, newest first, at most admin.OpenReportsLimit+1.
func (r *AdminRepo) OpenReports(ctx context.Context) ([]admin.OpenReport, error) {
	rows, err := r.pool.Query(ctx, `
		select `+adminPersonColumns+`, `+reportColumns+`
		from (`+allReports+`) r join users u on u.id = r.owner
		where r.resolved_at is null
		order by r.created_at desc
		limit $1`, admin.OpenReportsLimit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []admin.OpenReport{}
	for rows.Next() {
		var o admin.OpenReport
		if err := scanAdminPerson(rows, &o.Owner, reportFields(&o.Report)...); err != nil {
			return nil, err
		}
		o.Report.CreatedAt = o.Report.CreatedAt.UTC()
		out = append(out, o)
	}
	return out, rows.Err()
}

// DisabledPeople implements admin.Repository: disabled accounts, the most
// recently disabled first.
func (r *AdminRepo) DisabledPeople(ctx context.Context) ([]admin.Person, error) {
	rows, err := r.pool.Query(ctx, `
		select `+adminPersonColumns+`
		from users u where u.disabled_at is not null
		order by u.disabled_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []admin.Person{}
	for rows.Next() {
		var p admin.Person
		if err := scanAdminPerson(rows, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Person implements admin.Repository: a person with a report about them, a
// post or a comment (open or closed), a disabled account, or someone the
// decision log names (so a person whose reports went with their reporters'
// accounts still opens). Anyone else is admin.ErrNotFound, like an account
// that is not there, and the email is not read at all. The profile photo is
// its storage key (avatar_key); the page signs the link.
func (r *AdminRepo) Person(ctx context.Context, id uuid.UUID) (admin.Person, error) {
	var p admin.Person
	err := scanAdminPerson(r.pool.QueryRow(ctx, `
		select `+adminPersonColumns+`
		from users u
		where u.id = $1 and (u.disabled_at is not null
		       or exists (select 1 from user_reports where reported_id = u.id)
		       or exists (select 1 from post_reports where owner_id = u.id)
		       or exists (select 1 from comment_reports where owner_id = u.id)
		       or exists (select 1 from moderation_events where owner_id = u.id))`, id), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.Person{}, admin.ErrNotFound
	}
	return p, err
}

// Reports implements admin.Repository: every report about the person, open
// ones first, newest first within each (allReports says what each carries).
func (r *AdminRepo) Reports(ctx context.Context, person uuid.UUID) ([]admin.Report, error) {
	rows, err := r.pool.Query(ctx, `
		select `+reportColumns+`
		from (`+allReports+`) r
		where r.owner = $1
		order by (r.resolved_at is null) desc, r.created_at desc
		limit 200`, person)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []admin.Report{}
	for rows.Next() {
		var rep admin.Report
		if err := rows.Scan(reportFields(&rep)...); err != nil {
			return nil, err
		}
		rep.CreatedAt = rep.CreatedAt.UTC()
		out = append(out, rep)
	}
	return out, rows.Err()
}

// Events implements admin.Repository: the whole log about the person, by
// their own clock, the database's breaking a tie.
func (r *AdminRepo) Events(ctx context.Context, person uuid.UUID) ([]admin.Event, error) {
	rows, err := r.pool.Query(ctx, `
		select id, item, action, item_id, snapshot, reasons, at
		from moderation_events where owner_id = $1
		order by at desc, created_at desc`, person)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []admin.Event{}
	for rows.Next() {
		var e admin.Event
		if err := rows.Scan(&e.ID, &e.Item, &e.Action, &e.ItemID, &e.Snapshot, &e.Reasons, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Removed implements admin.Repository. Each item carries its newest removal
// in the log, for its reasons and its Ref; an item removed with no log row
// (straight in the database) has neither. A comment's days left run from
// the earliest removal of it, its post and the comment it answers (least
// skips the nulls): the purge of a post takes its comments, and of a
// comment its replies.
func (r *AdminRepo) Removed(ctx context.Context) ([]admin.RemovedItem, error) {
	rows, err := r.pool.Query(ctx, `
		select `+adminPersonColumns+`, x.kind, x.id, x.title, x.local_date, x.note, x.photo_keys, x.body, x.photo_key,
		       x.removed_at, `+daysLeft("x.goes_from")+`, ev.id, coalesce(ev.reasons, array[]::text[])
		from (
			select 'post' as kind, e.id, e.user_id, e.title, e.local_date::text as local_date,
			       coalesce(e.comment, '') as note,
			       coalesce((select array_agg(ph.storage_key order by ph.position) from entry_photos ph
			                 where ph.entry_id = e.id), array[]::text[]) as photo_keys,
			       '' as body, '' as photo_key, e.removed_at, e.removed_at as goes_from
			from entries e where `+insideRemovalGrace("e.removed_at")+`
			union all
			select 'comment', c.id, c.user_id, '', '', '', array[]::text[], c.body, coalesce(c.photo_key, ''), c.removed_at,
			       least(c.removed_at, (select mge.removed_at from entries mge where mge.id = c.entry_id),
			             (select mgp.removed_at from post_comments mgp where mgp.id = c.parent_id))
			from post_comments c where `+insideRemovalGrace("c.removed_at")+`
		) x
		join users u on u.id = x.user_id
		left join lateral (
			select me.id, me.reasons from moderation_events me
			where me.owner_id = x.user_id and me.item_id = x.id and me.action = 'removed'
			order by me.created_at desc limit 1) ev on true
		order by x.removed_at desc, x.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []admin.RemovedItem{}
	for rows.Next() {
		var it admin.RemovedItem
		var event *uuid.UUID
		if err := scanAdminPerson(rows, &it.Owner, &it.Kind, &it.ID, &it.Title, &it.LocalDate, &it.Note, &it.PhotoKeys,
			&it.Body, &it.PhotoKey, &it.RemovedAt, &it.DaysLeft, &event, &it.Reasons); err != nil {
			return nil, err
		}
		if event != nil {
			it.Ref = admin.Ref(*event)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// RemovedCount implements admin.Repository.
func (r *AdminRepo) RemovedCount(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `select
		(select count(*) from entries where `+insideRemovalGrace("removed_at")+`) +
		(select count(*) from post_comments where `+insideRemovalGrace("removed_at")+`)`).Scan(&n)
	return n, err
}

// moderatedItem is a post or a comment in SQL, for the actions on it: its
// table, its reports and their column naming it, the column the log keeps
// of it, its post's column (a post is its own), and its fragments over an
// alias.
type moderatedItem struct {
	table, reports, column, snapshot, entry string
	underReview                             func(alias string) string
	// removed is true when an admin's removal keeps it from everyone: it
	// was removed, or for a comment its post or the comment it answers was.
	removed func(alias string) string
	// aboveUnderReview is true when what it sits under is hidden while under
	// review, which keeps it from everyone but that one's owner: for a
	// comment, its post or the comment it answers. A post sits under nothing.
	aboveUnderReview func(alias string) string
}

var moderatedItems = map[string]moderatedItem{
	admin.KindPost: {table: "entries", reports: "post_reports", column: "entry_id", snapshot: "title", entry: "id",
		underReview:      postUnderReview,
		removed:          func(x string) string { return "(" + x + ".removed_at is not null)" },
		aboveUnderReview: func(string) string { return "false" }},
	admin.KindComment: {table: "post_comments", reports: "comment_reports", column: "comment_id", snapshot: "body", entry: "entry_id",
		underReview: commentUnderReview,
		removed: func(x string) string {
			return "(" + x + ".removed_at is not null or " + postOfCommentRemoved(x) + " or " + parentRemoved(x) + ")"
		},
		aboveUnderReview: func(x string) string {
			return "(exists (select 1 from entries mue where mue.id = " + x + ".entry_id and " + postUnderReview("mue") + ")" +
				" or exists (select 1 from post_comments mup where mup.id = " + x + ".parent_id and " + commentUnderReview("mup") + "))"
		}},
}

// postOfCommentRemoved is true when the post of the comment under alias x
// was removed, and parentRemoved when the comment it answers was: either
// keeps the comment from everyone too.
func postOfCommentRemoved(x string) string {
	return "exists (select 1 from entries mde where mde.id = " + x + ".entry_id and mde.removed_at is not null)"
}

func parentRemoved(x string) string {
	return "exists (select 1 from post_comments mdp where mdp.id = " + x + ".parent_id and mdp.removed_at is not null)"
}

// decision is one row of the decision log.
type decision struct {
	owner          uuid.UUID
	item, action   string
	entry, comment *uuid.UUID
	itemID         uuid.UUID
	snapshot       string
	reasons        []string
	at             time.Time
}

// itemDecision is a decision about the post or comment id of kind, whose
// post is entry.
func itemDecision(kind, action string, owner, id, entry uuid.UUID, snapshot string, at time.Time) decision {
	d := decision{owner: owner, item: kind, action: action, entry: &entry, itemID: id, snapshot: snapshot, at: at}
	if kind == admin.KindComment {
		d.comment = &id
	}
	return d
}

// log writes d in tx.
func (d decision) log(ctx context.Context, tx pgx.Tx) error {
	reasons := d.reasons
	if reasons == nil {
		reasons = []string{}
	}
	_, err := tx.Exec(ctx, `
		insert into moderation_events (owner_id, item, action, entry_id, comment_id, item_id, snapshot, reasons, at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		d.owner, d.item, d.action, d.entry, d.comment, d.itemID, d.snapshot, reasons, d.at)
	return err
}

// DismissItem implements admin.Repository. A dismissal of reports about a
// person only closes them: people are never hidden. For a post or a comment,
// in one transaction:
//  1. the posts or comments the open ones among the ids are on are held for
//     update, in id order. A report of one holds it for key share through
//     its foreign key (and keepReport for no key update), so none lands until
//     this is done;
//  2. which of them are hidden while under review is read;
//  3. the reports are closed, and which of them this closed reports on;
//  4. each of those that was hidden, is not now and is not removed shows
//     again for everyone but its reporters, and that is logged for its
//     owner (the bell tells them). A comment shows again only when its post
//     and the comment it answers are not hidden under review either.
//
// So a card drawn before more reports came in follows the same rule: what
// counts is whether it was hidden just before and is not just after. A
// comment that stayed hidden under its post shows again, unlogged, when the
// post's own reports are dismissed later: its writer is not told.
func (r *AdminRepo) DismissItem(ctx context.Context, kind string, ids []uuid.UUID, person uuid.UUID, at, loggedAt time.Time) (int, bool, error) {
	if kind == admin.KindPerson {
		tag, err := r.pool.Exec(ctx, `update user_reports set resolved_at = $3, resolution = 'dismissed'
			where reporter_id = any($1::uuid[]) and reported_id = $2 and resolved_at is null`, ids, person, at)
		return int(tag.RowsAffected()), false, err
	}
	m, ok := moderatedItems[kind]
	if !ok {
		return 0, false, admin.ErrNotFound
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	targets, err := collect[uuid.UUID](tx.Query(ctx, `
		select x.id from `+m.table+` x
		where x.id in (select mr.`+m.column+` from `+m.reports+` mr
		               where mr.id = any($1::uuid[]) and mr.owner_id = $2 and mr.resolved_at is null)
		order by x.id
		for update`, ids, person))
	if err != nil {
		return 0, false, err
	}
	before := map[uuid.UUID]bool{}
	rows, err := tx.Query(ctx, `select x.id, `+m.underReview("x")+` from `+m.table+` x where x.id = any($1::uuid[])`, targets)
	if err != nil {
		return 0, false, err
	}
	for rows.Next() {
		var id uuid.UUID
		var hidden bool
		if err := rows.Scan(&id, &hidden); err != nil {
			rows.Close()
			return 0, false, err
		}
		before[id] = hidden
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, false, err
	}
	closed, err := collect[*uuid.UUID](tx.Query(ctx, `update `+m.reports+` set resolved_at = $3, resolution = 'dismissed'
		where id = any($1::uuid[]) and owner_id = $2 and resolved_at is null
		returning `+m.column, ids, person, at))
	if err != nil {
		return 0, false, err
	}
	// Only what this closed reports on can show again by it. A Disable
	// closes the reports about the person (ResolvePerson) without waiting
	// for the hold above, so it can close the card's reports between the
	// read and this: the item is no longer hidden then, but this Dismiss
	// closed nothing, and it says nothing.
	closedOn := map[uuid.UUID]bool{}
	for _, id := range closed {
		if id != nil {
			closedOn[*id] = true
		}
	}
	rows, err = tx.Query(ctx, `
		select x.id, x.user_id, x.`+m.snapshot+`, x.`+m.entry+`
		from `+m.table+` x
		where x.id = any($1::uuid[]) and not `+m.underReview("x")+` and not `+m.removed("x")+`
		  and not `+m.aboveUnderReview("x"), targets)
	if err != nil {
		return 0, false, err
	}
	var shown []decision
	for rows.Next() {
		var id, owner, entry uuid.UUID
		var snapshot string
		if err := rows.Scan(&id, &owner, &snapshot, &entry); err != nil {
			rows.Close()
			return 0, false, err
		}
		if before[id] && closedOn[id] {
			shown = append(shown, itemDecision(kind, admin.LogShownAgain, owner, id, entry, snapshot, loggedAt))
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, false, err
	}
	for _, d := range shown {
		if err := d.log(ctx, tx); err != nil {
			return 0, false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, err
	}
	return len(closed), len(shown) > 0, nil
}

// collect reads a query's one column, closing its rows.
func collect[T any](rows pgx.Rows, err error) ([]T, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[T])
}

// RemoveItem implements admin.Repository, in one transaction. The post or
// comment is held for update first, as DismissItem holds it, so a report of
// it that is under way lands before and is closed here. One removed already
// changes nothing: inside the grace that is admin.ErrAlreadyDone (the second
// click of a double click), past it admin.ErrNotFound. The reasons kept in
// the log are those of the reports this closes, each once, the most serious
// first.
func (r *AdminRepo) RemoveItem(ctx context.Context, kind string, target, person uuid.UUID, at, loggedAt time.Time) error {
	m, ok := moderatedItems[kind]
	if !ok {
		return admin.ErrNotFound
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	var snapshot string
	var entry uuid.UUID
	var removed, inside bool
	err = tx.QueryRow(ctx, `select `+m.snapshot+`, `+m.entry+`, removed_at is not null,
		removed_at is not null and `+insideRemovalGrace("removed_at")+`
		from `+m.table+` where id = $1 and user_id = $2 for update`, target, person).Scan(&snapshot, &entry, &removed, &inside)
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.ErrNotFound
	}
	if err != nil {
		return err
	}
	switch {
	case inside:
		return admin.ErrAlreadyDone
	case removed:
		return admin.ErrNotFound
	}
	if _, err := tx.Exec(ctx, `update `+m.table+` set removed_at = now() where id = $1`, target); err != nil {
		return err
	}
	reasons, err := collect[string](tx.Query(ctx, `update `+m.reports+` set resolved_at = $2, resolution = 'removed'
		where `+m.column+` = $1 and resolved_at is null returning reason`, target, at))
	if err != nil {
		return err
	}
	d := itemDecision(kind, admin.LogRemoved, person, target, entry, snapshot, loggedAt)
	d.reasons = mostSeriousFirst(reasons)
	if err := d.log(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// mostSeriousFirst is each reason once, by admin.ReasonRank.
func mostSeriousFirst(reasons []string) []string {
	out := []string{}
	for _, r := range reasons {
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b string) int {
		if d := admin.ReasonRank(a) - admin.ReasonRank(b); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	return out
}

// RestoreItem implements admin.Repository. The line is read in the update
// itself, and the purge holds the row while it deletes it, so of a Bring it
// back and the purge at the line only one wins. A comment is brought back
// only while its post and the comment it answers are not removed, read in
// that update too: back under a removed one, it would still be gone for
// everyone, though its writer was told it is back.
func (r *AdminRepo) RestoreItem(ctx context.Context, kind string, target, person uuid.UUID, loggedAt time.Time) error {
	m, ok := moderatedItems[kind]
	if !ok {
		return admin.ErrNotFound
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	// In the fragments, which alias their own tables, the table's bare name
	// is the row being updated.
	above := ""
	if kind == admin.KindComment {
		above = ` and not ` + postOfCommentRemoved(m.table) + ` and not ` + parentRemoved(m.table)
	}
	var snapshot string
	var entry uuid.UUID
	err = tx.QueryRow(ctx, `update `+m.table+` set removed_at = null
		where id = $1 and user_id = $2 and `+insideRemovalGrace("removed_at")+above+`
		returning `+m.snapshot+`, `+m.entry, target, person).Scan(&snapshot, &entry)
	if errors.Is(err, pgx.ErrNoRows) {
		return whyNotBack(ctx, tx, m, kind, target, person)
	}
	if err != nil {
		return err
	}
	if err := itemDecision(kind, admin.LogRestored, person, target, entry, snapshot, loggedAt).log(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// whyNotBack says why Bring it back brought nothing back: a comment inside
// the grace whose post (admin.ErrPostRemoved, named first) or the comment it
// answers (admin.ErrParentRemoved) is still removed; one that is back, with
// a Bring it back as the newest decision about it (admin.ErrAlreadyDone: the
// second click of a double click); or else admin.ErrNotFound. Newest is by
// created_at, the database's clock, as adminPersonColumns reads the log.
func whyNotBack(ctx context.Context, tx pgx.Tx, m moderatedItem, kind string, target, person uuid.UUID) error {
	post, parent := "false", "false"
	if kind == admin.KindComment {
		post, parent = postOfCommentRemoved("x"), parentRemoved("x")
	}
	var inside, postGone, parentGone, back bool
	err := tx.QueryRow(ctx, `
		select x.removed_at is not null and `+insideRemovalGrace("x.removed_at")+`, `+post+`, `+parent+`,
		       x.removed_at is null and coalesce((select me.action = '`+admin.LogRestored+`' from moderation_events me
		                                          where me.owner_id = x.user_id and me.item_id = x.id
		                                          order by me.created_at desc limit 1), false)
		from `+m.table+` x where x.id = $1 and x.user_id = $2`, target, person).Scan(&inside, &postGone, &parentGone, &back)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return admin.ErrNotFound
	case err != nil:
		return err
	case inside && postGone:
		return admin.ErrPostRemoved
	case inside && parentGone:
		return admin.ErrParentRemoved
	case back:
		return admin.ErrAlreadyDone
	}
	return admin.ErrNotFound
}

// ResolvePerson implements admin.Repository.
func (r *AdminRepo) ResolvePerson(ctx context.Context, person uuid.UUID, resolution string, at time.Time) error {
	for _, q := range []string{
		`update user_reports set resolved_at = $3, resolution = $2 where reported_id = $1 and resolved_at is null`,
		`update post_reports set resolved_at = $3, resolution = $2 where owner_id = $1 and resolved_at is null`,
		`update comment_reports set resolved_at = $3, resolution = $2 where owner_id = $1 and resolved_at is null`,
	} {
		if _, err := r.pool.Exec(ctx, q, person, resolution, at); err != nil {
			return err
		}
	}
	return nil
}

// ReportTarget implements admin.Repository.
func (r *AdminRepo) ReportTarget(ctx context.Context, kind string, id uuid.UUID) (*uuid.UUID, uuid.UUID, error) {
	q := `select entry_id, owner_id from post_reports where id = $1`
	if kind == admin.KindComment {
		q = `select comment_id, owner_id from comment_reports where id = $1`
	} else if kind != admin.KindPost {
		return nil, uuid.UUID{}, admin.ErrNotFound
	}
	var target *uuid.UUID
	var owner uuid.UUID
	err := r.pool.QueryRow(ctx, q, id).Scan(&target, &owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, uuid.UUID{}, admin.ErrNotFound
	}
	return target, owner, err
}

// SetDisabled implements admin.Repository. Disabling also deletes the
// account's Forgot password code, so none can be used while it is disabled
// (or after it is enabled again), revokes every refresh token, so the account
// is logged out everywhere, and deletes its push tokens, so its phones stop
// getting pushes at once. One transaction, in the order of session_locks.go:
// the account's row is held for no key update while its old state is read,
// and the decision is logged (only when the state changes) while it is held.
// The revoke runs twice, as in a password change (UserRepo.ChangePassword
// says why): a refresh in flight at this moment cannot leave a live session
// behind.
func (r *AdminRepo) SetDisabled(ctx context.Context, person uuid.UUID, at *time.Time, loggedAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if at != nil {
		if _, err := tx.Exec(ctx, `delete from password_resets where user_id = $1`, person); err != nil {
			return err
		}
	}
	var wasDisabled bool
	err = tx.QueryRow(ctx, `select disabled_at is not null from users where id = $1 for no key update`, person).Scan(&wasDisabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update users set disabled_at = $2 where id = $1`, person, at); err != nil {
		return err
	}
	if disabling := at != nil; disabling != wasDisabled {
		action := admin.LogEnabled
		if disabling {
			action = admin.LogDisabled
		}
		d := decision{owner: person, item: admin.ItemAccount, action: action, itemID: person, at: loggedAt}
		if err := d.log(ctx, tx); err != nil {
			return err
		}
	}
	if at != nil {
		for range 2 {
			if _, err := tx.Exec(ctx, revokeAllQ, person); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, deletePushTokensQ, person); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
