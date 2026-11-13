package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
)

// PersonEntries implements board.Repository: owner's entries the viewer may
// see (visibleOwner, on the viewer's posted days), newest first, strictly
// before the date `before` when set, at most limit. One owner has at most one
// entry a day (entries_one_per_day), so the date is a full place in this
// order, and it needs no row of its own: an entry deleted after a page was
// read leaves the next page whole.
func (r *BoardRepo) PersonEntries(ctx context.Context, viewer, owner uuid.UUID, before *string, limit int) ([]board.EntryRow, error) {
	rows, err := r.pool.Query(ctx, boardEntrySelect+`
		where e.user_id = $2 and e.user_id <> $1 and `+visibleOwner+`
		  and exists (select 1 from posted_days v where v.user_id = $1 and v.local_date = e.local_date)
		  and ($3::date is null or e.local_date < $3::date)
		order by e.local_date desc, e.created_at desc, e.id desc
		limit $4`, viewer, owner, before, limit)
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

// PostCount implements board.Repository.
func (r *BoardRepo) PostCount(ctx context.Context, owner uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `select count(*) from entries where user_id = $1 and archived_at is null`, owner).Scan(&n)
	return n, err
}
