package social

// ReviewThreshold is how many people must have an open report on a post or a
// comment before it is hidden while under review (developer, 2026-11-17).
// Only reporters whose accounts are not gone count, and the state is worked
// out on every read from the open reports, never stored.
const ReviewThreshold = 3

// Facts are what decides whether a viewer may see one entry.
type Facts struct {
	// IsOwner: the viewer wrote the entry.
	IsOwner bool
	// Follows: the viewer follows the owner with an accepted follow.
	Follows bool
	// Blocked: either of the two blocked the other.
	Blocked bool
	// Archived: the owner put the entry away.
	Archived bool
	// ViewerPostedDay: the viewer has a posted day (any source) for the
	// entry's local_date. The gate reads posted days, never entries, so a
	// deleted post keeps its day open and a skip never opens one.
	ViewerPostedDay bool
	// Removed: an admin removed the entry (migration 0020). It is gone for
	// everyone, its owner included.
	Removed bool
	// UnderReview: ReviewThreshold people reported the entry and the admin
	// has not looked yet. Its owner still sees it.
	UnderReview bool
	// Reported: the viewer reported the entry. It is gone for them for good.
	Reported bool
}

// CanView is THE rule. A removed entry is gone for everyone. Otherwise the
// owner always sees their entry. Anyone else sees it only when they follow
// the owner (accepted), nobody blocked anybody, the entry is not archived,
// they did not report it, it is not hidden while under review, and they
// posted that day themselves. Phase 6 adds the public branch here and
// nowhere else.
func CanView(f Facts) bool {
	if f.Removed {
		return false
	}
	if f.IsOwner {
		return true
	}
	if f.Blocked || f.Archived || f.Reported || f.UnderReview {
		return false
	}
	return f.Follows && f.ViewerPostedDay
}

// CanReact is CanView for anyone but the owner.
func CanReact(f Facts) bool {
	return CanView(f) && !f.IsOwner
}

// CanComment is CanView with the owner's comments switch on.
func CanComment(f Facts, commentsOff bool) bool {
	return CanView(f) && !commentsOff
}

// CanDeleteComment: the comment's writer or the post's owner.
func CanDeleteComment(isWriter, isOwner bool) bool {
	return isWriter || isOwner
}
