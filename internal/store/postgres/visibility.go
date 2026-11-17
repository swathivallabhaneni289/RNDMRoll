package postgres

import (
	"strconv"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// Moderation on every read of a post or a comment (migration 0020), as where
// fragments. Each takes the viewer as an SQL expression, a placeholder such
// as "$1::uuid" or a column such as "e.user_id", the way blockedEither does,
// and the table alias of the post or the comment. Their subqueries use
// aliases starting with m, so a viewer column such as p.user_id is never
// shadowed.
//
//   - A post or comment the viewer reported is gone for them for good,
//     whatever became of the report.
//   - One with social.ReviewThreshold open reports from people whose accounts
//     are not gone is hidden while under review, from everyone but its owner
//     (for a comment, its writer). It is worked out here on every read, never
//     stored, so a dismissed report or a reporter who goes stops counting at
//     once.
//   - One an admin removed is gone for everyone, its owner included.
//
// The admin page and the cleanup reads use none of these: the admin must see
// removed content, and a delete must still find its files.

// reviewThreshold is social.ReviewThreshold written into the SQL.
var reviewThreshold = strconv.Itoa(social.ReviewThreshold)

// postOpenReports counts the open reports on the post under alias e from
// people whose accounts are not gone: what postUnderReview weighs. A
// reporter has one report a post (unique reporter_id, entry_id), so the count
// is a count of people.
func postOpenReports(e string) string {
	return `(select count(*) from post_reports mro join users mru on mru.id = mro.reporter_id
		where mro.entry_id = ` + e + `.id and mro.resolved_at is null and not ` + goneAccount("mru") + `)`
}

// commentOpenReports is postOpenReports for the comment under alias c.
func commentOpenReports(c string) string {
	return `(select count(*) from comment_reports mco join users mcu on mcu.id = mco.reporter_id
		where mco.comment_id = ` + c + `.id and mco.resolved_at is null and not ` + goneAccount("mcu") + `)`
}

// postUnderReview is a where fragment true when the post under alias e is
// hidden while under review.
func postUnderReview(e string) string {
	return `(` + postOpenReports(e) + ` >= ` + reviewThreshold + `)`
}

// commentUnderReview is postUnderReview for the comment under alias c.
func commentUnderReview(c string) string {
	return `(` + commentOpenReports(c) + ` >= ` + reviewThreshold + `)`
}

// postReportedBy is a where fragment true when the viewer v reported the post
// under alias e, open or closed.
func postReportedBy(v, e string) string {
	return `exists (select 1 from post_reports mrv where mrv.reporter_id = ` + v + ` and mrv.entry_id = ` + e + `.id)`
}

// commentReportedBy is postReportedBy for the comment under alias c.
func commentReportedBy(v, c string) string {
	return `exists (select 1 from comment_reports mcv where mcv.reporter_id = ` + v + ` and mcv.comment_id = ` + c + `.id)`
}

// postShownTo is a where fragment true when moderation leaves the post under
// alias e to the viewer v: it is not removed, and it is theirs, or they did
// not report it and it is not under review. It says nothing about follows,
// blocks or days: postVisibleTo is the whole rule.
func postShownTo(v, e string) string {
	return `(` + e + `.removed_at is null and (` + e + `.user_id = ` + v +
		` or (not ` + postReportedBy(v, e) + ` and not ` + postUnderReview(e) + `)))`
}

// commentShownTo is postShownTo for the comment under alias c, whose owner is
// its writer. The post's owner gets no exception: a comment of someone else's
// that is under review is hidden from them too. A reply goes with its
// top-level comment, so a read that does not use board.Thread applies this to
// the parent as well.
func commentShownTo(v, c string) string {
	return `(` + c + `.removed_at is null and (` + c + `.user_id = ` + v +
		` or (not ` + commentReportedBy(v, c) + ` and not ` + commentUnderReview(c) + `)))`
}

// postVisibleTo is social.CanView as a where fragment, for the viewer v and
// the post under alias e: postShownTo, and then the post is theirs, or it is
// not archived and is by someone they follow (accepted) with no block either
// way, on a day they posted too. It is the one copy of the rule in SQL; a
// list that also leaves out the owner's own archived posts adds that itself
// (visibleOwner).
func postVisibleTo(v, e string) string {
	return `(` + postShownTo(v, e) + ` and (` + e + `.user_id = ` + v + ` or (` + e + `.archived_at is null
		and exists (select 1 from follows mf where mf.follower_id = ` + v + ` and mf.followee_id = ` + e + `.user_id and mf.status = 'accepted')
		and not ` + blockedEither(v, e+".user_id") + `
		and exists (select 1 from posted_days mpd where mpd.user_id = ` + v + ` and mpd.local_date = ` + e + `.local_date))))`
}
