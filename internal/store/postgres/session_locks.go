package postgres

// The session tables are written by several transactions at once: a log in,
// a refresh (RefreshTokenRepo.Rotate), a log out (RevokeByHash), a password
// change (changePassword), a Forgot password reset (resetPassword, through
// endSessions), a password set from the terminal (UserRepo.SetPassword, also
// through endSessions), an admin's disable, a deletion request
// (UserRepo.RequestDeletion), the purge (PurgeRepo.Purge), the phone
// saving or deleting its push token (PushRepo), and a sign-up's code making
// its account (PendingSignupRepo.Finish). So that no two of them can ever wait
// on each other in a circle (Postgres would end one of them with a deadlock
// error), each takes rows in this order and never goes back to an earlier
// table once it holds a later one:
//
//	pending_signups, password_resets, users, refresh_tokens, push_tokens
//
// pending_signups comes first because the account a sign-up makes is the
// users row the code is spent for: Finish holds every waiting sign-up of the
// email before it inserts the account, and nothing that holds a users row
// ever reaches for a sign-up. Its other writers (the start, a try, a new
// code, the sweep) touch that table alone.
//
// A plain read takes no lock, so reading a table out of turn is fine. So is
// the key share a write to push_tokens takes on the refresh token it names
// (its foreign key): revoking or rotating a token holds the row for no key
// update, which a key share never waits on. That is also why rows of users
// and refresh_tokens are held for no key update and never for update: every
// write that names them (refresh_tokens and push_tokens name users,
// push_tokens names refresh_tokens) takes a key share on them. A statement
// that takes several rows of one table takes them in id order, so two such
// statements over the same rows cannot each hold a row the other wants. The
// purge is the one writer that holds a users row for update: it is about to
// delete the row, and any write that would name the account must wait for
// that and then fail.
const (
	// revokeAllQ revokes every live refresh token of the account $1.
	revokeAllQ = `update refresh_tokens set revoked_at = now() where id in (
		select id from refresh_tokens where user_id = $1 and revoked_at is null order by id for no key update)`
	// deletePushTokensQ deletes every push token of the account $1.
	deletePushTokensQ = `delete from push_tokens where id in (
		select id from push_tokens where user_id = $1 order by id for update)`
)
