package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/user"
)

// RefreshTokenRepo is a pgx-backed implementation of user.RefreshTokenRepository.
type RefreshTokenRepo struct {
	pool *pgxpool.Pool
}

// NewRefreshTokenRepo builds a RefreshTokenRepo backed by pool.
func NewRefreshTokenRepo(pool *pgxpool.Pool) user.RefreshTokenRepository {
	return &RefreshTokenRepo{pool: pool}
}

func (r *RefreshTokenRepo) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time, userAgent *string) (uuid.UUID, error) {
	const q = `insert into refresh_tokens (user_id, token_hash, expires_at, user_agent) values ($1, $2, $3, $4::text) returning id`
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, q, userID, tokenHash, expiresAt, userAgent).Scan(&id); err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

// GetActiveByHash returns ErrTokenExpired for a past expires_at and
// ErrTokenInvalid for a revoked or missing row.
func (r *RefreshTokenRepo) GetActiveByHash(ctx context.Context, tokenHash []byte) (*user.RefreshToken, error) {
	const q = `select id, user_id, expires_at, revoked_at from refresh_tokens where token_hash = $1`
	var rt user.RefreshToken
	err := r.pool.QueryRow(ctx, q, tokenHash).Scan(&rt.ID, &rt.UserID, &rt.ExpiresAt, &rt.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrTokenInvalid
		}
		return nil, err
	}
	if rt.RevokedAt != nil {
		return nil, user.ErrTokenInvalid
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, user.ErrTokenExpired
	}
	return &rt, nil
}

// Rotate revokes oldID and inserts a new row in one transaction, so a crash
// mid-rotation cannot leave two simultaneously valid refresh tokens. The
// new row is inserted before the old row is updated: rotated_to is a
// foreign key into this same table, so it must reference an existing row.
// The session carries on under the new id: the phone's push token tied to
// oldID moves to it in the same transaction (migration 0017).
func (r *RefreshTokenRepo) Rotate(ctx context.Context, oldID uuid.UUID, newHash []byte, expiresAt time.Time) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.UUID{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	var userID uuid.UUID
	var userAgent *string
	const selectQ = `select user_id, user_agent from refresh_tokens where id = $1`
	if err := tx.QueryRow(ctx, selectQ, oldID).Scan(&userID, &userAgent); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, user.ErrTokenInvalid
		}
		return uuid.UUID{}, err
	}

	var newID uuid.UUID
	const insertQ = `insert into refresh_tokens (user_id, token_hash, expires_at, user_agent) values ($1, $2, $3, $4::text) returning id`
	if err := tx.QueryRow(ctx, insertQ, userID, newHash, expiresAt, userAgent).Scan(&newID); err != nil {
		return uuid.UUID{}, err
	}

	// revoked_at is rechecked here, not just in GetActiveByHash: a revoke or a
	// second rotation that committed after the caller's check must not let
	// this call mint a live replacement. Zero rows rolls the insert back.
	const updateQ = `update refresh_tokens set revoked_at = now(), rotated_to = $1 where id = $2 and revoked_at is null`
	tag, err := tx.Exec(ctx, updateQ, newID, oldID)
	if err != nil {
		return uuid.UUID{}, err
	}
	if tag.RowsAffected() == 0 {
		return uuid.UUID{}, user.ErrTokenInvalid
	}
	// After the update above, which holds the old row: a push token being
	// saved for the old session holds that row too (PushRepo.SaveToken), so it
	// is either moved here or saved after this commit, finding the new id.
	// Push rows are taken in id order, as every statement that takes several
	// does (session_locks.go says why).
	const moveQ = `update push_tokens set refresh_token_id = $1 where id in (
		select id from push_tokens where refresh_token_id = $2 order by id for update)`
	if _, err := tx.Exec(ctx, moveQ, newID, oldID); err != nil {
		return uuid.UUID{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.UUID{}, err
	}
	return newID, nil
}

// maxChainSteps bounds a walk along rotated_to. A session refreshed every
// few minutes for its whole life is a few thousand steps; the bound only
// guards against a loop, which rotated_to can never form (it always names a
// newer row).
const maxChainSteps = 1_000_000

// maxReuseSteps bounds the walk RevokeByHash makes from a token that is no
// longer live to the token its session holds now. A refresh answer lost on
// its way, or a log out with the token from before it, is a step or two. A
// token more refreshes old than this ends nothing (it is refused all the
// same), so a token kept and sent again and again never costs more than this
// many steps.
const maxReuseSteps = 50

// RevokeByHash ends the session tokenHash belongs to; the interface
// (user.RefreshTokenRepository) says when it returns ErrTokenInvalid. A
// rotated token is revoked already, so what it ends is the token at the end
// of its rotated_to chain: one refresh answer lost on its way to the phone, a
// log out with the token from before it, or a stolen token used after the
// phone (or the phone after the thief) all end the session, and with it the
// push token tied to it. The end is found in one statement, then held: a
// refresh of it that is in flight holds it too, so this waits for it and then
// follows it to the token it made, which can never outlive the session. The
// whole walk, both parts, takes at most maxReuseSteps steps: what it reaches
// within them is revoked, and a session further on than that is left alone.
func (r *RefreshTokenRepo) RevokeByHash(ctx context.Context, tokenHash []byte) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	var end uuid.UUID
	var step int
	err = tx.QueryRow(ctx, `
		with recursive chain (id, rotated_to, step) as (
			select id, rotated_to, 0 from refresh_tokens where token_hash = $1
			union all
			select r.id, r.rotated_to, c.step + 1 from refresh_tokens r join chain c on r.id = c.rotated_to
			where c.step < $2
		)
		select id, step from chain order by step desc limit 1`, tokenHash, maxReuseSteps).Scan(&end, &step)
	if errors.Is(err, pgx.ErrNoRows) {
		return user.ErrTokenInvalid
	}
	if err != nil {
		return err
	}
	for ; step <= maxReuseSteps; step++ {
		var next *uuid.UUID
		var live bool
		err := tx.QueryRow(ctx, `select rotated_to, revoked_at is null from refresh_tokens where id = $1 for no key update`,
			end).Scan(&next, &live)
		if err != nil {
			return err
		}
		if next != nil {
			end = *next
			continue
		}
		if !live {
			return user.ErrTokenInvalid
		}
		if _, err := tx.Exec(ctx, `update refresh_tokens set revoked_at = now() where id = $1`, end); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	return user.ErrTokenInvalid
}

// RevokeAllForUser revokes every live token of userID,
// taking the rows in id order (session_locks.go).
func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, revokeAllQ, userID)
	return err
}
