package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// Mute implements social.Repository.
func (r *SocialRepo) Mute(ctx context.Context, muter, muted uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into mutes (muter_id, muted_id, created_at) values ($1, $2, $3)
		on conflict (muter_id, muted_id) do nothing`, muter, muted, at)
	return err
}

// Unmute implements social.Repository.
func (r *SocialRepo) Unmute(ctx context.Context, muter, muted uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `delete from mutes where muter_id = $1 and muted_id = $2`, muter, muted)
	return err
}

// IsMuted implements social.Repository.
func (r *SocialRepo) IsMuted(ctx context.Context, muter, muted uuid.UUID) (bool, error) {
	var yes bool
	err := r.pool.QueryRow(ctx, `select exists (select 1 from mutes where muter_id = $1 and muted_id = $2)`, muter, muted).Scan(&yes)
	return yes, err
}

// Report implements social.Repository.
func (r *SocialRepo) Report(ctx context.Context, reporter, reported uuid.UUID, reason string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into user_reports (reporter_id, reported_id, reason, created_at) values ($1, $2, $3, $4)
		on conflict (reporter_id, reported_id) do update set reason = excluded.reason, created_at = excluded.created_at`,
		reporter, reported, reason, at)
	return err
}

// Bio implements social.Repository.
func (r *SocialRepo) Bio(ctx context.Context, id uuid.UUID) (string, error) {
	var bio string
	err := r.pool.QueryRow(ctx, `select coalesce(bio, '') from users where id = $1`, id).Scan(&bio)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", social.ErrNotFound
	}
	return bio, err
}

// PeopleOf implements social.Repository.
func (r *SocialRepo) PeopleOf(ctx context.Context, viewer, owner uuid.UUID, followers bool) ([]social.Person, error) {
	// The other end of owner's accepted follows: who follows owner, or whom
	// owner follows.
	other, mine := "f.follower_id", "f.followee_id"
	if !followers {
		other, mine = "f.followee_id", "f.follower_id"
	}
	rows, err := r.pool.Query(ctx, `
		select `+personColumns+` from follows f join users u on u.id = `+other+`
		where `+mine+` = $2 and f.status = 'accepted'
		  and not `+blockedEither("$1::uuid", "u.id")+`
		order by f.accepted_at desc nulls last, u.id`, viewer, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []social.Person{}
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Suggestions implements social.Repository. A person who follows the
// viewer comes first, then the most people in common.
func (r *SocialRepo) Suggestions(ctx context.Context, viewer uuid.UUID, limit int) ([]social.Suggestion, error) {
	rows, err := r.pool.Query(ctx, `
		with cand as (
			select f2.followee_id as id, 1 as mutual, false as follows_you
			from follows f1 join follows f2 on f2.follower_id = f1.followee_id and f2.status = 'accepted'
			where f1.follower_id = $1 and f1.status = 'accepted'
			union all
			select f.follower_id, 0, true from follows f
			where f.followee_id = $1 and f.status = 'accepted'
		), scored as (
			select id, sum(mutual)::int as mutual, bool_or(follows_you) as follows_you
			from cand group by id
		)
		select `+personColumns+`, s.mutual, s.follows_you
		from scored s join users u on u.id = s.id
		where u.id <> $1
		  and u.name is not null and u.name <> '' and u.username is not null
		  and not exists (select 1 from follows mine where mine.follower_id = $1 and mine.followee_id = u.id)
		  and not `+blockedEither("$1::uuid", "u.id")+`
		order by s.follows_you desc, s.mutual desc, lower(u.username)
		limit $2`, viewer, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []social.Suggestion{}
	for rows.Next() {
		var sg social.Suggestion
		p, err := scanPerson(rows, &sg.Mutual, &sg.FollowsYou)
		if err != nil {
			return nil, err
		}
		sg.Person = p
		out = append(out, sg)
	}
	return out, rows.Err()
}
