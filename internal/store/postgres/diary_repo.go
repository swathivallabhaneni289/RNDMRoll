package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/diary"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
)

// DiaryReader is the read-only diary.Reader over the pool. Every query is
// scoped to one person, opens no transaction block and takes no lock, so it
// never waits for a spin or a post. Archived entries are left out of rows,
// chips, tastes and recent; spin days and goal weeks do not look at entries.
type DiaryReader struct {
	pool *pgxpool.Pool
}

// NewDiaryReader builds a DiaryReader backed by pool.
func NewDiaryReader(pool *pgxpool.Pool) *DiaryReader {
	return &DiaryReader{pool: pool}
}

var _ diary.Reader = (*DiaryReader)(nil)

func diaryUserID(userID string) (uuid.UUID, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("diary: user id is not a uuid: %w", err)
	}
	return id, nil
}

// entries runs entrySelect with the extra where text (which may use $2 and
// up) and fills each entry's photos with one more query.
func (r *DiaryReader) entries(ctx context.Context, uid uuid.UUID, where string, args ...any) ([]spin.Entry, error) {
	rows, err := r.pool.Query(ctx, entrySelect+where, append([]any{uid}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []spin.Entry{}
	ids := []uuid.UUID{}
	for rows.Next() {
		var e spin.Entry
		var dateStr string
		if err := rows.Scan(&e.ID, &e.SpinID, &dateStr, &e.CategoryID, &e.CategoryName, &e.Title, &e.Stars,
			&e.Reaction, &e.Comment, &e.Late, &e.CreatedAt, &e.UpdatedAt, &e.Archived, &e.CommentsOff); err != nil {
			return nil, err
		}
		if e.LocalDate, err = spin.ParseDate(dateStr); err != nil {
			return nil, err
		}
		e.CreatedAt, e.UpdatedAt = e.CreatedAt.UTC(), e.UpdatedAt.UTC()
		e.Photos = []spin.EntryPhoto{}
		id, err := uuid.Parse(e.ID)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if len(out) == 0 {
		return out, nil
	}
	prows, err := r.pool.Query(ctx, `
		select entry_id::text, id::text, position, storage_key, content_type, size_bytes, source
		from entry_photos
		where user_id = $1 and entry_id = any($2::uuid[])
		order by entry_id, position`, uid, ids)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	byEntry := make(map[string][]spin.EntryPhoto, len(out))
	for prows.Next() {
		var entryID string
		var p spin.EntryPhoto
		if err := prows.Scan(&entryID, &p.ID, &p.Position, &p.Key, &p.ContentType, &p.SizeBytes, &p.Source); err != nil {
			return nil, err
		}
		byEntry[entryID] = append(byEntry[entryID], p)
	}
	if err := prows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if ph, ok := byEntry[out[i].ID]; ok {
			out[i].Photos = ph
		}
	}
	return out, nil
}

// Rows implements diary.Reader. A date is either an entry or a skipped day,
// never both (a skipped day has no posted-day row), so the two lists never
// share a date and a date alone is a paging key.
func (r *DiaryReader) Rows(ctx context.Context, userID string, categoryID *string, before string, limit int) ([]diary.Row, error) {
	uid, err := diaryUserID(userID)
	if err != nil {
		return nil, err
	}
	var beforeArg any
	if before != "" {
		beforeArg = before
	}
	var catArg any
	if categoryID != nil {
		cid, err := uuid.Parse(*categoryID)
		if err != nil {
			return []diary.Row{}, nil
		}
		catArg = cid
	}
	list, err := r.entries(ctx, uid, `
		e.archived_at is null
		and ($2::date is null or e.local_date < $2::date)
		and ($3::uuid is null or s.category_id = $3::uuid)
		order by e.local_date desc
		limit $4`, beforeArg, catArg, limit)
	if err != nil {
		return nil, err
	}
	rows := make([]diary.Row, 0, len(list))
	for i := range list {
		e := list[i]
		rows = append(rows, diary.Row{
			Kind: diary.KindEntry, Entry: &e, LocalDate: e.LocalDate.String(),
			Category: spin.CategoryRef{ID: e.CategoryID, Name: e.CategoryName},
		})
	}
	if categoryID == nil {
		srows, err := r.pool.Query(ctx, `
			select local_date::text, category_id::text, category_name
			from (
				select distinct on (local_date) local_date, category_id, category_name, result
				from spins
				where user_id = $1 and ($2::date is null or local_date < $2::date)
				order by local_date desc, seq desc
			) latest
			where result = 'skipped'
			  and not exists (select 1 from posted_days p where p.user_id = $1 and p.local_date = latest.local_date)
			order by local_date desc
			limit $3`, uid, beforeArg, limit)
		if err != nil {
			return nil, err
		}
		defer srows.Close()
		for srows.Next() {
			var row diary.Row
			row.Kind = diary.KindSkipped
			if err := srows.Scan(&row.LocalDate, &row.Category.ID, &row.Category.Name); err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
		if err := srows.Err(); err != nil {
			return nil, err
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].LocalDate > rows[j].LocalDate })
		if len(rows) > limit {
			rows = rows[:limit]
		}
	}
	return rows, nil
}

// chipOrder puts live categories first by wheel position, then removed ones
// by name ignoring case.
const chipOrder = `order by (c.archived_at is not null), case when c.archived_at is null then c.position end, lower(c.name), c.id`

// Chips implements diary.Reader.
func (r *DiaryReader) Chips(ctx context.Context, userID string) ([]diary.Chip, error) {
	uid, err := diaryUserID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		select c.id::text, c.name, c.archived_at is not null
		from wheel_categories c
		where c.user_id = $1
		  and exists (
			select 1 from entries e
			join spins s on s.id = e.spin_id and s.user_id = e.user_id
			where e.user_id = $1 and e.archived_at is null and s.category_id = c.id)
		`+chipOrder, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []diary.Chip{}
	for rows.Next() {
		var c diary.Chip
		if err := rows.Scan(&c.ID, &c.Name, &c.Removed); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SpinDays implements diary.Reader: the days spun, bonus spins not counted.
func (r *DiaryReader) SpinDays(ctx context.Context, userID string) (int, error) {
	uid, err := diaryUserID(userID)
	if err != nil {
		return 0, err
	}
	var n int
	err = r.pool.QueryRow(ctx, `select count(*) from spins where user_id = $1 and kind = 'daily'`, uid).Scan(&n)
	return n, err
}

// GoalWeekStarts implements diary.Reader.
func (r *DiaryReader) GoalWeekStarts(ctx context.Context, userID string, max int) ([]string, error) {
	uid, err := diaryUserID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		select week_start::text from goal_weeks where user_id = $1 order by week_start desc limit $2`, uid, max)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Tastes implements diary.Reader.
func (r *DiaryReader) Tastes(ctx context.Context, userID string) ([]diary.RawTaste, error) {
	uid, err := diaryUserID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		select c.id::text, c.name, c.archived_at is not null, sum(e.stars)::int, count(*)::int
		from entries e
		join spins s on s.id = e.spin_id and s.user_id = e.user_id
		join wheel_categories c on c.id = s.category_id and c.user_id = s.user_id
		where e.user_id = $1 and e.archived_at is null
		group by c.id, c.name, c.archived_at, c.position
		`+chipOrder, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []diary.RawTaste{}
	for rows.Next() {
		var t diary.RawTaste
		if err := rows.Scan(&t.Chip.ID, &t.Chip.Name, &t.Chip.Removed, &t.SumStars, &t.Count); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Recent implements diary.Reader.
func (r *DiaryReader) Recent(ctx context.Context, userID string, n int) ([]spin.Entry, error) {
	uid, err := diaryUserID(userID)
	if err != nil {
		return nil, err
	}
	return r.entries(ctx, uid, `e.archived_at is null order by e.local_date desc limit $2`, n)
}

// TimeZone implements diary.Reader.
func (r *DiaryReader) TimeZone(ctx context.Context, userID string) (string, error) {
	uid, err := diaryUserID(userID)
	if err != nil {
		return "", err
	}
	var name string
	err = r.pool.QueryRow(ctx, `select tz_name from user_time_zones where user_id = $1`, uid).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return name, err
}

// Social implements diary.Reader with the comments the post page shows the
// owner: comments and replies, leaving out anyone with a block either way and
// a reply whose top-level comment's writer has one (board.Thread). A comment
// the owner hid is left out too, so it never shows under a post in the list.
// Only the person's own entries are read.
func (r *DiaryReader) Social(ctx context.Context, userID string, entryIDs []string) (map[string]diary.PostSocial, error) {
	uid, err := diaryUserID(userID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(entryIDs))
	for _, s := range entryIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("diary: entry id is not a uuid: %w", err)
		}
		ids = append(ids, id)
	}
	out := map[string]diary.PostSocial{}
	if len(ids) == 0 {
		return out, nil
	}

	shown := `
		from post_comments c
		join entries e on e.id = c.entry_id
		join users u on u.id = c.user_id
		left join post_comments p on p.id = c.parent_id
		where c.entry_id = any($2::uuid[]) and e.user_id = $1 and c.hidden_at is null
		  and not ` + blockedEither("$1::uuid", "c.user_id") + `
		  and (c.parent_id is null or not ` + blockedEither("$1::uuid", "p.user_id") + `)`
	rows, err := r.pool.Query(ctx, `select c.entry_id, count(*) `+shown+` group by c.entry_id`, uid, ids)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			rows.Close()
			return nil, err
		}
		s := out[id.String()]
		s.Comments = n
		out[id.String()] = s
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.pool.Query(ctx, `
		select entry_id, author_id, name, username, avatar_url, body from (
		  select c.entry_id, u.id as author_id, coalesce(u.name, '') as name, coalesce(u.username, '') as username,
		    u.avatar_url, c.body, c.created_at, c.id,
		    row_number() over (partition by c.entry_id order by c.created_at desc, c.id desc) as n
		  `+shown+`
		) x
		where n <= $3
		order by entry_id, created_at, id`, uid, ids, diary.LatestCount)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, author uuid.UUID
		var latest diary.LatestComment
		if err := rows.Scan(&id, &author, &latest.Name, &latest.Username, &latest.PhotoURL, &latest.Text); err != nil {
			return nil, err
		}
		latest.AuthorID = author.String()
		s := out[id.String()]
		s.Latest = append(s.Latest, latest)
		out[id.String()] = s
	}
	return out, rows.Err()
}
