package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
)

// PersonEntries implements board.Repository: owner's entries the viewer may
// see (visibleOwner, on the viewer's posted days), newest first, strictly
// after the entry `after` in that order when set, at most limit.
func (r *BoardRepo) PersonEntries(ctx context.Context, viewer, owner uuid.UUID, after *uuid.UUID, limit int) ([]board.EntryRow, error) {
	rows, err := r.pool.Query(ctx, boardEntrySelect+`
		where e.user_id = $2 and e.user_id <> $1 and `+visibleOwner+`
		  and exists (select 1 from posted_days v where v.user_id = $1 and v.local_date = e.local_date)
		  and ($3::uuid is null or (e.local_date, e.created_at, e.id) <
		      (select a.local_date, a.created_at, a.id from entries a where a.id = $3))
		order by e.local_date desc, e.created_at desc, e.id desc
		limit $4`, viewer, owner, after, limit)
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
